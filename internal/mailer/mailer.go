package mailer

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
)

var ErrDisabled = errors.New("mailer is not configured")

type Message struct {
	To      string
	Subject string
	Body    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type DisabledSender struct{}

func (d DisabledSender) Send(_ context.Context, _ Message) error {
	return ErrDisabled
}

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewSMTPSender(host string, port int, username, password, from string) *SMTPSender {
	return &SMTPSender{
		host:     strings.TrimSpace(host),
		port:     port,
		username: strings.TrimSpace(username),
		password: password,
		from:     strings.TrimSpace(from),
	}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, err := mail.ParseAddress(msg.To); err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	if strings.TrimSpace(s.host) == "" || strings.TrimSpace(s.from) == "" || s.port <= 0 {
		return ErrDisabled
	}

	headers := []string{
		"From: " + sanitizeHeaderValue(s.from),
		"To: " + sanitizeHeaderValue(msg.To),
		"Subject: " + sanitizeHeaderValue(msg.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}

	payload := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Body

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if err := smtp.SendMail(addr, auth, s.from, []string{msg.To}, []byte(payload)); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
	}

	return nil
}

func sanitizeHeaderValue(in string) string {
	out := strings.ReplaceAll(in, "\r", "")
	out = strings.ReplaceAll(out, "\n", "")
	return strings.TrimSpace(out)
}
