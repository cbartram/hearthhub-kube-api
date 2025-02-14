package cognito

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type RefreshSessionHandler struct{}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *RefreshSessionHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not read body from request: %v", err)})
		return
	}

	var reqBody service.CognitoAuthRequest
	if err = json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if reqBody.DiscordID == "" {
		log.Errorf("error: discord id '%s' missing from request body: ", reqBody.DiscordID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: discordId or refreshToken missing."})
		return
	}

	log.Infof("authenticating user with discord id: %s", reqBody.DiscordID)
	creds, err := cognitoService.RefreshSession(ctx, reqBody.RefreshToken)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("error: failed to refresh user session: %v", err),
		})
	}

	log.Infof("user auth ok")
	c.JSON(http.StatusOK, creds)
}
