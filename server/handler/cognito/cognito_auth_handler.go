package cognito

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type AuthHandler struct{}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *AuthHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	log.Infof("authenticating user with discord id: %s", user.DiscordID)
	cognitoUser, err := cognitoService.AuthUser(ctx, &user.Credentials.RefreshToken, &user.DiscordID)
	if err != nil {
		log.Errorf("user is unauthorized: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user unauthorized",
		})
	}

	log.Infof("user auth ok")
	c.JSON(http.StatusOK, cognitoUser)
}
