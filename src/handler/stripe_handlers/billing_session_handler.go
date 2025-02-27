package stripe_handlers

import (
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	portalsession "github.com/stripe/stripe-go/v81/billingportal/session"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"net/http"
	"os"
)

type BillingSessionHandler struct{}

func (h *BillingSessionHandler) HandleRequest(c *gin.Context) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	sessionId, ok := c.GetQuery("sessionId")

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "you must provide a \"sessionId\" in the query parameters.",
		})
		return
	}

	s, err := session.Get(sessionId, nil)

	if err != nil {
		log.Errorf("failed to find stripe_handlers session: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to find stripe_handlers checkout session for billing with id: %s", sessionId),
		})
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(s.Customer.ID),
		ReturnURL: stripe.String(util.GetHostname() + "/pricing?success=true&session_id=" + sessionId),
	}
	ps, err := portalsession.New(params)
	if err != nil {
		log.Errorf("failed to create billing portal session: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to create billing portal session: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url": ps.URL,
	})
}
