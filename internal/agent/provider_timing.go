package agent

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// providerTimingProxy measures HTTP exchanges without recording URLs, headers,
// credentials or bodies. The listener is ephemeral and loopback-only, inside
// the execution sandbox. It forwards only to the configured provider origin.
func providerTimingProxy(upstream string) (string, func() map[string]int64, error) {
	return providerTimingProxyTransport(upstream, http.DefaultTransport.(*http.Transport).Clone())
}

func providerTimingProxyTransport(upstream string, transport *http.Transport) (string, func() map[string]int64, error) {
	target, err := url.Parse(upstream)
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return "", nil, &url.Error{Op: "provider timing", URL: "", Err: errInvalidProviderOrigin{}}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	transport.ResponseHeaderTimeout = 100 * time.Second
	stats := &providerHTTPStats{values: map[string]int64{}}
	proxy := httputil.NewSingleHostReverseProxy(target)
	direct := proxy.Director
	proxy.Director = func(r *http.Request) { direct(r); r.Host = target.Host }
	proxy.Transport = &providerTimingTransport{base: transport, stats: stats}
	proxy.ErrorLog = log.New(io.Discard, "", 0)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Provider transport unavailable", 502)
	}
	server := &http.Server{Handler: proxy, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	finish := func() map[string]int64 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
		}
		transport.CloseIdleConnections()
		stats.mu.Lock()
		defer stats.mu.Unlock()
		out := map[string]int64{}
		for k, v := range stats.values {
			out[k] = v
		}
		return out
	}
	// ReverseProxy appends the provider's base path; SDK appends its endpoint.
	return "http://" + listener.Addr().String(), finish, nil
}

type errInvalidProviderOrigin struct{}

func (errInvalidProviderOrigin) Error() string { return "invalid HTTPS provider origin" }

type providerHTTPStats struct {
	mu     sync.Mutex
	values map[string]int64
}

func (s *providerHTTPStats) add(key string, n int64) { s.mu.Lock(); s.values[key] += n; s.mu.Unlock() }

type providerTimingTransport struct {
	base  http.RoundTripper
	stats *providerHTTPStats
}

func (t *providerTimingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	t.stats.add("provider_http_requests", 1)
	response, err := t.base.RoundTrip(r)
	t.stats.add("provider_http_headers_ns", int64(time.Since(start)))
	if err != nil {
		t.stats.add("provider_http_failures", 1)
		t.stats.add("provider_http_complete_ns", int64(time.Since(start)))
		return nil, err
	}
	if response.StatusCode >= 400 {
		t.stats.add("provider_http_failures", 1)
	}
	response.Body = &providerTimingBody{ReadCloser: response.Body, start: start, stats: t.stats}
	return response, nil
}

type providerTimingBody struct {
	io.ReadCloser
	start time.Time
	stats *providerHTTPStats
	once  sync.Once
}

func (b *providerTimingBody) done() {
	b.once.Do(func() { b.stats.add("provider_http_complete_ns", int64(time.Since(b.start))) })
}
func (b *providerTimingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.done()
	}
	return n, err
}
func (b *providerTimingBody) Close() error { err := b.ReadCloser.Close(); b.done(); return err }
