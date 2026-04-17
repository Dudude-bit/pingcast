package channel

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.ChannelRegistry = (*Registry)(nil)

type registryEntry struct {
	label   string
	factory port.ChannelSenderFactory
}

// cbEntry wraps a circuit breaker with an atomic last-accessed timestamp for TTL eviction.
type cbEntry struct {
	cb         *gobreaker.CircuitBreaker[any]
	lastAccess atomic.Int64 // unix timestamp
}

type Registry struct {
	entries map[domain.ChannelType]registryEntry
	cbs     sync.Map
	cbTTL   time.Duration
	stop    chan struct{}
}

func NewRegistry() *Registry {
	r := &Registry{
		entries: make(map[domain.ChannelType]registryEntry),
		cbTTL:   1 * time.Hour,
		stop:    make(chan struct{}),
	}
	go r.evictLoop()
	return r
}

func (r *Registry) Close() {
	close(r.stop)
}

func (r *Registry) Register(t domain.ChannelType, label string, f port.ChannelSenderFactory) {
	r.entries[t] = registryEntry{
		label:   label,
		factory: f,
	}
}

// CreateSender creates a sender wrapped with retry + per-channel circuit breaker.
func (r *Registry) CreateSender(t domain.ChannelType, channelID uuid.UUID, config json.RawMessage) (port.AlertSender, error) {
	entry, ok := r.entries[t]
	if !ok {
		return nil, fmt.Errorf("unknown channel type: %s", t)
	}
	inner, err := entry.factory.CreateSender(config)
	if err != nil {
		return nil, err
	}
	cb := r.getOrCreateCB(t, channelID)
	return NewRetryingSender(inner, cb), nil
}

func (r *Registry) getOrCreateCB(t domain.ChannelType, channelID uuid.UUID) *gobreaker.CircuitBreaker[any] {
	key := fmt.Sprintf("%s:%s", t, channelID)
	now := time.Now().Unix()

	if v, ok := r.cbs.Load(key); ok {
		//nolint:errcheck // type assertion safe: only *cbEntry is Stored into r.cbs
		e := v.(*cbEntry)
		e.lastAccess.Store(now)
		return e.cb
	}

	cb := NewCircuitBreaker(key, 5, 60*time.Second)
	e := &cbEntry{cb: cb}
	e.lastAccess.Store(now)
	actual, _ := r.cbs.LoadOrStore(key, e)
	//nolint:errcheck // type assertion safe: only *cbEntry is Stored into r.cbs
	return actual.(*cbEntry).cb
}

func (r *Registry) evictLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-r.cbTTL).Unix()
			r.cbs.Range(func(key, value any) bool {
				//nolint:errcheck // type assertion safe: only *cbEntry is Stored into r.cbs
				if e := value.(*cbEntry); e.lastAccess.Load() < cutoff {
					r.cbs.Delete(key)
				}
				return true
			})
		}
	}
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
