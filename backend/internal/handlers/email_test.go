package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// startFakeSMTP starts a minimal SMTP server on localhost that accepts one
// session, speaks just enough of the protocol for net/smtp.SendMail to
// succeed, and reports the DATA payload it received on the returned
// channel. Good enough to prove EmailHandler actually talks SMTP, without
// pulling in a full mail server dependency just for tests.
func startFakeSMTP(t *testing.T) (addr string, received <-chan string) {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	msgCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serveFakeSMTPSession(conn, msgCh)
	}()

	return ln.Addr().String(), msgCh
}

func serveFakeSMTPSession(conn net.Conn, msgCh chan<- string) {
	reader := bufio.NewReader(conn)
	respond := func(line string) { _, _ = fmt.Fprintf(conn, "%s\r\n", line) }

	respond("220 fake.smtp ready")
	var inData bool
	var data strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				respond("250 OK")
				msgCh <- data.String()
				continue
			}
			data.WriteString(line)
			data.WriteString("\n")
			continue
		}

		switch upper := strings.ToUpper(line); {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			respond("250 fake.smtp")
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
			respond("250 OK")
		case upper == "DATA":
			respond("354 End data with <CR><LF>.<CR><LF>")
			inData = true
		case upper == "QUIT":
			respond("221 Bye")
			return
		default:
			respond("500 unrecognized command")
		}
	}
}

func TestEmailHandler_Execute_Success(t *testing.T) {
	addr, received := startFakeSMTP(t)
	h := &EmailHandler{Mailer: &Mailer{Addr: addr, From: "noreply@example.com"}}

	result, err := h.Execute(context.Background(), json.RawMessage(
		`{"to":"user@example.com","subject":"Hi","body":"Hello there"}`,
	))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var res EmailResult
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.SentTo != "user@example.com" {
		t.Errorf("SentTo = %q, want %q", res.SentTo, "user@example.com")
	}

	select {
	case msg := <-received:
		if !strings.Contains(msg, "Subject: Hi") {
			t.Errorf("captured message missing subject: %q", msg)
		}
		if !strings.Contains(msg, "Hello there") {
			t.Errorf("captured message missing body: %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fake SMTP server did not receive a message")
	}
}

func TestEmailHandler_Execute_MissingTo(t *testing.T) {
	h := &EmailHandler{Mailer: &Mailer{Addr: "localhost:1", From: "noreply@example.com"}}
	if _, err := h.Execute(context.Background(), json.RawMessage(`{"subject":"x"}`)); err == nil {
		t.Fatal("Execute() error = nil, want an error for missing \"to\"")
	}
}

func TestEmailHandler_Execute_RejectsCRLFInjection(t *testing.T) {
	h := &EmailHandler{Mailer: &Mailer{Addr: "localhost:1", From: "noreply@example.com"}}
	tests := []struct {
		name    string
		payload string
	}{
		{"crlf in to", `{"to":"a@example.com\r\nBcc: evil@example.com","subject":"x"}`},
		{"crlf in subject", `{"to":"a@example.com","subject":"x\r\nBcc: evil@example.com"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := h.Execute(context.Background(), json.RawMessage(tt.payload)); err == nil {
				t.Fatal("Execute() error = nil, want a rejection of embedded CRLF")
			}
		})
	}
}

func TestEmailHandler_Execute_InvalidPayload(t *testing.T) {
	h := &EmailHandler{Mailer: &Mailer{Addr: "localhost:1", From: "noreply@example.com"}}
	if _, err := h.Execute(context.Background(), json.RawMessage(`not json`)); err == nil {
		t.Fatal("Execute() error = nil, want a decode error")
	}
}

func TestEmailHandler_Execute_ContextCancelled(t *testing.T) {
	h := &EmailHandler{Mailer: &Mailer{Addr: "localhost:1", From: "noreply@example.com"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.Execute(ctx, json.RawMessage(`{"to":"a@example.com"}`))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Execute() error = %v, want context.Canceled", err)
	}
}
