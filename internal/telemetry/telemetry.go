// Package telemetry records bounded operation names, counts and durations.
// No request paths, repository names, response bodies or credentials are stored.
package telemetry

import (
	"net/http"
	"sync"
	"time"
)

type Sample struct {
	Count             uint64  `json:"count"`
	Errors            uint64  `json:"errors"`
	RateLimited       uint64  `json:"rate_limited"`
	TotalMilliseconds float64 `json:"total_ms"`
	MaxMilliseconds   float64 `json:"max_ms"`
}
type Registry struct {
	mu      sync.Mutex
	samples map[string]Sample
}

func (r *Registry) Record(name string, elapsed time.Duration, status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.samples == nil {
		r.samples = map[string]Sample{}
	}
	s := r.samples[name]
	s.Count++
	if status >= 400 || status == 0 {
		s.Errors++
	}
	if status == 429 {
		s.RateLimited++
	}
	ms := float64(elapsed) / float64(time.Millisecond)
	s.TotalMilliseconds += ms
	if ms > s.MaxMilliseconds {
		s.MaxMilliseconds = ms
	}
	r.samples[name] = s
}
func (r *Registry) Snapshot() map[string]Sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]Sample{}
	for k, v := range r.samples {
		out[k] = v
	}
	return out
}

type Transport struct {
	Base     http.RoundTripper
	Registry *Registry
	Name     string
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	start := time.Now()
	response, err := base.RoundTrip(req)
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	t.Registry.Record(t.Name, time.Since(start), status)
	return response, err
}
