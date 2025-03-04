package server

import (
	"github.com/cbartram/hearthhub-common/model"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
)

type GetServerHandler struct{}

func (g *GetServerHandler) HandleRequest(c *gin.Context, db *gorm.DB) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)
	c.JSON(http.StatusOK, user.Servers)
}
