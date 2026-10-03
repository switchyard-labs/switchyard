package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignedWorkerProtocol(t *testing.T) {
	secret := "01234567890123456789012345678901"
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Switchyard-Signature") != Signature(secret, r.Header.Get("X-Switchyard-Time"), r.Method, r.URL.RequestURI(), nil) {
			t.Error("invalid request signature")
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "succeeded"})
	}))
	defer server.Close()
	client, err := New(server.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	client.http = server.Client()
	var result map[string]string
	if err = client.Status(context.Background(), "run-1", &result); err != nil || result["status"] != "succeeded" || calls != 1 {
		t.Fatalf("protocol %v %v", result, err)
	}
}
func TestRejectInsecureWorkerConfiguration(t *testing.T) {
	for _, origin := range []string{"http://worker.example", "https://user:password@worker.example", "https://worker.example/path", "https://worker.example?token=x"} {
		if _, err := New(origin, "01234567890123456789012345678901"); err == nil {
			t.Fatal(origin)
		}
	}
}
