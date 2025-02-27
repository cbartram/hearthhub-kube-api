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

func ConsumeMessageWithDelay(message service.Message, cognito service.CognitoService) {
	log.Infof("processing rabbitmq message type: %s", message.Type)

	switch message.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted", "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err := json.Unmarshal(message.Body, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			return
		}

		user, err := cognito.FindUserByAttribute(context.Background(), "custom:stripe_customer_id", subscription.Customer.ID)
		if err != nil {
			log.Errorf("failed to find user with customer id: %s, error: %v", subscription.ID, err)
			return
		}

		if user == nil {
			log.Errorf("no cognito user found with customer id: %s", subscription.ID)
			return
		}

		var attribute types.AttributeType
		if message.Type == "customer.subscription.created" {
			attribute = types.AttributeType{
				Name:  stripe.String("custom:stripe_sub_id"),
				Value: stripe.String(subscription.ID),
			}
		} else {
			// sub updates, pauses, and cancellations both affect the sub_status field and will update it to either:
			// "active", "paused" or "canceled"
			attribute = types.AttributeType{
				Name:  stripe.String("custom:stripe_sub_status"),
				Value: stripe.String(string(subscription.Status)),
			}
		}

		err = cognito.AdminUpdateUserAttributes(context.Background(), *user.Username, []types.AttributeType{attribute})

		if err != nil {
			log.Errorf("failed to update cognito with stripe subscription id: %s, error: %v", subscription.ID, err)
			return
		}
		log.Infof("subscription updated for user %s, id: %s, status: %s", *user.Username, subscription.ID, subscription.Status)
	}
}

func (w *WebhookHandler) HandleRequest(c *gin.Context, rabbitMQService *service.RabbitMqService) {
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

	eventType := string(event.Type)

	// Note: these cases are actually ordered in the timeline that webhook events are received
	switch eventType {
	case
		"customer.subscription.created",
		"customer.subscription.updated",
		"customer.subscription.paused",
		"customer.subscription.deleted",
		"customer.subscription.trial_will_end":
		log.Infof("enqueueing message with type: %s", eventType)
		err = rabbitMQService.PublishMessage(&service.Message{
			Type: eventType,
			Body: event.Data.Raw,
		})

		if err != nil {
			log.Errorf("failed to enqueue message with type: %s, error: %v", eventType, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue message"})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}
