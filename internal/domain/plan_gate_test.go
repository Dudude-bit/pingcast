package domain

import "testing"

func TestRequiresPro(t *testing.T) {
	tests := []struct {
		name string
		plan Plan
		want bool
	}{
		{"free user denied", PlanFree, true},
		{"pro user allowed", PlanPro, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiresPro(tt.plan); got != tt.want {
				t.Fatalf("RequiresPro(%v) = %v, want %v", tt.plan, got, tt.want)
			}
		})
	}
}
