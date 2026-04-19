//go:build integration

package harness

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// FakeTarget is an httptest.Server whose response can be scripted:
// chain RespondWith/FailNext/Slow to build a queue of behaviours. The
// last queued response repeats once the queue drains.
type FakeTarget struct {
	Server *httptest.Server
	URL    string

	mu    sync.Mutex
	queue []response
	hits  int
}

type response struct {
	status int
	delay  time.Duration
}

// NewFakeTarget starts a target that returns 200 by default. Tests
// override the default by calling RespondWith/FailNext/Slow — the
// first such call clears the default and enqueues the requested
// behaviour.
func NewFakeTarget(t *testing.T) *FakeTarget {
	t.Helper()
	ft := &FakeTarget{}
	ft.Server = httptest.NewServer(http.HandlerFunc(ft.handle))
	ft.URL = ft.Server.URL
	t.Cleanup(ft.Server.Close)
	return ft
}

func (f *FakeTarget) handle(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	f.hits++
	r := response{status: 200}
	if len(f.queue) > 1 {
		r = f.queue[0]
		f.queue = f.queue[1:]
	} else if len(f.queue) == 1 {
		r = f.queue[0]
	}
	f.mu.Unlock()

	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	w.WriteHeader(r.status)
}

func (f *FakeTarget) RespondWith(status int) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, response{status: status})
	return f
}

// FailNext enqueues n 500 responses, after which the tail of the
// queue resumes repeating.
func (f *FakeTarget) FailNext(n int) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < n; i++ {
		f.queue = append(f.queue, response{status: 500})
	}
	return f
}

func (f *FakeTarget) Slow(d time.Duration) *FakeTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, response{status: 200, delay: d})
	return f
}

func (f *FakeTarget) Hits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}
