// Package handlers implements job.Handler for each supported job type.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// EmailJobType is the job.Registry key for EmailHandler.
const EmailJobType = "send_email"

// EmailPayload is the send_email job's input.
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// EmailResult is the send_email job's output.
type EmailResult struct {
	SentTo string    `json:"sent_to"`
	SentAt time.Time `json:"sent_at"`
}

// Mailer sends plain-text email via SMTP. net/smtp.SendMail negotiates
// STARTTLS automatically when the server advertises it, so this works
// unmodified against real providers on the standard submission port 587
// (Gmail, Outlook, SendGrid, Mailgun, SES, ...) with Username/Password set
// to that provider's SMTP credentials -- not just the unauthenticated local
// Mailpit sink used in dev. It does NOT support implicit-TLS-only endpoints
// (e.g. port 465 with no STARTTLS), and has no per-call cancellation since
// net/smtp.SendMail is synchronous and ctx-unaware.
//
// Extracted out of EmailHandler so other handlers that can optionally email
// their own output (ReportHandler, CSVHandler) share the exact same
// delivery path instead of each wiring SMTP themselves.
type Mailer struct {
	// Addr is the SMTP server address, e.g. "localhost:1025".
	Addr string
	// From is the envelope and header From address.
	From string
	// Username and Password enable PLAIN auth when Username is non-empty;
	// leave both empty for an unauthenticated relay like Mailpit.
	Username string
	Password string
}

// Send delivers a plain-text email. It rejects header injection via CRLF in
// to/subject rather than silently stripping it, so a malformed job surfaces
// as an error instead of a corrupted or hijacked message.
func (m *Mailer) Send(to, subject, body string) error {
	if err := rejectCRLF("to", to); err != nil {
		return err
	}
	if err := rejectCRLF("subject", subject); err != nil {
		return err
	}

	msg := buildMessage(m.From, to, subject, body)
	if err := smtp.SendMail(m.Addr, m.auth(), m.From, []string{to}, msg); err != nil {
		return fmt.Errorf("handlers: send email: %w", err)
	}
	return nil
}

func (m *Mailer) auth() smtp.Auth {
	if m.Username == "" {
		return nil
	}
	host, _, _ := strings.Cut(m.Addr, ":")
	return smtp.PlainAuth("", m.Username, m.Password, host)
}

// EmailHandler sends plain-text email via Mailer.
type EmailHandler struct {
	Mailer *Mailer
}

var _ job.Handler = (*EmailHandler)(nil)

// Execute implements job.Handler.
func (h *EmailHandler) Execute(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var p EmailPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("handlers: decode email payload: %w", err)
	}
	if p.To == "" {
		return nil, errors.New("handlers: email payload missing \"to\"")
	}

	if err := h.Mailer.Send(p.To, p.Subject, p.Body); err != nil {
		return nil, err
	}

	return json.Marshal(EmailResult{SentTo: p.To, SentAt: time.Now().UTC()})
}

func buildMessage(from, to, subject, body string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.Bytes()
}

func rejectCRLF(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("handlers: email %s must not contain newlines", field)
	}
	return nil
}
