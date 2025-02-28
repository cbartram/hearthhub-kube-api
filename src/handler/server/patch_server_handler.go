package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/cfg"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type PatchServerHandler struct{}

// HandleRequest Much of this logic overlaps with the /create endpoint. It uses the same request body, validation logic, and method structure.
// The primary difference is in how it patches the container run args for a deployment rather than creating a new one.
func (p *PatchServerHandler) HandleRequest(c *gin.Context, kubeService service.KubernetesService, cognito service.CognitoService, stripeService *service.StripeService, ctx context.Context) {
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

	user := tmp.(*service.CognitoUser)

	attributes, err := cognito.GetUserAttributes(ctx, &user.Credentials.AccessToken)
	serverDetails := util.GetAttribute(attributes, "custom:server_details")
	currentServerDetails := CreateServerResponse{}
	if err != nil {
		log.Errorf("could not get user attributes: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not get user attributes: %s", err)})
		return
	}

	// Verify the user has a src to patch
	if serverDetails == "nil" {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("src does not exists for user: %s", user.DiscordID)})
		return
	}

	json.Unmarshal([]byte(serverDetails), &currentServerDetails)

	limits, err := stripeService.GetSubscriptionLimits(user.SubscriptionId)
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

	config := MakeConfigWithDefaults(&reqBody)
	valheimServer, err := PatchServerDeployment(config, kubeService, user)
	if err != nil {
		log.Errorf("could not patch dedicated src deployment: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not patch dedicated src deployment: " + err.Error()})
		return
	}

	// Patch doesn't have access to all fields like: previous src ip, port, or the pvc name
	// so they are copied from the previous values in cognito
	valheimServer.ServerIp = currentServerDetails.ServerIp
	valheimServer.ServerPort = currentServerDetails.ServerPort
	valheimServer.PvcName = currentServerDetails.PvcName

	serverData, err := json.Marshal(valheimServer)
	if err != nil {
		log.Errorf("failed to marshall src data to json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to marshall src data to json: %s", err.Error())})
		return
	}

	attr := util.MakeAttribute("custom:server_details", string(serverData))
	err = cognito.UpdateUserAttributes(ctx, &user.Credentials.AccessToken, []types.AttributeType{attr})
	if err != nil {
		log.Errorf("failed to update src details in cognito user attribute: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("failed to update src details in cognito user attribute: %v", err)})
		return
	}

	c.JSON(http.StatusOK, valheimServer)
}

// PatchServerDeployment Updates a src deployment with new container args.
func PatchServerDeployment(config *cfg.Config, kubeService service.KubernetesService, user *service.CognitoUser) (response *CreateServerResponse, err error) {
	deployment, err := kubeService.GetClient().AppsV1().Deployments("hearthhub").Get(
		context.TODO(),
		fmt.Sprintf("valheim-%s", user.DiscordID),
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %v", err)
	}

	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "valheim" {
			deployment.Spec.Template.Spec.Containers[i].Args = []string{config.ToStringArgs()}
			break
		}
	}

	_, err = kubeService.GetClient().AppsV1().Deployments("hearthhub").Update(
		context.TODO(),
		deployment,
		metav1.UpdateOptions{},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update deployment: %v", err)
	}

	return &CreateServerResponse{
		WorldDetails:   *config,
		DeploymentName: deployment.Name,
		State:          cfg.TERMINATED,
	}, nil
}
