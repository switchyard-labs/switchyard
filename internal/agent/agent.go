// Package agent is the Switchyard Agent substrate: roles, provider profiles,
// a credential store (plaintext only in the execution-resolution path), and a
// pluggable runner adapter. The first adapter is deterministic (Strut worker);
// a coding-agent CLI adapter is wired for real providers (requires a provider
// credential + CLI to exercise).
package agent

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Role is the capability contract referenced by workflows/panels. It does NOT
// contain secrets.
type Role struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Profile      string   `json:"profile"`
}

// BuiltinRoles are available immediately without configuration.
var BuiltinRoles = []Role{
	{Name: "implementer", Capabilities: []string{"read", "edit-draft", "create-branch", "commit"}, Profile: "default"},
	{Name: "reviewer", Capabilities: []string{"read", "review", "findings"}, Profile: "default"},
	{Name: "conflict-resolver", Capabilities: []string{"read", "compare", "create-attempt"}, Profile: "default"},
}

// CredentialMetadata is the only representation of a credential exposed to the
// UI/API. The secret itself is never returned.
type CredentialMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Scope     string    `json:"scope"` // personal | organization
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// CredentialStore stores provider credentials encrypted at rest. Plaintext
// exists only in resolve-for-execution, which materializes a scoped 0600 file.
type CredentialStore struct {
	put        func(values map[string]any, idem string) error
	list       func() ([]map[string]any, error)
	findAndDel func(id string) error
	key        []byte
	update     func(string, string) error
}

func NewCredentialStore(
	put func(map[string]any, string) error,
	list func() ([]map[string]any, error),
	findAndDel func(string) error,
	key []byte,
) *CredentialStore {
	return &CredentialStore{put: put, list: list, findAndDel: findAndDel, key: key}
}

func (c *CredentialStore) SetUpdater(update func(string, string) error) { c.update = update }

func (c *CredentialStore) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func (c *CredentialStore) decrypt(ct string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("malformed credential ciphertext")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	out, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", errors.New("credential decrypt failed (key mismatch?)")
	}
	return string(out), nil
}

func (c *CredentialStore) Create(name, provider, scope, secret string) (string, error) {
	id := "cred_" + randToken(12)
	ct, err := c.encrypt(secret)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := c.put(map[string]any{
		"id": id, "name": name, "provider": provider, "scope": scope,
		"ciphertext": ct, "created_at": now, "last_used": "",
	}, "cred-"+id); err != nil {
		return "", err
	}
	return id, nil
}

// ResolveForExecution decrypts and writes a scoped 0600 secret file for one
// execution. The file must be deleted by the caller after the run.
func (c *CredentialStore) ResolveForExecution(id, runDir string) (string, error) {
	items, err := c.list()
	if err != nil {
		return "", err
	}
	var ct string
	for _, it := range items {
		if it["id"] == id {
			ct, _ = it["ciphertext"].(string)
			break
		}
	}
	if ct == "" {
		return "", errors.New("credential not found")
	}
	plain, err := c.decrypt(ct)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(runDir, "credential-*")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err = f.Write([]byte(plain)); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// Delete revokes a credential by id.
func (c *CredentialStore) Delete(id string) error {
	return c.findAndDel(id)
}

func (c *CredentialStore) Rotate(id, newSecret string) error {
	items, err := c.list()
	if err != nil {
		return err
	}
	for _, it := range items {
		if it["id"] == id {
			ct, err := c.encrypt(newSecret)
			if err != nil {
				return err
			}
			if c.update == nil {
				return errors.New("atomic credential updater unavailable")
			}
			return c.update(id, ct)
		}
	}
	return errors.New("credential not found")
}

func (c *CredentialStore) Metadata() ([]CredentialMetadata, error) {
	items, err := c.list()
	if err != nil {
		return nil, err
	}
	out := []CredentialMetadata{}
	for _, it := range items {
		m := CredentialMetadata{
			ID: textValue(it["id"]), Name: textValue(it["name"]),
			Provider: textValue(it["provider"]), Scope: textValue(it["scope"]),
		}
		if t, err := time.Parse(time.RFC3339, textValue(it["created_at"])); err == nil {
			m.CreatedAt = t
		}
		out = append(out, m)
	}
	return out, nil
}

// Execution is one invocation of an Agent role.
type Execution struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"`
	AttemptID string            `json:"attempt_id"`
	Adapter   string            `json:"adapter"`
	Status    string            `json:"status"`
	Started   time.Time         `json:"started"`
	Finished  time.Time         `json:"finished,omitempty"`
	Output    string            `json:"output,omitempty"`
	Result    map[string]string `json:"result,omitempty"` // path -> new content
}

// Runner is the pluggable adapter. It produces a bounded, attributable result
// for one execution.
type Runner interface {
	Run(exec *Execution, task map[string]any) error
}

// DeterministicRunner is the in-process deterministic adapter: it produces a
// fixed, attributable edit (current + append) with no subprocess. This is the
// certified substrate for workflows, reviews, conflict resolution and the
// dogfood loop. The runner abstraction stays so a real coding-agent CLI (or a
// future Strut/bounded-executor) can slot in without changing the substrate.
type DeterministicRunner struct {
	// Apply applies the produced file change (path+content) — the control
	// plane owns Git mutation through the ref substrate.
	Apply func(exec *Execution, path, content string) (string, error)
}

func (r *DeterministicRunner) Run(ex *Execution, task map[string]any) error {
	file, _ := task["file"].(string)
	appendLine, _ := task["append"].(string)
	current := ""
	if cur, ok := task["current"].(string); ok {
		current = cur
	}
	// deterministic edit: current + append (identical to the original
	// deterministic worker's input + append).
	newContent := current + appendLine
	ex.Output = fmt.Sprintf("deterministic edit of %s (+%d bytes)", file, len(newContent))
	if ex.Result == nil {
		ex.Result = map[string]string{}
	}
	ex.Result[file] = newContent
	if r.Apply != nil {
		sha, err := r.Apply(ex, file, newContent)
		if err != nil {
			return err
		}
		ex.Output += " -> " + sha
	}
	return nil
}

// CLIRunner spawns a configured coding-agent CLI with a provider profile. It is
// the wired-but-unexercised real-provider adapter until a provider credential
// and CLI are available in the environment.
type CLIRunner struct {
	Bin        string
	WorkDir    string
	Credential func() (string, error) // resolves a credential file path for the run
}

func (r *CLIRunner) Run(ex *Execution, task map[string]any) error {
	if r.Bin == "" {
		return errors.New("CLIRunner: no CLI binary configured (requires a provider credential + CLI)")
	}
	credPath := ""
	if r.Credential != nil {
		var err error
		if credPath, err = r.Credential(); err != nil {
			return err
		}
	}
	if credPath != "" {
		defer os.Remove(credPath)
	}
	cmd := exec.Command(r.Bin)
	cmd.Dir = r.WorkDir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "SWITCHYARD_CREDENTIAL_FILE=" + credPath, "SWITCHYARD_TASK=agent"}
	var output limitedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	out := output.String()
	if credPath != "" {
		if secret, e := os.ReadFile(credPath); e == nil && len(secret) > 0 {
			out = strings.ReplaceAll(out, string(secret), "[REDACTED]")
		}
	}
	ex.Output = out
	if err != nil {
		return fmt.Errorf("agent cli exited unsuccessfully: %w", err)
	}
	return nil
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
func textValue(v any) string { s, _ := v.(string); return s }

// limitedOutput consumes all writes while retaining at most one MiB.
type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}
