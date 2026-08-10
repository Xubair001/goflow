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

// EmailHandler sends plain-text email via SMTP. It has no TLS/STARTTLS
// support and no per-call cancellation (net/smtp.SendMail is synchronous
// and ctx-unaware) — fine for relaying to a local dev SMTP sink like
// Mailpit, but swap in a proper ctx-aware SMTP client before pointing this
// at a real mail provider.
type EmailHandler struct {
	// Addr is the SMTP server address, e.g. "localhost:1025".
	Addr string
	// From is the envelope and header From address.
	From string
	// Username and Password enable PLAIN auth when Username is non-empty;
	// leave both empty for an unauthenticated relay like Mailpit.
	Username string
	Password string
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
	// Reject header injection via CRLF in attacker-influenced fields rather
	// than silently stripping it, so a malformed job surfaces as an error
	// instead of a corrupted or hijacked message.
	if err := rejectCRLF("to", p.To); err != nil {
		return nil, err
	}
	if err := rejectCRLF("subject", p.Subject); err != nil {
		return nil, err
	}

	auth := h.auth()
	msg := buildMessage(h.From, p.To, p.Subject, p.Body)
	if err := smtp.SendMail(h.Addr, auth, h.From, []string{p.To}, msg); err != nil {
		return nil, fmt.Errorf("handlers: send email: %w", err)
	}

	return json.Marshal(EmailResult{SentTo: p.To, SentAt: time.Now().UTC()})
}

func (h *EmailHandler) auth() smtp.Auth {
	if h.Username == "" {
		return nil
	}
	host, _, _ := strings.Cut(h.Addr, ":")
	return smtp.PlainAuth("", h.Username, h.Password, host)
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
