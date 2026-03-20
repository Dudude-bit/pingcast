package port

import "context"

type AlertSender interface {
	NotifyDown(ctx context.Context, monitorName, monitorTarget, cause string) error
	NotifyUp(ctx context.Context, monitorName, monitorTarget string) error
}
