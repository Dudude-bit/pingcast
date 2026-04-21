package port

import "context"

// Mailer is the outbound-email port for transactional flows the API
// service owns (status-subscription confirmation + unsubscribe links +
// incident notifications). Deliberately narrow: no templates, no
// channels — just "send this body to this address". Real deliverability
// lives behind adapters (Resend, Postmark, net/smtp) that implement this.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}
