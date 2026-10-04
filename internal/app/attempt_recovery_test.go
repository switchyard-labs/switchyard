package app

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/artifacts"
	"testing"
)

func TestPublicationRecoveryRejectsPriorExecution(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("a", 40)
	want := "attempt attempt1 run (provider/model) execution exe_new"
	for _, tc := range []struct {
		name, message string
		accept        bool
	}{
		{"legacy attempt commit", "attempt attempt1 run (provider/model)", false},
		{"prior execution", "attempt attempt1 run (provider/model) execution exe_old", false},
		{"embedded message", "unrelated " + want, false},
		{"exact execution", want + "\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ac := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: credentialTransport(func(r *http.Request) (*http.Response, error) {
				if !strings.Contains(r.URL.Path, "/log") {
					t.Fatalf("unexpected request %s", r.URL.Path)
				}
				data, _ := json.Marshal(map[string]any{"result": []artifacts.Commit{{Hash: head, Message: tc.message}}})
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
			})})
			a := &App{Artifacts: ac}
			if got := a.commitHasMessage("demo", "branch", head, want); got != tc.accept {
				t.Fatalf("recovery accepted=%v, want %v", got, tc.accept)
			}
			if a.commitHasMessage("demo", "branch", strings.Repeat("b", 40), want) {
				t.Fatal("recovered unrelated head")
			}
		})
	}
}
