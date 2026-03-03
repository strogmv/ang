package smtp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/port"
)

type Client struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func New(cfg *config.Config) *Client {
	return &Client{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPass,
		From:     cfg.SMTPFrom,
	}
}

func (c *Client) Send(ctx context.Context, msg port.EmailMessage) error {
	_ = ctx
	if c.Host == "" {
		return fmt.Errorf("smtp host not configured")
	}
	addr := fmt.Sprintf("%s:%s", c.Host, c.Port)
	from := c.From
	if from == "" {
		from = c.Username
	}
	if from == "" {
		return fmt.Errorf("smtp from not configured")
	}

	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"MIME-Version: 1.0",
	}

	body := msg.Text
	if msg.HTML != "" {
		headers = append(headers, "Content-Type: text/html; charset=UTF-8")
		body = msg.HTML
	} else {
		headers = append(headers, "Content-Type: text/plain; charset=UTF-8")
	}

	data := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	var auth smtp.Auth
	if c.Username != "" || c.Password != "" {
		auth = smtp.PlainAuth("", c.Username, c.Password, c.Host)
	}
	return smtp.SendMail(addr, auth, from, []string{msg.To}, []byte(data))
}
