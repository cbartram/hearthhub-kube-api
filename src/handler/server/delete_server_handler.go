package server

import (
	"fmt"
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

func (d *DeleteServerHandler) HandleRequest(c *gin.Context, w *service.Wrapper) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	// Add simple deployment and pvc actions which have already been applied so we can re-use the same logic
	// to roll them back i.e. delete them!
	w.KubeService.AddAction(service.DeploymentAction{Deployment: &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("valheim-%s", user.DiscordID),
			Namespace: "hearthhub",
		},
	}})

	w.KubeService.AddAction(service.PVCAction{PVC: &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("valheim-pvc-%s", user.DiscordID),
			Namespace: "hearthhub",
		},
	}})

	// Delete deployment and pvc before updating cognito to avoid a scenario where the user could spin up more than 1 src
	// if their cognito gets updated but src deletion fails.
	names, err := w.KubeService.Rollback()
	if err != nil {
		log.Errorf("error deleting deployment/pvc: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete deployment/pvc: %v", err)})
		return
	}

	var server model.Server
	tx := w.HearthhubDb.Where("user_id = ?", user.ID).Delete(&server)
	if tx.Error != nil {
		log.Errorf("error deleting server from db: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error deleting server from db: %v", err)})
		return
	}

	w.HearthhubDb.Where("server_id = ?", server.ID).Delete(&model.WorldDetails{})

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("deleted resources successfully"),
		"resources": names,
	})
}
