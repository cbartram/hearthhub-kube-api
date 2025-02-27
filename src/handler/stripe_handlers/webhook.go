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
	switch message.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		err := json.Unmarshal(message.Body, &session)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			return
		}
		log.Infof("checkout session completed %s.", session.ID)

		discordId := session.Metadata["discordId"]
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
				Value: stripe.String("unknown"), // Sub status is unknown at this point
			},
		})

		if err != nil {
			log.Errorf("failed to reconcile stripe_handlers data with cognito attributes: %v", err)
			return
		}
	case "customer.subscription.updated", "customer.subscription.deleted", "customer.subscription.created", "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err := json.Unmarshal(message.Body, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			return
		}

		user, err := cognito.FindUserByAttribute(context.Background(), "custom:stripe_sub_id", subscription.ID)
		if err != nil {
			log.Errorf("failed to find user with subscription ID %s: %v", subscription.ID, err)
			return
		}

		if user == nil {
			log.Errorf("no user found with subscription ID: %s", subscription.ID)
			return
		}

		if message.Type == "customer.subscription.updated" {
			err = cognito.AdminUpdateUserAttributes(context.Background(), *user.Username, []types.AttributeType{
				{
					Name:  stripe.String("custom:stripe_sub_status"),
					Value: stripe.String(string(subscription.Status)),
				},
			})
			log.Infof("subscription updated for user %s, status: %s", *user.Username, subscription.Status)
		} else if message.Type == "customer.subscription.deleted" {
			err = cognito.AdminUpdateUserAttributes(context.Background(), *user.Username, []types.AttributeType{
				{
					Name:  stripe.String("custom:stripe_sub_status"),
					Value: stripe.String("canceled"),
				},
			})

			log.Infof("subscription canceled for user %s", *user.Username)
		}

		if err != nil {
			log.Errorf("failed to update user attributes: %v", err)
			return
		}
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

	switch eventType {
	case "checkout.session.completed",
		"customer.subscription.updated",
		"customer.subscription.deleted",
		"customer.subscription.created",
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
