package server

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type DeleteServerHandler struct{}

func (d *DeleteServerHandler) HandleRequest(c *gin.Context, kubeService service.KubernetesService, cognito service.CognitoService, ctx context.Context) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	// Add simple deployment and pvc actions which have already been applied so we can re-use the same logic
	// to roll them back i.e. delete them!
	kubeService.AddAction(service.DeploymentAction{Deployment: &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("valheim-%s", user.DiscordID),
			Namespace: "hearthhub",
		},
	}})

	kubeService.AddAction(service.PVCAction{PVC: &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("valheim-pvc-%s", user.DiscordID),
			Namespace: "hearthhub",
		},
	}})

	// Delete deployment and pvc before updating cognito to avoid a scenario where the user could spin up more than 1 src
	// if their cognito gets updated but src deletion fails.
	names, err := kubeService.Rollback()
	if err != nil {
		log.Errorf("error deleting deployment/pvc: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete deployment/pvc: %v", err)})
		return
	}

	err = cognito.UpdateUserAttributes(ctx, &user.Credentials.AccessToken, []types.AttributeType{
		{
			Name:  aws.String("custom:server_details"),
			Value: aws.String("nil"),
		},
		{
			Name:  aws.String("custom:installed_mods"),
			Value: aws.String("{}"),
		},
		{
			Name:  aws.String("custom:installed_backups"),
			Value: aws.String("{}"),
		},
	})

	if err != nil {
		log.Errorf("error updating user attributes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error updating user attributes: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("deleted resources successfully"),
		"resources": names,
	})
}
