package cognito

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/customer"
	"gorm.io/gorm"
	"io"
	"net/http"
)

type CreateUserRequestHandler struct{}

// HandleRequest This method handles the creation of a new cognito user after the user has finished the discord
// OAuth flow. It will return a Cognito refresh token AND access token which will be used by the Kraken service to authenticate a user
// in subsequent runs. In subsequent runs a user who is attempting to authenticate must use their refresh token to gain
// an access token.
func (h *CreateUserRequestHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService, db *gorm.DB) {
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
	var user model.User
	tx := db.Where("discord_id = ?", reqBody.DiscordID).First(&user)

	if tx.RowsAffected == 0 {
		log.Infof("no user found with id: %s, creating user", reqBody.DiscordID)
		creds, err := cognitoService.CreateCognitoUser(ctx, &reqBody)
		if err != nil {
			log.Errorf("error while creating new cognito user: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error while creating new cognito user:" + err.Error(),
			})
			return
		}

		cust, err := customer.New(&stripe.CustomerParams{
			Params: stripe.Params{},
			Email:  &reqBody.DiscordEmail,
			Metadata: map[string]string{
				"discord-id":       reqBody.DiscordID,
				"discord-username": reqBody.DiscordUsername,
			},
			Name: &reqBody.DiscordUsername,
		})

		if err != nil {
			log.Errorf("error while creating new stripe customer: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error while creating new stripe user: " + err.Error(),
			})
			return
		}

		newUser := model.User{
			DiscordUsername:    reqBody.DiscordUsername,
			Email:              reqBody.DiscordEmail,
			DiscordID:          reqBody.DiscordID,
			AvatarId:           reqBody.AvatarId,
			CustomerId:         cust.ID,
			SubscriptionId:     "",
			SubscriptionStatus: string(stripe.SubscriptionStatusUnpaid),
			SubscriptionLimits: model.SubscriptionLimits{},
			Credentials: model.CognitoCredentials{
				RefreshToken:    *creds.RefreshToken,
				AccessToken:     *creds.AccessToken,
				TokenExpiration: creds.ExpiresIn,
				IdToken:         *creds.IdToken,
			},
		}

		tx = db.Create(&newUser)
		if tx.Error != nil {
			log.Errorf("error while creating new user: %v", tx.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error while creating new user: " + tx.Error.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, newUser)
	} else {
		log.Infof("user already exists, refreshing session")
		creds, err := cognitoService.RefreshSession(ctx, reqBody.DiscordID)
		if err != nil {
			log.Errorf("error: failed to refresh existing user session: " + err.Error())
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("user with discord id: %s already exists. failed to refresh session", reqBody.DiscordID),
			})
			return
		}

		user.Credentials = model.CognitoCredentials{
			RefreshToken:    creds.RefreshToken,
			AccessToken:     creds.AccessToken,
			IdToken:         creds.IdToken,
			TokenExpiration: creds.TokenExpiration,
		}

		c.JSON(http.StatusOK, user)
	}
}
