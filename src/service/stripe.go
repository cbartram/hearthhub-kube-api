package service

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/productfeature"
	"github.com/stripe/stripe-go/v81/subscription"
	"os"
	"strconv"
	"strings"
)

type StripeService struct{}

type SubscriptionLimits struct {
	CpuLimit            int  `json:"cpuLimit"`
	MemoryLimit         int  `json:"memoryLimit"`
	MaxBackups          int  `json:"maxBackups"`
	MaxWorlds           int  `json:"maxWorlds"`
	ExistingWorldUpload bool `json:"existingWorldUpload"`
}

func MakeStripeService() *StripeService {
	apiKey := os.Getenv("STRIPE_SECRET_KEY")
	if apiKey == "" {
		log.Errorf("STRIPE_SECRET_KEY environment variable not set")
	}

	stripe.Key = apiKey
	return &StripeService{}
}

// VerifyActiveSubscription checks if a subscription is active for a given customer
func (s *StripeService) VerifyActiveSubscription(customerId string, subscriptionId string) (bool, error) {
	sub, err := subscription.Get(subscriptionId, nil)
	if err != nil {
		return false, fmt.Errorf("error retrieving subscription: %v", err)
	}

	if sub.Customer.ID != customerId {
		return false, fmt.Errorf("subscription does not belong to customer %s", customerId)
	}

	return sub.Status == stripe.SubscriptionStatusActive || sub.Status == stripe.SubscriptionStatusTrialing, nil
}

func (s *StripeService) GetSubscriptionLimits(subscriptionId string) (*SubscriptionLimits, error) {
	sub, err := subscription.Get(subscriptionId, nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving subscription: %v", err)
	}

	if len(sub.Items.Data) == 0 || len(sub.Items.Data) > 1 {
		return nil, fmt.Errorf("unexpected number of subscription items: %d", len(sub.Items.Data))
	}

	limits := SubscriptionLimits{
		ExistingWorldUpload: false,
	}
	productId := sub.Items.Data[0].Price.Product.ID

	features := productfeature.List(&stripe.ProductFeatureListParams{
		Product: stripe.String(productId),
	})

	for features.Next() {
		name := features.ProductFeature().EntitlementFeature.Name
		if strings.Contains(name, "GB RAM") {
			limit, _ := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(name, " GB RAM")))
			limits.MemoryLimit = limit
		} else if strings.Contains(name, "CPU Cores") {
			limit, _ := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(name, " CPU Cores")))
			limits.CpuLimit = limit
		} else if strings.Contains(name, "Backups") {
			limit, _ := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(name, " Backups")))
			limits.MaxBackups = limit
		} else if strings.Contains(name, "Worlds") {
			limit, _ := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(name, " Worlds")))
			limits.MaxWorlds = limit
		} else if strings.Contains(name, "Existing World Upload") {
			limits.ExistingWorldUpload = true
		}
	}

	return &limits, nil
}
