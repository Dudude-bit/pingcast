package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/port"
)

// BillingService enforces the founder-price cap and marks subscriptions
// with their LemonSqueezy variant. Founder seats are a real scarcity
// promise (spec §5, §11) so the cap is the source of truth, not
// marketing copy — webhook sets `subscription_variant`, pricing page
// and dashboard both query this service to decide which checkout
// link to expose.
type BillingService struct {
	users      port.UserRepo
	founderCap int
}

func NewBillingService(users port.UserRepo, founderCap int) *BillingService {
	return &BillingService{users: users, founderCap: founderCap}
}

// FounderStatus is the shape returned from /api/billing/founder-status.
type FounderStatus struct {
	Available bool
	Used      int64
	Cap       int
}

func (s *BillingService) FounderStatus(ctx context.Context) (FounderStatus, error) {
	used, err := s.users.CountActiveFounderSubscriptions(ctx)
	if err != nil {
		return FounderStatus{}, err
	}
	return FounderStatus{
		Available: used < int64(s.founderCap),
		Used:      used,
		Cap:       s.founderCap,
	}, nil
}

// SetSubscriptionVariant records which variant a Pro sub landed on so
// CountActiveFounderSubscriptions can enforce the cap. Called from the
// LemonSqueezy webhook when subscription_created fires.
func (s *BillingService) SetSubscriptionVariant(ctx context.Context, userID uuid.UUID, variant string) error {
	return s.users.SetSubscriptionVariant(ctx, userID, variant)
}
