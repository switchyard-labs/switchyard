package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"switchyard/internal/agent"
	"switchyard/internal/maildelivery"
	"switchyard/internal/trestle"
	"sync"
	"testing"
)

type fileTestMailer struct {
	mu       sync.Mutex
	path     string
	messages []maildelivery.Message
}

func (m *fileTestMailer) Send(_ context.Context, msg maildelivery.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	data, err := json.Marshal(m.messages)
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0600)
}

// Explicit opt-in: the target must be a disposable loopback Trestle instance.
// This never sends real mail and uses the real collection persistence APIs.
func TestRecoveryBrowserWithTrestle(t *testing.T) {
	base := os.Getenv("SWITCHYARD_RECOVERY_TEST_TRESTLE")
	if base == "" {
		t.Skip("isolated Trestle/browser certification opt-in")
	}
	parsed, e := url.Parse(base)
	if e != nil || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("only disposable loopback Trestle permitted")
	}
	body, _ := json.Marshal(map[string]any{"username": "admin", "email": "admin@example.test", "password": "auth-preview-admin-password", "applicationRegistrationPolicy": "closed"})
	response, e := http.Post(base+"/admin/v1/setup", "application/json", bytes.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 201 && response.StatusCode != 200 && response.StatusCode != 409 {
		t.Fatal("setup failed", response.StatusCode)
	}
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	mailfile := filepath.Join(dir, "messages.json")
	a := New(trestle.New(base, "admin", "auth-preview-admin-password"), nil, filepath.Join(root, "public"), dir)
	a.Mailer = &fileTestMailer{path: mailfile}
	a.Secrets = agent.NewCredentialStore(func(map[string]any, string) error { return nil }, func() ([]map[string]any, error) { return nil, nil }, func(string) error { return nil }, []byte("0123456789abcdef0123456789abcdef"))
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.example")
	if e = a.Provision(); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewTLSServer(a.Handler())
	t.Setenv("SWITCHYARD_PUBLIC_URL", server.URL)
	defer server.Close()
	command := exec.Command("node", filepath.Join(root, "scripts/operations/auth-recovery-browser.mjs"))
	command.Env = append(os.Environ(), "RECOVERY_PREVIEW_URL="+server.URL, "RECOVERY_MAIL_FILE="+mailfile, "RECOVERY_EVIDENCE_DIR="+filepath.Join(root, "docs/evidence/auth-hardening"))
	output, e := command.CombinedOutput()
	if e != nil {
		t.Fatalf("browser certification failed: %s", strings.TrimSpace(string(output)))
	}
	t.Log(strings.TrimSpace(string(output)))
}
