package notifier

import (
	"context"
	"fmt"
	"net/smtp"
)

type EmailSender struct {
	host string
	port int
	user string
	pass string
	from string
}

func NewEmailSender(host string, port int, user, pass, from string) *EmailSender {
	return &EmailSender{host: host, port: port, user: user, pass: pass, from: from}
}

func (e *EmailSender) SendDown(_ context.Context, to, monitorName, url, cause string) error {
	subject := fmt.Sprintf("[PingCast] %s is DOWN", monitorName)
	body := fmt.Sprintf("Monitor: %s\nURL: %s\nStatus: DOWN\nCause: %s", monitorName, url, cause)
	return e.send(to, subject, body)
}

func (e *EmailSender) SendUp(_ context.Context, to, monitorName, url string) error {
	subject := fmt.Sprintf("[PingCast] %s is back UP", monitorName)
	body := fmt.Sprintf("Monitor: %s\nURL: %s\nStatus: UP", monitorName, url)
	return e.send(to, subject, body)
}

func (e *EmailSender) send(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		e.from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	auth := smtp.PlainAuth("", e.user, e.pass, e.host)

	return smtp.SendMail(addr, auth, e.from, []string{to}, []byte(msg))
}
