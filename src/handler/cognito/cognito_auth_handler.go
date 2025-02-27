package cognito

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type AuthHandler struct{}
type AuthorizationLevel string

var (
	NoAuth               AuthorizationLevel = "NO_AUTH"
	CognitoAuth          AuthorizationLevel = "COGNITO_AUTH"
	CognitoAndStripeAuth AuthorizationLevel = "COGNITO_STRIPE_AUTH"
)

// This mapping is VERY important. It specifies the level of authorization required to access each resource (frontend page).
var authorizationMap = map[string]AuthorizationLevel{
	"pricing":   CognitoAuth,
	"dashboard": CognitoAndStripeAuth,
	"landing":   NoAuth,
	"login":     NoAuth,
}

// HandleRequest Authenticates that a refresh token is valid for a given user id. This returns the entire
// user object with a refreshed access token.
func (h *AuthHandler) HandleRequest(c *gin.Context, ctx context.Context, cognitoService service.CognitoService, stripeService *service.StripeService) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	resource, ok := c.GetQuery("resource")
	if !ok {
		log.Errorf("no resource specified to auth access")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no resource specified"})
		return
	}

	authorizationValue, exists := authorizationMap[resource]
	if !exists {
		log.Errorf("invalid resource")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource specified"})
		return
	}

	log.Infof("authenticating user with discord id: %s", user.DiscordID)
	cognitoUser, err := cognitoService.AuthUser(ctx, &user.Credentials.RefreshToken, &user.DiscordID)
	if err != nil {
		log.Errorf("user is unauthorized: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user unauthorized",
		})
	}

	// The user does not need stripe sub to access the resource
	if authorizationValue == CognitoAuth || authorizationValue == NoAuth {
		log.Infof("user auth ok, no stripe sub required for resource: %s", resource)
		c.JSON(http.StatusOK, cognitoUser)
		return
	}

	// Else we know it requires both cognito and stripe so proceed to verify stripe
	ok, err = stripeService.VerifyActiveSubscription(cognitoUser.CustomerId, cognitoUser.SubscriptionId)
	if err != nil {
		log.Errorf("unable to verify stripe subscription status: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unable to verify stripe subscription status",
		})
		return
	}

	if !ok {
		log.Errorf("invalid or expired stripe subscription: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid subscription",
		})
		return
	}

	log.Infof("user auth ok -- stripe sub verified for resource: %s", resource)
	c.JSON(http.StatusOK, cognitoUser)
}
