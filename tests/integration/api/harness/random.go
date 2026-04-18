//go:build integration

package harness

import (
	"crypto/rand"
	"sync"
)

// FakeRandom wraps a deterministic byte queue when Queue() was used,
// otherwise falls back to crypto/rand.
type FakeRandom struct {
	mu   sync.Mutex
	next [][]byte
}

func NewFakeRandom() *FakeRandom { return &FakeRandom{} }

// Queue stores an exact byte sequence returned on the next Read() call.
func (r *FakeRandom) Queue(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next = append(r.next, append([]byte(nil), b...))
}

func (r *FakeRandom) Read(p []byte) (int, error) {
	r.mu.Lock()
	if len(r.next) > 0 {
		b := r.next[0]
		r.next = r.next[1:]
		r.mu.Unlock()
		n := copy(p, b)
		return n, nil
	}
	r.mu.Unlock()
	return rand.Read(p)
}
