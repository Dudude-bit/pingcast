package port

import (
	"context"

	"github.com/kirillinakin/pingcast/internal/domain"
)

type AlertSender interface {
	Send(ctx context.Context, event *domain.AlertEvent) error
}
