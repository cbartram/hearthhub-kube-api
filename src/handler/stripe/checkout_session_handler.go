package stripe

import (
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"net/http"
	"os"
)

type CheckoutSessionHandler struct{}

func (h *CheckoutSessionHandler) HandleRequest(c *gin.Context) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	priceId, ok := c.GetQuery("priceId")

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "you must provide a priceId in the query parameters.",
		})
		return
	}

	host := util.GetHostname()

	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			// Provide the exact Price ID (for example, pr_1234) of the product you want to sell
			Price:    stripe.String(priceId),
			Quantity: stripe.Int64(1),
		},
		},
		Mode:         stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:   stripe.String(host + "?success=true"),
		CancelURL:    stripe.String(host + "?canceled=true"),
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
	}

	s, err := session.New(params)

	if err != nil {
		log.Errorf("failed to create new stripe session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to create new checkout session: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url": s.URL,
	})
}
