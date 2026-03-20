package smtp

import (
	"context"
	"fmt"
	gosmtp "net/smtp"

	"github.com/kirillinakin/pingcast/internal/port"
)

// compile-time check
var _ port.AlertSender = (*recipientAlert)(nil)

// Sender knows how to send e-mail via SMTP.
type Sender struct {
	host string
	port int
	user string
	pass string
	from string
}

// New creates an SMTP Sender.
func New(host string, port int, user, pass, from string) *Sender {
	return &Sender{host: host, port: port, user: user, pass: pass, from: from}
}

// recipientAlert implements port.AlertSender for a specific recipient.
type recipientAlert struct {
	sender *Sender
	to     string
}

// ForRecipient returns an AlertSender bound to the given e-mail address.
func (s *Sender) ForRecipient(to string) port.AlertSender {
	return &recipientAlert{sender: s, to: to}
}

func (a *recipientAlert) NotifyDown(_ context.Context, name, target, cause string) error {
	subject := fmt.Sprintf("[PingCast] %s is DOWN", name)
	body := fmt.Sprintf("Monitor: %s\nTarget: %s\nStatus: DOWN\nCause: %s", name, target, cause)
	return a.sender.send(a.to, subject, body)
}

func (a *recipientAlert) NotifyUp(_ context.Context, name, target string) error {
	subject := fmt.Sprintf("[PingCast] %s is back UP", name)
	body := fmt.Sprintf("Monitor: %s\nTarget: %s\nStatus: UP", name, target)
	return a.sender.send(a.to, subject, body)
}

func (s *Sender) send(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := gosmtp.PlainAuth("", s.user, s.pass, s.host)

	return gosmtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
}
