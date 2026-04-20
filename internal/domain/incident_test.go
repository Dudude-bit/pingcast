package domain

import "testing"

func TestIncidentState_Valid(t *testing.T) {
	tests := []struct {
		s    IncidentState
		want bool
	}{
		{IncidentStateInvestigating, true},
		{IncidentStateIdentified, true},
		{IncidentStateMonitoring, true},
		{IncidentStateResolved, true},
		{"", false},
		{"garbage", false},
	}
	for _, tt := range tests {
		if got := tt.s.Valid(); got != tt.want {
			t.Errorf("%q.Valid() = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestIncidentState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    IncidentState
		to      IncidentState
		wantErr bool
	}{
		// Forward moves always OK.
		{IncidentStateInvestigating, IncidentStateIdentified, false},
		{IncidentStateInvestigating, IncidentStateMonitoring, false},
		{IncidentStateInvestigating, IncidentStateResolved, false},
		{IncidentStateIdentified, IncidentStateMonitoring, false},
		{IncidentStateIdentified, IncidentStateResolved, false},
		{IncidentStateMonitoring, IncidentStateResolved, false},
		// Same-state is a no-op, not an error (an update post at the same
		// state just adds narrative without advancing).
		{IncidentStateInvestigating, IncidentStateInvestigating, false},
		// Rewind is permitted only from monitoring → identified.
		{IncidentStateMonitoring, IncidentStateIdentified, false},
		// Everything else backwards is rejected.
		{IncidentStateResolved, IncidentStateInvestigating, true},
		{IncidentStateResolved, IncidentStateIdentified, true},
		{IncidentStateMonitoring, IncidentStateInvestigating, true},
		{IncidentStateIdentified, IncidentStateInvestigating, true},
		// Garbage target.
		{IncidentStateInvestigating, "garbage", true},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := tt.from.CanTransitionTo(tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
