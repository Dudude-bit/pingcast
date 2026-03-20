package checker

import (
	"encoding/json"
	"fmt"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.CheckerRegistry = (*Registry)(nil)

type registryEntry struct {
	label   string
	checker port.MonitorChecker
}

type Registry struct {
	entries map[domain.MonitorType]registryEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[domain.MonitorType]registryEntry)}
}

func (r *Registry) Register(t domain.MonitorType, label string, c port.MonitorChecker) {
	r.entries[t] = registryEntry{label: label, checker: c}
}

func (r *Registry) Get(t domain.MonitorType) (port.MonitorChecker, error) {
	entry, ok := r.entries[t]
	if !ok {
		return nil, fmt.Errorf("unknown monitor type: %s", t)
	}
	return entry.checker, nil
}

func (r *Registry) Types() []port.MonitorTypeInfo {
	types := make([]port.MonitorTypeInfo, 0, len(r.entries))
	for t, entry := range r.entries {
		types = append(types, port.MonitorTypeInfo{
			Type:   t,
			Label:  entry.label,
			Schema: entry.checker.ConfigSchema(),
		})
	}
	return types
}

func (r *Registry) ValidateConfig(t domain.MonitorType, raw json.RawMessage) error {
	checker, err := r.Get(t)
	if err != nil {
		return err
	}
	return checker.ValidateConfig(raw)
}

func (r *Registry) Target(t domain.MonitorType, raw json.RawMessage) string {
	checker, err := r.Get(t)
	if err != nil {
		return ""
	}
	return checker.Target(raw)
}

func (r *Registry) Host(t domain.MonitorType, raw json.RawMessage) string {
	checker, err := r.Get(t)
	if err != nil {
		return ""
	}
	return checker.Host(raw)
}
