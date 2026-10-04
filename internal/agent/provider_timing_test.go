package agent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderTimingSeparatesHeadersAndStreamingCompletion(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/go/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture-private" {
			t.Error("proxy changed scoped provider request")
		}
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, "fixture-private-stream")
	}))
	defer upstream.Close()
	base, finish, err := providerTimingProxyTransport(upstream.URL+"/zen/go/v1", upstream.Client().Transport.(*http.Transport).Clone())
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("POST", base+"/chat/completions", strings.NewReader("fixture-private-prompt"))
	req.Header.Set("Authorization", "Bearer fixture-private")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "fixture-private-stream" {
		t.Fatal("stream changed")
	}
	stats := finish()
	if stats["provider_http_requests"] != 1 || stats["provider_http_failures"] != 0 || stats["provider_http_complete_ns"] <= stats["provider_http_headers_ns"] || stats["provider_http_complete_ns"] < int64(20*time.Millisecond) {
		t.Fatalf("missing streaming timing: %v", stats)
	}
	if len(stats) > 4 {
		t.Fatal("unexpected diagnostic metadata")
	}
}
