package cognito

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

type CreateUserRequestHandler struct{}

// HandleRequest This method handles the creation of a new cognito user after the user has finished the discord
// OAuth flow. It will return a Cognito refresh token AND access token which will be used by the Kraken service to authenticate a user
// in subsequent runs. In subsequent runs a user who is attempting to authenticate must use their refresh token to gain
// an access token.
func (h *CreateUserRequestHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody service.CognitoCreateUserRequest
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// We want to assert that the user does not exist before we create it.
	user, _ := cognitoService.FindUserByAttribute(context.Background(), "name", reqBody.DiscordID)
	if user == nil {
		creds, err := cognitoService.CreateCognitoUser(ctx, &reqBody)
		if err != nil {
			log.Errorf("error while creating new cognito user: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error while creating new cognito user:" + err.Error(),
			})
			return
		}

		// Note: this does not provide the cognito id. However, users are located via username (discord id) not cognito id.
		c.JSON(http.StatusOK, service.CognitoUser{
			DiscordUsername:  reqBody.DiscordUsername,
			Email:            reqBody.DiscordEmail,
			DiscordID:        reqBody.DiscordID,
			AvatarId:         reqBody.AvatarId,
			InstalledMods:    map[string]bool{}, // A user has no mods installed when first created so this is safe
			InstalledBackups: map[string]bool{},
			InstalledConfig:  map[string]bool{},
			Credentials: service.CognitoCredentials{
				RefreshToken:    *creds.RefreshToken,
				AccessToken:     *creds.AccessToken,
				TokenExpiration: creds.ExpiresIn,
				IdToken:         *creds.IdToken,
			},
		})
	} else {
		// User already exists.
		log.Infof("user already exists, refreshing session")
		creds, err := cognitoService.RefreshSession(ctx, reqBody.DiscordID)
		if err != nil {
			log.Errorf("error: failed to refresh existing user session: " + err.Error())
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("user with discord id: %s already exists. failed to refresh session", reqBody.DiscordID),
			})
			return
		}

		var installedMods, installedBackups, installedConfig map[string]bool
		for _, attr := range user.Attributes {
			if *attr.Name == "custom:installed_mods" {
				json.Unmarshal([]byte(*attr.Value), &installedMods)
			} else if *attr.Name == "custom:installed_backups" {
				json.Unmarshal([]byte(*attr.Value), &installedBackups)
			} else if *attr.Name == "custom:installed_config" {
				json.Unmarshal([]byte(*attr.Value), &installedConfig)
			}
		}

		c.JSON(http.StatusOK, service.CognitoUser{
			DiscordUsername:  reqBody.DiscordUsername,
			Email:            reqBody.DiscordEmail,
			DiscordID:        reqBody.DiscordID,
			AvatarId:         reqBody.AvatarId,
			InstalledMods:    installedMods,
			InstalledBackups: installedBackups,
			InstalledConfig:  installedConfig,
			Credentials:      *creds,
		})
	}
}
