package cognito

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/src/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type AuthHandler struct{}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *AuthHandler) HandleRequest(c *gin.Context, ctx context.Context, wrapper *service.Wrapper) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	log.Infof("authenticating user with discord id: %s", user.DiscordID)
	user, err := wrapper.CognitoService.AuthUser(ctx, &user.Credentials.RefreshToken, &user.DiscordID, wrapper.HearthhubDb)
	if err != nil {
		log.Errorf("user is unauthorized: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user unauthorized",
		})
		return
	}

	status, err := wrapper.StripeService.GetActiveSubscription(user.CustomerId, user.SubscriptionId)
	if err != nil {
		log.Errorf("unable to verify stripe subscription status: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unable to verify stripe subscription status",
		})
		return
	}

	limits, err := wrapper.StripeService.GetSubscriptionLimits(user.SubscriptionId)
	if err != nil {
		log.Errorf("failed to get sub limits for sub id: %s, error: %v", user.SubscriptionId, err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "failed to parse subscription limits",
		})
		return
	}

	user.SubscriptionLimits = *limits
	user.SubscriptionStatus = status
	c.JSON(http.StatusOK, user)
}
