package checker

import "sync"

type HostLimiter struct {
	mu    sync.Mutex
	semas map[string]chan struct{}
}

func NewHostLimiter() *HostLimiter {
	return &HostLimiter{
		semas: make(map[string]chan struct{}),
	}
}

func (h *HostLimiter) Acquire(host string) {
	h.mu.Lock()
	sema, ok := h.semas[host]
	if !ok {
		sema = make(chan struct{}, 1)
		h.semas[host] = sema
	}
	h.mu.Unlock()
	sema <- struct{}{}
}

func (h *HostLimiter) Release(host string) {
	h.mu.Lock()
	sema, ok := h.semas[host]
	h.mu.Unlock()
	if ok {
		<-sema
	}
}
