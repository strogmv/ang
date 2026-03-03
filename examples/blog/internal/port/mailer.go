package port

import "context"

type EmailMessage struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, msg EmailMessage) error
}
