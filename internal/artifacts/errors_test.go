package artifacts

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type errorTransport func(*http.Request) (*http.Response, error)

func (f errorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpstreamErrorsAcrossReadAndTokenPaths(t *testing.T) {
	for _, tc := range []struct {
		status     int
		code       string
		downstream int
	}{{404, "repository_not_found", 404}, {429, "upstream_rate_limited", 429}, {401, "artifacts_auth_failed", 502}, {403, "artifacts_auth_failed", 502}, {503, "artifacts_unavailable", 503}, {504, "upstream_timeout", 504}} {
		t.Run(tc.code+http.StatusText(tc.status), func(t *testing.T) {
			c := NewWithHTTP("account", "namespace", "unused", &http.Client{Transport: errorTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{"17"}}, Body: io.NopCloser(strings.NewReader("sensitive provider body"))}, nil
			})})
			c.tokenCache = "fixture"
			c.tokenAt = time.Now()
			paths := []func() error{func() error { _, e := c.GetRepo("repo"); return e }, func() error { _, e := c.RawFile("repo", "main", "README.md"); return e }, func() error { _, e := c.MintToken("repo", "read", 60); return e }, func() error { _, e := c.ReadBounded(context.Background(), "repo", "main", "README.md", 1024); return e }}
			for _, path := range paths {
				err := path()
				var upstream *Error
				if !errors.As(err, &upstream) || upstream.Code != tc.code || upstream.Status != tc.downstream || upstream.RetryAfter != "17" {
					t.Fatalf("wrong classification: %v", err)
				}
				if strings.Contains(err.Error(), "sensitive") {
					t.Fatal("upstream body disclosed")
				}
			}
		})
	}
}
