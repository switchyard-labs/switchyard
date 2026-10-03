package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"switchyard/internal/trestle"
	"testing"
)

func TestEventFailureDoesNotAdvanceObservation(t *testing.T) {
	store := &queueStore{records: map[string][]*queueRecord{}}
	fail := true
	envelopes := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/events/") {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			data, _ := json.Marshal(body["values"])
			envelopes = append(envelopes, string(data))
			if fail {
				w.WriteHeader(503)
				return
			}
			fmt.Fprint(w, `{"id":"event","version":1}`)
			return
		}
		store.ServeHTTP(w, r)
	}))
	defer server.Close()
	a := &App{Trestle: trestle.New(server.URL, "fixture", "fixture"), Hub: NewHub()}
	before, after := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if _, err := a.observeTransition("repo", "main", before, after, "queue"); err == nil {
		t.Fatal("failed event returned success")
	}
	if len(store.records["ref_obs"]) != 0 || len(store.records["event_receipts"]) != 1 {
		t.Fatal("observation advanced or intent missing")
	}
	fail = false
	if _, err := a.observeTransition("repo", "main", before, after, "reconciliation"); err != nil {
		t.Fatal(err)
	}
	if len(envelopes) != 2 || envelopes[0] != envelopes[1] || len(store.records["ref_obs"]) != 1 {
		t.Fatalf("retry mismatch: envelopes=%v observations=%d receipts=%d", envelopes, len(store.records["ref_obs"]), len(store.records["event_receipts"]))
	}
}
func TestEventRejectsMutableOrInvalidSHA(t *testing.T) {
	a := &App{}
	for _, sha := range []string{"main", "abc", "../../outside"} {
		if _, err := a.observeTransition("repo", "main", "", sha, "test"); err == nil {
			t.Fatal(sha)
		}
	}
}
