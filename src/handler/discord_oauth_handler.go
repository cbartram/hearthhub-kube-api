package handler

import (
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type DiscordRequestHandler struct{}

// HandleRequest Handles the /api/v1/discord-oauth route which the service calls to trade a code for an OAuth
// access token.
func (h *DiscordRequestHandler) HandleRequest(c *gin.Context, discordService *service.DiscordService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody map[string]string
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	code := reqBody["code"]

	if reqBody["code"] == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "access code: 'code' is required",
		})
		return
	}

	log.Infof("exchanging code: %s for oauth access token with origin: %s/discord/oauth", code, c.Request.Header.Get("Origin"))
	token, err := discordService.ExchangeCodeForToken(code, c.Request.Header.Get("Origin"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to exchange code: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, service.DiscordTokenResponse{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		ExpiresIn:    token.ExpiresIn,
		RefreshToken: token.RefreshToken,
		Scope:        token.Scope,
	})
}
