package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/agent"
	"switchyard/internal/artifacts"
	"switchyard/internal/refs"
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

func TestAgentFailureAPIIsActionableAndDoesNotLeakDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{{"provider_auth_failed", 424}, {"provider_rate_limited", 429}, {"provider_timeout", 504}, {"runner_memory_limit", 503}, {"runner_pid_limit", 503}, {"runner_cancelled", 409}, {"git_push_failed", 502}} {
		w := httptest.NewRecorder()
		ex := &agent.Execution{ID: "exe_test", FailureCode: tc.code}
		err := errors.New("private diagnostic must not escape")
		if tc.code == "git_push_failed" {
			ex.FailureCode = ""
			err = &refs.StageError{Code: tc.code, Err: err}
		}
		writeAgentExecutionError(w, ex, err)
		if w.Code != tc.status {
			t.Fatalf("%s status %d", tc.code, w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["error"] != tc.code || body["execution"] != "exe_test" || body["message"] == "" {
			t.Fatalf("unexpected response %v", body)
		}
		if strings.Contains(w.Body.String(), "private diagnostic") {
			t.Fatal("diagnostic leaked")
		}
	}
}
