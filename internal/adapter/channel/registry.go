package channel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.ChannelRegistry = (*Registry)(nil)

type registryEntry struct {
	label   string
	factory port.ChannelSenderFactory
	cb      *CircuitBreaker
}

type Registry struct {
	entries map[domain.ChannelType]registryEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[domain.ChannelType]registryEntry)}
}

func (r *Registry) Register(t domain.ChannelType, label string, f port.ChannelSenderFactory) {
	r.entries[t] = registryEntry{
		label:   label,
		factory: f,
		cb:      NewCircuitBreaker(5, 60*time.Second),
	}
}

// CreateSenderWithRetry creates a sender wrapped with retry + circuit breaker.
func (r *Registry) CreateSenderWithRetry(t domain.ChannelType, config json.RawMessage) (port.AlertSender, error) {
	entry, ok := r.entries[t]
	if !ok {
		return nil, fmt.Errorf("unknown channel type: %s", t)
	}
	inner, err := entry.factory.CreateSender(config)
	if err != nil {
		return nil, err
	}
	return NewRetryingSender(inner, entry.cb), nil
}

func (r *Registry) Get(t domain.ChannelType) (port.ChannelSenderFactory, error) {
	entry, ok := r.entries[t]
	if !ok {
		return nil, fmt.Errorf("unknown channel type: %s", t)
	}
	return entry.factory, nil
}

func (r *Registry) Types() []port.ChannelTypeInfo {
	types := make([]port.ChannelTypeInfo, 0, len(r.entries))
	for t, entry := range r.entries {
		types = append(types, port.ChannelTypeInfo{
			Type:   t,
			Label:  entry.label,
			Schema: entry.factory.ConfigSchema(),
		})
	}
	return types
}

func (r *Registry) ValidateConfig(t domain.ChannelType, raw json.RawMessage) error {
	factory, err := r.Get(t)
	if err != nil {
		return err
	}
	return factory.ValidateConfig(raw)
}
