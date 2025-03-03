package server

import (
	"context"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
)

type ScaleServerRequest struct {
	Replicas *int32 `json:"replicas"`
}

type ScaleServerHandler struct{}

func (h *ScaleServerHandler) HandleRequest(c *gin.Context, w *service.Wrapper) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody ScaleServerRequest
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if reqBody.Replicas == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "replicas field required"})
		return
	}

	if *reqBody.Replicas > 1 || *reqBody.Replicas < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "replicas must be either 1 or 0"})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	// Verify that src details is "nil". This avoids a scenario where a
	// user could create more than 1 src.
	if len(user.Servers) == 0 {
		log.Errorf("user: %s has no server to scale", user.DiscordID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "no server to scale."})
		return
	}

	// TODO Currently only 1 server is supported per user. If more in the future then this needs updated
	server := user.Servers[0]

	if server.State == model.RUNNING && *reqBody.Replicas == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server already running. replicas must be 0 when server state is: RUNNING"})
		return
	}

	if server.State == model.TERMINATED && *reqBody.Replicas == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no server to terminate. replicas must be 1 when server state is: TERMINATED"})
		return
	}

	// Scale down the deployment
	deploymentName := fmt.Sprintf("valheim-%s", user.DiscordID)
	err = UpdateServerArgs(w.KubeService, deploymentName, &server)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update deployment args: %v", err)})
		return
	}

	scale, err := w.KubeService.GetClient().AppsV1().Deployments("hearthhub").GetScale(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("failed to get deployment scale from kubernetes api: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get deployment scale from kubernetes api: %v", err)})
		return
	}

	scale.Spec.Replicas = *reqBody.Replicas
	_, err = w.KubeService.GetClient().AppsV1().Deployments("hearthhub").UpdateScale(context.TODO(), deploymentName, scale, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("failed to update deployment scale: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update deployment scale: %v", err)})
		return
	}

	state := model.TERMINATED
	if *reqBody.Replicas == 1 {
		state = model.RUNNING
	}
	server.State = state
	tx := w.HearthhubDb.Save(server)
	if tx.Error != nil {
		log.Errorf("could not update server state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not update server state: %s", err)})
		return
	}

	c.JSON(http.StatusOK, server)
}

// UpdateServerArgs Update's a deployment's args to reflect what is in Cognito. This avoids complex argument merging logic by simply having the frontend
// update cognito with the new src args.
func UpdateServerArgs(kubeService service.KubernetesService, deploymentName string, server *model.Server) error {
	deployment, err := kubeService.GetClient().AppsV1().Deployments("hearthhub").Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("error getting deployment: %v", err)
		return err
	}

	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "valheim" {
			deployment.Spec.Template.Spec.Containers[i].Args = []string{server.WorldDetails.ToStringArgs()}
			break
		}
	}

	_, err = kubeService.GetClient().AppsV1().Deployments("hearthhub").Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("error updating deployment: %v", err)
		return err
	}

	return nil
}
