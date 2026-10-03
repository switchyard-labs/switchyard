package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetricsRetainStreamingAndStatus(t *testing.T) {
	a := &App{}
	handler := a.measureHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.(http.Flusher).Flush()
		w.Write([]byte("event"))
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/logs?secret=not-recorded", nil))
	stats := a.Metrics.Snapshot()
	if w.Code != 429 || !w.Flushed || stats["http_api"].RateLimited != 1 || len(stats) != 1 {
		t.Fatalf("stream/status metrics incorrect: %+v", stats)
	}
}
