package app

import (
	"net/http"
	"strings"
	"time"
)

type measuredWriter struct {
	http.ResponseWriter
	status int
}

func (w *measuredWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *measuredWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *measuredWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}
func (w *measuredWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (a *App) measureHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		record := &measuredWriter{ResponseWriter: w}
		defer func() {
			status := record.status
			if status == 0 {
				status = 200
			}
			name := "http_static"
			if strings.HasPrefix(r.URL.Path, "/api/") {
				name = "http_api"
			}
			a.Metrics.Record(name, time.Since(start), status)
		}()
		limit := int64(16 << 20)
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/repositories/") && strings.HasSuffix(r.URL.Path, "/release-assets") {
			limit = 64 << 20
		}
		r.Body = http.MaxBytesReader(record, r.Body, limit)
		next.ServeHTTP(record, r)
	})
}

// OperationalMetrics is served only by an explicitly configured loopback
// listener. It returns status counts, never record bodies or identities.
func (a *App) OperationalMetrics(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/metrics" || r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	states := map[string]map[string]int{}
	failures := []string{}
	for _, collection := range []string{"iq", "integration_effects", "workflow_runs", "workflow_claims", "action_runs"} {
		records, err := a.Trestle.ListRecords(collection, "")
		if err != nil {
			failures = append(failures, collection)
			continue
		}
		counts := map[string]int{}
		for _, record := range records {
			status := strOr(record["status"])
			if collection == "integration_effects" || collection == "workflow_claims" {
				effect, err := decodeQueueEffect(record)
				if err != nil {
					counts["invalid_state"]++
					continue
				}
				status = effect.Phase
				if until, err := time.Parse(time.RFC3339Nano, effect.LeaseUntil); err == nil && until.After(time.Now()) && effect.Owner != "" {
					counts["active_lease"]++
				}
			}
			if status == "" {
				status = "recorded"
			}
			counts[status]++
		}
		states[collection] = counts
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"agent_policy": a.AgentPolicy, "http": a.Metrics.Snapshot(), "trestle": a.Trestle.Metrics.Snapshot(), "artifacts": a.Artifacts.Metrics.Snapshot(), "states": states, "unavailable_collections": failures})
}
