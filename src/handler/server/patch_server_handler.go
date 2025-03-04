package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type PatchServerHandler struct{}

// HandleRequest Much of this logic overlaps with the /create endpoint. It uses the same request body, validation logic, and method structure.
// The primary difference is in how it patches the container run args for a deployment rather than creating a new one.
func (p *PatchServerHandler) HandleRequest(c *gin.Context, ctx context.Context, w *service.Wrapper) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody CreateServerRequest
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	err = reqBody.Validate()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %s", err)})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)
	if len(user.Servers) >= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("server does not exists for user: %s", user.DiscordID)})
		return
	}

	limits, err := w.StripeService.GetSubscriptionLimits(user.SubscriptionId)
	if err != nil {
		log.Errorf("failed to get user subscription limits: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get subscription limit: %v", err)})
		return
	}
	user.SubscriptionLimits = *limits

	if *reqBody.BackupCount > user.SubscriptionLimits.MaxBackups {
		reqBody.BackupCount = &user.SubscriptionLimits.MaxBackups
		log.Infof("request max backups > users subscription limit: %d, new backup count set to limit: %d", user.SubscriptionLimits.MaxBackups, *reqBody.BackupCount)
	}

	world := MakeWorldWithDefaults(&reqBody)
	err = PatchServerDeployment(world, w.KubeService, user)
	if err != nil {
		log.Errorf("could not patch dedicated src deployment: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not patch dedicated src deployment: " + err.Error()})
		return
	}

	// Note: this assumes world and server name combo is unique TODO ensure this is true before creating new server
	var existingServer *model.Server
	for _, server := range user.Servers {
		if server.WorldDetails.World == *reqBody.World && server.WorldDetails.Name == *reqBody.Name {
			existingServer = &server
			break
		}
	}

	if existingServer == nil {
		log.Errorf("could not find server for user: %s where world name matches: %s and server name matches: %s ", user.DiscordID, *reqBody.World, *reqBody.Name)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("could not find server for user: %s where world name matches: %s and server name matches: %s ", user.DiscordID, *reqBody.World, *reqBody.Name)})
		return
	}

	// Update our database with the newly patched server args
	existingServer.WorldDetails = world
	tx := w.HearthhubDb.Save(existingServer)
	if tx.Error != nil {
		log.Errorf("could not save updated server details: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save updated server details: " + tx.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, existingServer)
}

// PatchServerDeployment Updates a src deployment with new container args.
func PatchServerDeployment(world *model.WorldDetails, kubeService service.KubernetesService, user *model.User) error {
	deployment, err := kubeService.GetClient().AppsV1().Deployments("hearthhub").Get(
		context.TODO(),
		fmt.Sprintf("valheim-%s", user.DiscordID),
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "valheim" {
			deployment.Spec.Template.Spec.Containers[i].Args = []string{world.ToStringArgs()}
			break
		}
	}

	_, err = kubeService.GetClient().AppsV1().Deployments("hearthhub").Update(
		context.TODO(),
		deployment,
		metav1.UpdateOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}

	return nil
}
