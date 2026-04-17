package smtp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	gosmtp "net/smtp"
	"strconv"
	"time"

	"github.com/kirillinakin/pingcast/internal/adapter/httperr"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.ChannelSenderFactory = (*Factory)(nil)
var _ port.AlertSender = (*sender)(nil)

type EmailChannelConfig struct {
	Address string `json:"address"`
}

type Factory struct {
	host string
	port int
	user string
	pass string
	from string
}

func NewFactory(host string, port int, user, pass, from string) *Factory {
	return &Factory{host: host, port: port, user: user, pass: pass, from: from}
}

func (f *Factory) CreateSender(config json.RawMessage) (port.AlertSender, error) {
	var cfg EmailChannelConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	return &sender{
		host: f.host, port: f.port, user: f.user, pass: f.pass, from: f.from,
		to: cfg.Address,
	}, nil
}

func (f *Factory) ValidateConfig(raw json.RawMessage) error {
	var cfg EmailChannelConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid email config: %w", err)
	}
	if cfg.Address == "" {
		return fmt.Errorf("email address required")
	}
	return nil
}

func (f *Factory) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "address", Label: "Email Address", Type: "text", Required: true, Placeholder: "alerts@company.com"},
	}}
}

type sender struct {
	host, user, pass, from, to string
	port                       int
}

func (s *sender) Send(ctx context.Context, event *domain.AlertEvent) error {
	var subject, body string
	switch event.Event {
	case domain.AlertDown:
		subject = fmt.Sprintf("[PingCast] %s is DOWN", event.MonitorName)
		body = fmt.Sprintf("Monitor: %s\nTarget: %s\nStatus: DOWN\nCause: %s", event.MonitorName, event.MonitorTarget, event.Cause)
	case domain.AlertUp:
		subject = fmt.Sprintf("[PingCast] %s is back UP", event.MonitorName)
		body = fmt.Sprintf("Monitor: %s\nTarget: %s\nStatus: UP", event.MonitorName, event.MonitorTarget)
	default:
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, s.to, subject, body)

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	auth := gosmtp.PlainAuth("", s.user, s.pass, s.host)

	// Dial with context-derived timeout instead of unbounded gosmtp.SendMail.
	deadline, hasDeadline := ctx.Deadline()
	dialTimeout := 10 * time.Second
	if hasDeadline {
		remaining := time.Until(deadline)
		if remaining < dialTimeout {
			dialTimeout = remaining
		}
	}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return httperr.ClassifyNetError(fmt.Errorf("smtp dial: %w", err))
	}

	client, err := gosmtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// Set a write deadline based on context.
	if hasDeadline {
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetDeadline(deadline)
		}
	}

	if err := client.Auth(auth); err != nil {
		return domain.NewDeliveryError("auth_error", 0, fmt.Errorf("smtp auth: %w", err))
	}
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(s.to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}
