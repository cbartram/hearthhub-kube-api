package stripe_handlers

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/price"
	"net/http"
	"os"
)

type CheckoutSessionHandler struct{}

func (h *CheckoutSessionHandler) HandleRequest(c *gin.Context, cognito service.CognitoService) {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	lookupKey, ok := c.GetQuery("key")
	var foundPrice *stripe.Price

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "you must provide a \"key\" in the query parameters.",
		})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	i := price.List(&stripe.PriceListParams{
		LookupKeys: stripe.StringSlice([]string{
			lookupKey,
		}),
	})

	for i.Next() {
		p := i.Price()
		foundPrice = p
	}

	if foundPrice == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("no price found for key: %s", lookupKey),
		})
		return
	}

	if user.CustomerId == "" {
		// Create a Stripe customer first
		customerParams := &stripe.CustomerParams{
			Email: stripe.String(user.Email),
			Metadata: map[string]string{
				"discordId": user.DiscordID,
			},
		}

		createdCustomer, err := customer.New(customerParams)
		if err != nil {
			log.Errorf("failed to create new stripe customer: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to create new stripe customer: %v", err),
			})
			return
		}

		user.CustomerId = createdCustomer.ID
		log.Infof("created customer with id: %s", user.CustomerId)
	} else {
		log.Infof("customer already exists with id: %s", user.CustomerId)
	}

	host := util.GetHostname()
	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(foundPrice.ID),
			Quantity: stripe.Int64(1),
		},
		},
		Customer:          stripe.String(user.CustomerId),
		ClientReferenceID: stripe.String(user.DiscordID),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(host + "/pricing?success=true&session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String(host + "/pricing?canceled=true"),
		AutomaticTax:      &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(false)},
		Metadata: map[string]string{
			"discordId": user.DiscordID,
		},
	}

	s, err := session.New(params)

	if err != nil {
		log.Errorf("failed to create new stripe_handlers session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to create new checkout session: %v", err),
		})
		return
	}

	err = cognito.AdminUpdateUserAttributes(context.Background(), user.DiscordID, []types.AttributeType{
		{
			Name:  stripe.String("custom:stripe_customer_id"),
			Value: stripe.String(user.CustomerId),
		},
	})

	if err != nil {
		log.Errorf("failed to update customer id in cognito: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to update customer id in cognito: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url": s.URL,
	})
}
