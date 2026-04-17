package httperr

import (
	"context"
	"errors"
	"net"

	"github.com/kirillinakin/pingcast/internal/domain"
)

// ClassifyNetError wraps a network-level error into a DeliveryError with proper reason.
func ClassifyNetError(err error) *domain.DeliveryError {
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return domain.NewDeliveryError("timeout", 0, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.NewDeliveryError("timeout", 0, err)
	}
	return domain.NewDeliveryError("network_error", 0, err)
}

// ClassifyHTTPStatus wraps an HTTP error response into a DeliveryError with proper reason.
func ClassifyHTTPStatus(statusCode int, err error) *domain.DeliveryError {
	switch {
	case statusCode == 401 || statusCode == 403:
		return domain.NewDeliveryError("auth_error", statusCode, err)
	case statusCode == 429:
		return domain.NewDeliveryError("rate_limited", statusCode, err)
	default:
		return domain.NewDeliveryError("unknown", statusCode, err)
	}
}
