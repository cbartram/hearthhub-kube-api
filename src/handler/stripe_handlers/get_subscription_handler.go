package stripe_handlers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/subscription"
	"net/http"
)

type GetSubscriptionHandler struct{}

func (g *GetSubscriptionHandler) HandleRequest(c *gin.Context) {
	subId, ok := c.GetQuery("id")

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "you must provide a \"id\" in the query parameters.",
		})
		return
	}

	sub, err := subscription.Get(subId, &stripe.SubscriptionParams{})
	if err != nil {
		log.Errorf("failed to get subscription with id: %s, error: %v", subId, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to get subscription with id: %s, error: %v", subId, err),
		})
		return
	}

	c.JSON(http.StatusOK, sub)
}
