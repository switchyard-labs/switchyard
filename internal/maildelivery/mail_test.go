package maildelivery

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func TestSMTPAuthenticatedTLSAndMultipart(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	listener, e := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	received := make(chan string, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		reply := func(s string) { rw.WriteString(s + "\r\n"); rw.Flush() }
		reply("220 fixture ESMTP")
		for {
			line, e := rw.ReadString('\n')
			if e != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				reply("250-fixture\r\n250 AUTH PLAIN")
			case strings.HasPrefix(line, "AUTH"):
				reply("235 authenticated")
			case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
				reply("250 accepted")
			case strings.HasPrefix(line, "DATA"):
				reply("354 continue")
				data, e := textproto.NewReader(rw.Reader).ReadDotBytes()
				if e != nil {
					return
				}
				received <- string(data)
				reply("250 delivered")
			case strings.HasPrefix(line, "QUIT"):
				reply("221 bye")
				return
			default:
				reply("500 unsupported")
			}
		}
	}()
	dialer := tls.Dialer{Config: &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}}
	smtp := SMTP{Address: listener.Addr().String(), Username: "fixture", Password: "fixture-password", From: "Switchyard <no-reply@example.test>", dial: func(ctx context.Context, n, a string) (net.Conn, error) { return dialer.DialContext(ctx, n, a) }}
	if e = smtp.Send(context.Background(), Message{To: "user@example.test", Subject: "Verify your Switchyard email", Text: "Text action", HTML: "<p>HTML action</p>"}); e != nil {
		t.Fatal(e)
	}
	body := <-received
	for _, part := range []string{"multipart/alternative", "text/plain", "text/html", "Text action", "<p>HTML action</p>"} {
		if !strings.Contains(body, part) {
			t.Fatal("missing MIME part", part)
		}
	}
	if strings.Contains(body, "fixture-password") {
		t.Fatal("transport credential in message")
	}
}
func TestSMTPUnavailableDoesNotExposeCredentials(t *testing.T) {
	smtp := SMTP{Address: "invalid", Username: "private-user", Password: "private-secret", From: "no-reply@example.test"}
	if e := smtp.Send(context.Background(), Message{To: "user@example.test"}); e != ErrUnavailable || strings.Contains(e.Error(), "private") {
		t.Fatal("unsafe provider error")
	}
}
