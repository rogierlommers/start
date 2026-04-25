package mailer

import (
	"context"
	"strings"
	"testing"
)

func TestDisabledSenderReturnsErrDisabled(t *testing.T) {
	if err := (DisabledSender{}).Send(context.Background(), Message{}); err != ErrDisabled {
		t.Fatalf("Send() error = %v, want %v", err, ErrDisabled)
	}
}

func TestNewSMTPSenderTrimsInputs(t *testing.T) {
	sender := NewSMTPSender(" smtp.example.com ", 2525, " user ", "pass", " from@example.com ")
	if sender.host != "smtp.example.com" || sender.username != "user" || sender.from != "from@example.com" {
		t.Fatalf("trimmed sender = %+v", sender)
	}
}

func TestSMTPSenderSendRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewSMTPSender("smtp.example.com", 25, "", "", "from@example.com").Send(ctx, Message{To: "person@example.com"})
	if err != context.Canceled {
		t.Fatalf("Send() error = %v, want %v", err, context.Canceled)
	}
}

func TestSMTPSenderSendRejectsInvalidRecipient(t *testing.T) {
	err := NewSMTPSender("smtp.example.com", 25, "", "", "from@example.com").Send(context.Background(), Message{To: "not-an-email"})
	if err == nil || !strings.Contains(err.Error(), "invalid recipient address") {
		t.Fatalf("Send() error = %v, want invalid recipient address", err)
	}
}

func TestSMTPSenderSendReturnsErrDisabledForMissingConfig(t *testing.T) {
	err := NewSMTPSender("", 25, "", "", "from@example.com").Send(context.Background(), Message{To: "person@example.com"})
	if err != ErrDisabled {
		t.Fatalf("Send() error = %v, want %v", err, ErrDisabled)
	}
}

func TestBuildMessageWithoutAttachments(t *testing.T) {
	payload := string(buildMessage("from@example.com", Message{
		To:      "to@example.com",
		Subject: "Hello\r\nInjected",
		Body:    "Body",
	}))

	if !strings.Contains(payload, "From: from@example.com") {
		t.Fatalf("payload missing From header: %q", payload)
	}
	if !strings.Contains(payload, "Subject: HelloInjected") {
		t.Fatalf("payload missing sanitized Subject header: %q", payload)
	}
	if !strings.Contains(payload, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("payload missing content type: %q", payload)
	}
	if !strings.HasSuffix(payload, "\r\n\r\nBody") {
		t.Fatalf("payload missing body suffix: %q", payload)
	}
}

func TestBuildMessageWithAttachments(t *testing.T) {
	payload := string(buildMessage("from@example.com", Message{
		To:      "to@example.com",
		Subject: "Hello",
		Body:    "Body",
		Attachments: []Attachment{{
			Filename: "report.txt",
			Data:     []byte("hello world"),
		}},
	}))

	if !strings.Contains(payload, "Content-Type: multipart/mixed; boundary=") {
		t.Fatalf("payload missing multipart header: %q", payload)
	}
	if !strings.Contains(payload, "Content-Disposition: attachment; filename=\"report.txt\"") {
		t.Fatalf("payload missing attachment header: %q", payload)
	}
	if !strings.Contains(payload, "aGVsbG8gd29ybGQ=") {
		t.Fatalf("payload missing base64 body: %q", payload)
	}
}

func TestSanitizeAttachmentFilenameAndHeaderValue(t *testing.T) {
	if got := sanitizeHeaderValue(" hello\r\nworld "); got != "helloworld" {
		t.Fatalf("sanitizeHeaderValue() = %q, want %q", got, "helloworld")
	}
	if got := sanitizeAttachmentFilename(" \"report.txt\"\n"); got != "report.txt" {
		t.Fatalf("sanitizeAttachmentFilename() = %q, want %q", got, "report.txt")
	}
	if got := sanitizeAttachmentFilename("   "); got != "attachment" {
		t.Fatalf("sanitizeAttachmentFilename(empty) = %q, want %q", got, "attachment")
	}
}

func TestDetectAttachmentContentType(t *testing.T) {
	if got := detectAttachmentContentType(Attachment{Filename: "image.png"}); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("detectAttachmentContentType(png) = %q", got)
	}
	if got := detectAttachmentContentType(Attachment{Filename: "", Data: []byte("plain text")}); got == "application/octet-stream" {
		t.Fatalf("detectAttachmentContentType(data) = %q, want detected type", got)
	}
	if got := detectAttachmentContentType(Attachment{}); got != "application/octet-stream" {
		t.Fatalf("detectAttachmentContentType(empty) = %q, want %q", got, "application/octet-stream")
	}
}

func TestSMTPSenderSendFailsOnDialError(t *testing.T) {
	// Port 1 is reserved and will always refuse the connection quickly.
	sender := NewSMTPSender("127.0.0.1", 1, "", "", "from@example.com")
	err := sender.Send(context.Background(), Message{To: "person@example.com"})
	if err == nil {
		t.Fatal("Send() error = nil, want dial error")
	}
	if !strings.Contains(err.Error(), "smtp send failed") {
		t.Fatalf("Send() error = %q, want smtp send failed", err)
	}
}

func TestSMTPSenderSendUsesPlainAuthWhenUsernameSet(t *testing.T) {
	// Verify the auth branch is exercised: connection still fails but the code path
	// that builds smtp.PlainAuth is covered.
	sender := NewSMTPSender("127.0.0.1", 1, "user", "pass", "from@example.com")
	err := sender.Send(context.Background(), Message{To: "person@example.com"})
	if err == nil {
		t.Fatal("Send() error = nil, want dial error")
	}
}
