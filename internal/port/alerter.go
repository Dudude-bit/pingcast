package port

import "context"

type AlertSender interface {
	NotifyDown(ctx context.Context, monitorName, monitorURL, cause string) error
	NotifyUp(ctx context.Context, monitorName, monitorURL string) error
}
