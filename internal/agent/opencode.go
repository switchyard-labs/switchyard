package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type serializedDiagnostics struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *serializedDiagnostics) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

// RunOpenCodeAdapter is the bounded JSON bridge executed inside CLIRunner's
// sandbox. It never loads the operator's existing OpenCode login or plugins.
func RunOpenCodeAdapter(input io.Reader, output, diagnostics io.Writer) error {
	// Child stderr and parsed stdout events can arrive concurrently.
	diagnostics = &serializedDiagnostics{w: diagnostics}
	var task Task
	if err := json.NewDecoder(io.LimitReader(input, maxResultBytes+1)).Decode(&task); err != nil {
		return errors.New("invalid adapter task")
	}
	if strings.ContainsAny(task.File, "*?") || strings.HasPrefix(task.File, "~") || strings.HasPrefix(task.File, "$HOME") {
		return errors.New("task path cannot contain permission-pattern expansion")
	}
	if err := task.Validate(); err != nil {
		return err
	}
	allowed := map[string]bool{"opencode": true, "opencode-go": true, "openrouter": true, "openai": true, "anthropic": true, "google": true, "deepseek": true}
	if !allowed[task.Provider] || task.Model == "" || len(task.Model) > 200 || strings.ContainsAny(task.Model, "\x00\r\n\t ") {
		return errors.New("invalid adapter provider/model")
	}
	bin := os.Getenv("SWITCHYARD_OPENCODE_BIN")
	if bin == "" || !filepath.IsAbs(bin) {
		return errors.New("OpenCode runtime not configured")
	}
	keyPath := os.Getenv("SWITCHYARD_CREDENTIAL_FILE")
	key, err := os.ReadFile(keyPath)
	if err != nil || len(key) == 0 || len(key) > 64<<10 {
		return errors.New("personal credential unavailable")
	}
	// Configuration references the scoped environment variable, not plaintext.
	configDir, err := os.MkdirTemp("/tmp", "switchyard-opencode-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir)
	config := map[string]any{"$schema": "https://opencode.ai/config.json", "model": task.Provider + "/" + task.Model, "share": "disabled", "autoupdate": false, "plugin": []string{}, "mcp": map[string]any{}, "permission": map[string]any{"*": "deny", "read": map[string]string{"*": "deny", task.File: "allow", "/workspace/" + task.File: "allow", "workspace/" + task.File: "allow"}, "edit": map[string]string{"*": "deny", task.File: "allow", "/workspace/" + task.File: "allow", "workspace/" + task.File: "allow"}}, "provider": map[string]any{task.Provider: map[string]any{"options": map[string]string{"apiKey": "{env:SWITCHYARD_PROVIDER_API_KEY}"}}}}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	configPath := filepath.Join(configDir, "config.json")
	if err = os.WriteFile(configPath, raw, 0600); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(task.File), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(task.File, []byte(task.Current), 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	prompt := "Edit only " + task.File + " in this isolated workspace. Do not run shell commands, access credentials, or read files outside this task. Write the complete requested result into that file. Task: " + task.Prompt
	cmd := exec.CommandContext(ctx, bin, "--pure", "run", "--format", "json", "--model", task.Provider+"/"+task.Model, "--dir", "/workspace", "--", prompt)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=/home/agent", "LANG=C.UTF-8", "OPENCODE_CONFIG=" + configPath, "OPENCODE_CONFIG_DIR=" + configDir, "XDG_CONFIG_HOME=" + configDir, "XDG_DATA_HOME=" + configDir + "/data", "XDG_CACHE_HOME=" + configDir + "/cache", "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_PROJECT_CONFIG=1", "OPENCODE_DISABLE_CLAUDE_CODE=1", "SWITCHYARD_PROVIDER_API_KEY=" + string(key)}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = diagnostics
	if err = cmd.Start(); err != nil {
		return errors.New("OpenCode process failed to start")
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	events := 0
	providerError := false
	for scanner.Scan() {
		events++
		if events > 1000 {
			cancel()
			break
		}
		var event map[string]any
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			kind, _ := event["type"].(string)
			if kind == "error" {
				providerError = true
			}
			fmt.Fprintf(diagnostics, "OpenCode event: %s\n", kind)
			if kind == "tool_use" {
				part, _ := event["part"].(map[string]any)
				state, _ := part["state"].(map[string]any)
				tool, _ := part["tool"].(string)
				input, _ := state["input"].(map[string]any)
				requested, _ := input["filePath"].(string)
				if len(requested) < 500 && requested != "" {
					fmt.Fprintf(diagnostics, "Requested path: %s\n", requested)
				}
				status, _ := state["status"].(string)
				detail, _ := state["error"].(string)
				if len(detail) > 1000 {
					detail = detail[:1000]
				}
				fmt.Fprintf(diagnostics, "OpenCode tool %s: %s %s\n", tool, status, detail)
			}
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil {
		cancel()
	}
	err = cmd.Wait()
	if err != nil || scanErr != nil || providerError || events > 1000 {
		return errors.New("OpenCode provider execution failed; inspect bounded execution diagnostics")
	}
	info, err := os.Lstat(task.File)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxResultBytes {
		return errors.New("invalid OpenCode file result")
	}
	result, err := os.ReadFile(task.File)
	if err != nil {
		return err
	}
	if string(result) == task.Current {
		return errors.New("OpenCode did not produce a file change")
	}
	return json.NewEncoder(output).Encode(map[string]any{"files": map[string]string{task.File: string(result)}})
}
