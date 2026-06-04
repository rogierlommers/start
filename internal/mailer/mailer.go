package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"path/filepath"
	"strings"
	"time"
)

var ErrDisabled = errors.New("mailer is not configured")

type Attachment struct {
	Filename string
	Data     []byte
}

type Message struct {
	To          string
	Subject     string
	Body        string
	Attachments []Attachment
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

	payload := buildMessage(s.from, msg)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if err := smtp.SendMail(addr, auth, s.from, []string{msg.To}, payload); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
	}

	return nil
}

// buildMessage constructs the SMTP payload as a multipart MIME message if attachments are present,
// otherwise as a simple text message.
func buildMessage(from string, msg Message) []byte {
	if len(msg.Attachments) == 0 {
		// Simple text message with no attachments
		headers := []string{
			"From: " + sanitizeHeaderValue(from),
			"To: " + sanitizeHeaderValue(msg.To),
			"Subject: " + sanitizeHeaderValue(msg.Subject),
			"MIME-Version: 1.0",
			"Content-Type: text/plain; charset=UTF-8",
		}
		return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Body)
	}

	// Multipart message with attachments
	boundary := fmt.Sprintf("boundary_%d", time.Now().UnixNano())
	buf := &bytes.Buffer{}

	headers := []string{
		"From: " + sanitizeHeaderValue(from),
		"To: " + sanitizeHeaderValue(msg.To),
		"Subject: " + sanitizeHeaderValue(msg.Subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s", boundary),
	}

	buf.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")

	// Write body part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	buf.WriteString(msg.Body + "\r\n")

	// Write attachments
	for _, att := range msg.Attachments {
		filename := sanitizeAttachmentFilename(att.Filename)
		contentType := detectAttachmentContentType(att)

		buf.WriteString("--" + boundary + "\r\n")
		fmt.Fprintf(buf, "Content-Type: %s; name=%q\r\n", contentType, filename)
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(buf, "Content-Disposition: attachment; filename=%q\r\n\r\n", filename)

		// Encode attachment data as base64 with line breaks every 76 chars
		encoded := base64.StdEncoding.EncodeToString(att.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := min(i+76, len(encoded))
			buf.WriteString(encoded[i:end] + "\r\n")
		}
	}

	buf.WriteString("--" + boundary + "--\r\n")

	return buf.Bytes()
}

func sanitizeHeaderValue(in string) string {
	out := strings.ReplaceAll(in, "\r", "")
	out = strings.ReplaceAll(out, "\n", "")
	return strings.TrimSpace(out)
}

func sanitizeAttachmentFilename(in string) string {
	trimmed := sanitizeHeaderValue(in)
	if trimmed == "" {
		return "attachment"
	}

	return strings.ReplaceAll(trimmed, "\"", "")
}

func detectAttachmentContentType(att Attachment) string {
	ext := strings.ToLower(filepath.Ext(att.Filename))
	if ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}

	if len(att.Data) > 0 {
		return http.DetectContentType(att.Data)
	}

	return "application/octet-stream"
}
