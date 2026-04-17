package checker

import "testing"

func TestHTTPChecker_ClientForTimeout_Caches(t *testing.T) {
	chk := NewHTTPChecker()

	a := chk.clientForTimeout(10)
	b := chk.clientForTimeout(10)
	if a != b {
		t.Fatalf("expected same *http.Client on repeated calls; got different pointers")
	}

	c := chk.clientForTimeout(30)
	if c == a {
		t.Fatalf("expected distinct *http.Client for different timeouts")
	}

	// Re-request the first timeout — must return the original cached instance.
	a2 := chk.clientForTimeout(10)
	if a2 != a {
		t.Fatalf("cache evicted unexpectedly")
	}
}
