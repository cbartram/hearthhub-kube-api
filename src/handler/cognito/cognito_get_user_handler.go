package cognito

import (
	"context"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type GetUserHandler struct{}

// HandleRequest Retrieves a user from Cognito.
func (h *GetUserHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService) {
	discordID := c.Query("discordId")
	if discordID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "discordId query parameter is required",
		})
		return
	}

	log.Infof("retrieving user with id: %s from cognito", discordID)
	// Note: This method does not return credentials with the user
	cognitoUser, err := cognitoService.GetUser(ctx, &discordID)

	if err == nil {
		c.JSON(http.StatusOK, cognitoUser)
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("user with id: %s does not exist", discordID),
		})
	}
}
