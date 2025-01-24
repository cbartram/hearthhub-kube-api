package server

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/cbartram/hearthhub-mod-api/server/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

type ScaleServerRequest struct {
	DiscordId    string `json:"discord_id"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Replicas     int32  `json:"replicas"`
}

type ScaleServerHandler struct{}

func (h *ScaleServerHandler) HandleRequest(c *gin.Context, clientset *kubernetes.Clientset, ctx context.Context) {
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

	if reqBody.Replicas > 1 || reqBody.Replicas < 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "replicas must be either 1 or 0"})
		return
	}

	// Update user info in Cognito with valheim server data.
	cognito := service.MakeCognitoService()
	log.Infof("authenticating user with discord id: %s", reqBody.DiscordId)
	user, err := cognito.AuthUser(ctx, &reqBody.RefreshToken, &reqBody.DiscordId)
	if err != nil {
		log.Errorf("could not authenticate user with refresh token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not authenticate user with refresh token: %s", err)})
		return
	}

	log.Infof("user authenticated: %s", user.Email)

	// Verify that server details is "nil". This avoids a scenario where a
	// user could create more than 1 server.
	attributes, err := cognito.GetUserAttributes(ctx, &user.Credentials.AccessToken)
	serverJson := util.GetAttribute(attributes, "custom:server_details")
	server := ValheimDedicatedServer{}

	if err != nil {
		log.Errorf("could not get user attributes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not get user attributes: %s", err)})
		return
	}

	if serverJson == "nil" {
		c.JSON(http.StatusNotFound, gin.H{"error": "valheim server does not exist. nothing to scale."})
		return
	}

	json.Unmarshal([]byte(serverJson), &server)

	if server.State == RUNNING && reqBody.Replicas == 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server already running. replicas must be 0 when server state is: RUNNING"})
		return
	}

	if server.State == TERMINATED && reqBody.Replicas == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no server to terminate. replicas must be 1 when server state is: TERMINATED"})
		return
	}

	// Scale down the deployment
	deploymentName := fmt.Sprintf("valheim-%s", reqBody.DiscordId)
	scale, err := clientset.AppsV1().Deployments("hearthhub").GetScale(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		// TODO If deployment doesn't exist we are in a bad state and need to set cognito custom:server_details to "nil"
		log.Errorf("failed to get deployment scale from kubernetes api: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get deployment scale from kubernetes api: %v", err)})
		return
	}

	scale.Spec.Replicas = reqBody.Replicas
	_, err = clientset.AppsV1().Deployments("hearthhub").UpdateScale(context.TODO(), deploymentName, scale, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("failed to update deployment scale: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update deployment scale: %v", err)})
		return
	}

	state := TERMINATED
	if reqBody.Replicas == 1 {
		state = RUNNING
	}
	s, err := UpdateServerDetails(ctx, cognito, &server, user, state)
	if err != nil {
		log.Errorf("could not update user attributes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not update user attributes: %s", err)})
		return
	}

	c.JSON(http.StatusOK, s)
}

func UpdateServerDetails(ctx context.Context, cognito *service.CognitoService, server *ValheimDedicatedServer, user *service.CognitoUser, state string) (*ValheimDedicatedServer, error) {
	server.State = state
	s, _ := json.Marshal(server)
	serverAttribute := util.MakeAttribute("custom:server_details", string(s))
	err := cognito.UpdateUserAttributes(ctx, &user.Credentials.AccessToken, []types.AttributeType{serverAttribute})
	if err != nil {
		return nil, err
	}
	return server, nil
}
