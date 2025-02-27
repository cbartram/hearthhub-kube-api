package stripe_handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"io"
	"net/http"
	"os"
)

type WebhookHandler struct{}

func (w *WebhookHandler) HandleRequest(c *gin.Context, cognito service.CognitoService) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	endpointSecret := os.Getenv("STRIPE_ENDPOINT_SECRET")
	signatureHeader := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
	if err != nil {
		log.Errorf("webhook signature verification failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Webhook signature verification failed: %v", err)})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}

		discordId := session.Metadata["discordId"]
		log.Infof("stripe sub status: %s", string(session.Subscription.Status))
		err = cognito.AdminUpdateUserAttributes(context.Background(), discordId, []types.AttributeType{
			{
				Name:  stripe.String("custom:stripe_customer_id"),
				Value: stripe.String(session.Customer.ID),
			},
			{
				Name:  stripe.String("custom:stripe_sub_id"),
				Value: stripe.String(session.Subscription.ID),
			},
			{
				Name:  stripe.String("custom:stripe_sub_status"),
				Value: stripe.String(string(session.Subscription.Status)),
			},
		})

		if err != nil {
			log.Errorf("failed to reconcile stripe_handlers data with cognito attributes: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to reconcile stripe_handlers data with cognito attributes: %v", err)})
			return
		}
	case "customer.subscription.deleted":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}
		log.Infof("Subscription deleted for %s.", subscription.ID)
	case "customer.subscription.updated":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}
		log.Infof("Subscription updated for %s.", subscription.ID)
	case "customer.subscription.created":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}
		log.Infof("Subscription created for %s", subscription.ID)
	case "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}
		log.Infof("Subscription trial will end for %s.", subscription.ID)
	case "entitlements.active_entitlement_summary.updated":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing webhook json: %v", err)})
			return
		}
		log.Infof("Active entitlement summary updated for %s.", subscription.ID)
	default:
		log.Infof("unknown event type: %s", event.Type)
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}
