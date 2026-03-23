package domain

import (
	"strings"
	"testing"
)

func TestValidateMonitorInput(t *testing.T) {
	tests := []struct {
		name               string
		monitorName        string
		intervalSeconds    int
		alertAfterFailures int
		wantErr            string
	}{
		{
			name:               "valid input",
			monitorName:        "My Monitor",
			intervalSeconds:    60,
			alertAfterFailures: 3,
		},
		{
			name:               "empty name",
			monitorName:        "",
			intervalSeconds:    60,
			alertAfterFailures: 3,
			wantErr:            "name is required",
		},
		{
			name:               "whitespace-only name",
			monitorName:        "   ",
			intervalSeconds:    60,
			alertAfterFailures: 3,
			wantErr:            "name is required",
		},
		{
			name:               "name too long",
			monitorName:        strings.Repeat("a", 256),
			intervalSeconds:    60,
			alertAfterFailures: 3,
			wantErr:            "name must be at most 255 characters",
		},
		{
			name:               "name exactly 255 chars is valid",
			monitorName:        strings.Repeat("a", 255),
			intervalSeconds:    60,
			alertAfterFailures: 3,
		},
		{
			name:               "interval too low",
			monitorName:        "My Monitor",
			intervalSeconds:    10,
			alertAfterFailures: 3,
			wantErr:            "interval must be at least 30 seconds",
		},
		{
			name:               "interval at minimum boundary",
			monitorName:        "My Monitor",
			intervalSeconds:    30,
			alertAfterFailures: 3,
		},
		{
			name:               "interval too high",
			monitorName:        "My Monitor",
			intervalSeconds:    86401,
			alertAfterFailures: 3,
			wantErr:            "interval must be at most 24 hours",
		},
		{
			name:               "interval at maximum boundary",
			monitorName:        "My Monitor",
			intervalSeconds:    86400,
			alertAfterFailures: 3,
		},
		{
			name:               "alertAfterFailures too low",
			monitorName:        "My Monitor",
			intervalSeconds:    60,
			alertAfterFailures: 0,
			wantErr:            "alert_after_failures must be at least 1",
		},
		{
			name:               "alertAfterFailures too high",
			monitorName:        "My Monitor",
			intervalSeconds:    60,
			alertAfterFailures: 101,
			wantErr:            "alert_after_failures must be at most 100",
		},
		{
			name:               "alertAfterFailures at maximum boundary",
			monitorName:        "My Monitor",
			intervalSeconds:    60,
			alertAfterFailures: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMonitorInput(tt.monitorName, tt.intervalSeconds, tt.alertAfterFailures)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr string
	}{
		{
			name:  "valid email",
			email: "user@example.com",
		},
		{
			name:  "valid email with subdomain",
			email: "admin@mail.example.co.uk",
		},
		{
			name:    "empty email",
			email:   "",
			wantErr: "email is required",
		},
		{
			name:    "invalid format no at sign",
			email:   "not-an-email",
			wantErr: "invalid email format",
		},
		{
			name:    "invalid format no domain",
			email:   "user@",
			wantErr: "invalid email format",
		},
		{
			name:    "invalid format spaces",
			email:   "user @example.com",
			wantErr: "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateChannelInput(t *testing.T) {
	tests := []struct {
		name        string
		channelName string
		channelType ChannelType
		wantErr     string
	}{
		{
			name:        "valid telegram channel",
			channelName: "My Alerts",
			channelType: ChannelTelegram,
		},
		{
			name:        "valid email channel",
			channelName: "Email Alerts",
			channelType: ChannelEmail,
		},
		{
			name:        "valid webhook channel",
			channelName: "Webhook Alerts",
			channelType: ChannelWebhook,
		},
		{
			name:        "empty name",
			channelName: "",
			channelType: ChannelTelegram,
			wantErr:     "channel name is required",
		},
		{
			name:        "whitespace-only name",
			channelName: "   ",
			channelType: ChannelTelegram,
			wantErr:     "channel name is required",
		},
		{
			name:        "name too long",
			channelName: strings.Repeat("x", 256),
			channelType: ChannelTelegram,
			wantErr:     "channel name must be at most 255 characters",
		},
		{
			name:        "invalid channel type",
			channelName: "My Channel",
			channelType: ChannelType("sms"),
			wantErr:     "invalid channel type",
		},
		{
			name:        "empty channel type",
			channelName: "My Channel",
			channelType: ChannelType(""),
			wantErr:     "invalid channel type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChannelInput(tt.channelName, tt.channelType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidEnum(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		allowed []string
		want    bool
	}{
		{
			name:    "contains",
			value:   "b",
			allowed: []string{"a", "b", "c"},
			want:    true,
		},
		{
			name:    "not contains",
			value:   "d",
			allowed: []string{"a", "b", "c"},
			want:    false,
		},
		{
			name:    "empty allowed list",
			value:   "a",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "empty value in non-empty list",
			value:   "",
			allowed: []string{"a", "b"},
			want:    false,
		},
		{
			name:    "empty value in list containing empty",
			value:   "",
			allowed: []string{"", "a"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidEnum(tt.value, tt.allowed)
			if got != tt.want {
				t.Fatalf("ValidEnum(%q, %v) = %v, want %v", tt.value, tt.allowed, got, tt.want)
			}
		})
	}
}
