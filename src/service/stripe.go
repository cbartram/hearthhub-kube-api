package service

import (
	"fmt"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/subscription"
	"os"
)

type StripeService struct{}

func MakeStripeService() *StripeService {
	return &StripeService{}
}

// VerifyActiveSubscription checks if a subscription is active for a given customer
func (s *StripeService) VerifyActiveSubscription(customerID string, subscriptionID string) (bool, error) {
	apiKey := os.Getenv("STRIPE_SECRET_KEY")
	if apiKey == "" {
		return false, fmt.Errorf("STRIPE_SECRET_KEY environment variable not set")
	}
	stripe.Key = apiKey

	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return false, fmt.Errorf("error retrieving subscription: %v", err)
	}

	if sub.Customer.ID != customerID {
		return false, fmt.Errorf("subscription does not belong to customer %s", customerID)
	}

	return sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing, nil
}
