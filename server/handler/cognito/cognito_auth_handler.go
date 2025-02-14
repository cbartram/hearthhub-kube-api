package cognito

import (
	"context"
	"encoding/json"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type AuthHandler struct{}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *AuthHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody service.CognitoAuthRequest
	if err = json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if reqBody.DiscordID == "" || reqBody.RefreshToken == "" {
		log.Errorf("error: discord id '%s' or refresh token: '%s' missing from request body: ", reqBody.DiscordID, reqBody.RefreshToken)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: discordId or refreshToken missing."})
		return
	}

	log.Infof("authenticating user with discord id: %s", reqBody.DiscordID)
	cognitoUser, err := cognitoService.AuthUser(ctx, &reqBody.RefreshToken, &reqBody.DiscordID)
	if err != nil {
		log.Errorf("user is unauthorized: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user unauthorized",
		})
	}

	log.Infof("user auth ok")
	c.JSON(http.StatusOK, cognitoUser)
}
