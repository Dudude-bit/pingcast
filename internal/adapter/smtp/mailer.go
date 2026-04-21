package smtp

import (
	"context"
	"fmt"
	"log/slog"
	stdsmtp "net/smtp"

	"github.com/kirillinakin/pingcast/internal/port"
)

// Mailer sends transactional emails over plain net/smtp. Wraps the
// existing SMTP_* envs the notifier already reads, so API + notifier
// share one set of creds. Zero-config when SMTP_HOST is unset — falls
// back to a logging stub so dev builds don't blow up on subscribe.
type Mailer struct {
	host, user, pass, from string
	port                   int
}

var _ port.Mailer = (*Mailer)(nil)

// NewMailer builds a sender. Empty host → noop with slog.Info output.
func NewMailer(host string, port int, user, pass, from string) *Mailer {
	return &Mailer{host: host, port: port, user: user, pass: pass, from: from}
}

func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	_ = ctx
	if m.host == "" {
		slog.Info("smtp noop (SMTP_HOST unset)",
			"to", to, "subject", subject)
		return nil
	}
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	auth := stdsmtp.PlainAuth("", m.user, m.pass, m.host)
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		m.from, to, subject, body,
	))
	if err := stdsmtp.SendMail(addr, auth, m.from, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}
