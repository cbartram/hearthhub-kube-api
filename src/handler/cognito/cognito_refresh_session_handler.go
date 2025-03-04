package cognito

import (
	"context"
	"fmt"
	"github.com/cbartram/hearthhub-common/model"
	common "github.com/cbartram/hearthhub-common/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type RefreshSessionHandler struct{}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *RefreshSessionHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService common.CognitoService) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	log.Infof("authenticating user with discord id: %s", user.DiscordID)
	creds, err := cognitoService.RefreshSession(ctx, user.Credentials.RefreshToken)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("error: failed to refresh user session: %v", err),
		})
	}

	c.JSON(http.StatusOK, creds)
}
