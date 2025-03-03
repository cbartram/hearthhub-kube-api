package stripe_handlers

import (
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
)

type WebhookHandler struct{}

func ConsumeMessageWithDelay(message service.Message, db *gorm.DB) {
	log.Infof("processing rabbitmq message type: %s", message.Type)

	switch message.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted", "customer.subscription.trial_will_end":
		var subscription stripe.Subscription
		err := json.Unmarshal(message.Body, &subscription)
		if err != nil {
			log.Errorf("error parsing webhook json: %v", err)
			return
		}

		var user model.User
		tx := db.Model(&model.User{}).Where("customer_id = ?", subscription.Customer.ID).First(&user)
		if tx.Error != nil {
			log.Errorf("failed to find user with customer id: %s, error: %v", subscription.Customer.ID, err)
			return
		}

		// Sub status for users are retrieved directly from stripe every time to ensure we have to maintain as little
		// stripe state as possible. Therefore, future webhooks like subscription updated, deleted, and trial will end which
		// mutate sub status can be ignored.
		if message.Type == "customer.subscription.created" {
			user.SubscriptionId = subscription.ID
		}

		tx = db.Save(&user)
		if tx.Error != nil {
			log.Errorf("failed to update user with stripe subscription id: %s, error: %v", subscription.ID, err)
			return
		}

		log.Infof("subscription updated for user %s, id: %s, status: %s", user.DiscordUsername, subscription.ID, subscription.Status)
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
