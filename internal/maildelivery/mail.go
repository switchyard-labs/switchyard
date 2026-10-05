// Package maildelivery keeps message transport independent of auth state.
package maildelivery

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type Message struct{ To, Subject, Text, HTML string }
type Mailer interface {
	Send(context.Context, Message) error
}

var ErrUnavailable = errors.New("mail delivery unavailable")

type SMTP struct {
	Address, Username, Password, From string
	dial                              func(context.Context, string, string) (net.Conn, error)
}

// SMTP uses authenticated implicit TLS (usually port 465). No fallback to
// plaintext, no credentials or provider errors in returned auth-facing errors.
func (s SMTP) Send(ctx context.Context, m Message) error {
	host, _, err := net.SplitHostPort(s.Address)
	if err != nil || s.Username == "" || s.Password == "" {
		return ErrUnavailable
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return ErrUnavailable
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil || strings.ContainsAny(m.Subject, "\r\n") {
		return ErrUnavailable
	}
	var body bytes.Buffer
	multi := multipart.NewWriter(&body)
	fmt.Fprintf(&body, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", from.String(), to.String(), m.Subject, multi.Boundary())
	for _, part := range []struct{ kind, value string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", part.kind+"; charset=utf-8")
		header.Set("Content-Transfer-Encoding", "8bit")
		writer, e := multi.CreatePart(header)
		if e != nil {
			return ErrUnavailable
		}
		if _, e = writer.Write([]byte(part.value)); e != nil {
			return ErrUnavailable
		}
	}
	if multi.Close() != nil {
		return ErrUnavailable
	}
	deadline := time.Now().Add(15 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}}
	dial := dialer.DialContext
	if s.dial != nil {
		dial = s.dial
	}
	conn, err := dial(ctx, "tcp", s.Address)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Close()
	stopCancel := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopCancel()
	conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return ErrUnavailable
	}
	defer client.Close()
	if err = client.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
		return ErrUnavailable
	}
	if err = client.Mail(from.Address); err != nil {
		return ErrUnavailable
	}
	if err = client.Rcpt(to.Address); err != nil {
		return ErrUnavailable
	}
	writer, err := client.Data()
	if err != nil {
		return ErrUnavailable
	}
	if _, err = writer.Write(body.Bytes()); err != nil {
		return ErrUnavailable
	}
	if err = writer.Close(); err != nil {
		return ErrUnavailable
	}
	if client.Quit() != nil {
		return ErrUnavailable
	}
	return nil
}
