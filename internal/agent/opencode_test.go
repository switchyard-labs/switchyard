package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeAdapterScopedResultAndProviderError(t *testing.T) {
	for _, mode := range []string{"success", "provider_error", "unchanged"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			old, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(old)
			key := filepath.Join(dir, "key")
			os.WriteFile(key, []byte("scoped-test-key"), 0600)
			script := `#!/bin/sh
set -eu
test -z "${HOST_PRIVATE_VALUE-}"
test "$SWITCHYARD_PROVIDER_API_KEY" = scoped-test-key
case "$*" in *scoped-test-key*) exit 3;; esac
`
			if mode == "success" {
				script += `printf 'generated' > file.go
printf '{"type":"step_finish"}\n'
`
			}
			if mode == "provider_error" {
				script += `printf '{"type":"error"}\n'
`
			}
			bin := filepath.Join(dir, "fake-opencode")
			os.WriteFile(bin, []byte(script), 0700)
			t.Setenv("SWITCHYARD_OPENCODE_BIN", bin)
			t.Setenv("SWITCHYARD_CREDENTIAL_FILE", key)
			t.Setenv("HOST_PRIVATE_VALUE", "must not escape")
			task := fixtureTask()
			task.Provider = "opencode-go"
			task.Model = "test-model"
			raw, _ := json.Marshal(task)
			var out, diag bytes.Buffer
			err = RunOpenCodeAdapter(bytes.NewReader(raw), &out, &diag)
			if mode == "success" {
				if err != nil || !strings.Contains(out.String(), `"file.go":"generated"`) {
					t.Fatalf("result: %v %s", err, out.String())
				}
			} else if err == nil {
				t.Fatal("invalid/unchanged provider result accepted")
			}
			if strings.Contains(out.String()+diag.String(), "scoped-test-key") {
				t.Fatal("credential entered output")
			}
		})
	}
}

func TestOpenCodePermissionPatternsCannotExpandTaskPath(t *testing.T) {
	for _, path := range []string{"*.go", "file?.go", "~/file.go", "$HOME/file.go"} {
		task := fixtureTask()
		task.File = path
		raw, _ := json.Marshal(task)
		var out bytes.Buffer
		if err := RunOpenCodeAdapter(bytes.NewReader(raw), &out, &out); err == nil {
			t.Fatalf("expanded task path accepted: %s", path)
		}
	}
}

func TestProviderErrorsUseStructuredStatusWithoutMessages(t *testing.T) {
	for _, tc := range []struct {
		status float64
		code   string
	}{{401, "provider_auth_failed"}, {403, "provider_auth_failed"}, {429, "provider_rate_limited"}, {408, "provider_timeout"}, {504, "provider_timeout"}, {500, "provider_request_failed"}} {
		event := map[string]any{"error": map[string]any{"data": map[string]any{"statusCode": tc.status, "message": "secret-token"}}}
		if got := providerEventFailure(event); got != tc.code {
			t.Fatalf("status %v: %s", tc.status, got)
		}
	}
}
