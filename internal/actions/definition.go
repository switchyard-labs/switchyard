package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// Definition is an approved, immutable compilation of JavaScript configuration.
// JavaScript runs only during owner approval, never in the privileged Worker.
type Definition struct {
	Source   string   `json:"source"`
	Revision string   `json:"revision"`
	Refs     []string `json:"refs"`
	Jobs     []Job    `json:"jobs"`
}

var actionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
var actionRef = regexp.MustCompile(`^refs/(heads|tags)/[A-Za-z0-9_./-]{1,250}$`)

func CompileDefinition(source string) (Definition, error) {
	var result Definition
	if len(source) == 0 || len(source) > 65536 || strings.Count(source, "export default") != 1 {
		return result, fmt.Errorf("Actions configuration requires one default export, at most 64 KiB")
	}
	runtime := goja.New()
	runtime.SetMaxCallStackSize(256)
	runtime.SetTimeSource(func() time.Time { return time.Unix(0, 0) })
	runtime.SetRandSource(func() float64 { return .5 })
	timer := time.AfterFunc(time.Second, func() { runtime.Interrupt("Actions configuration exceeded one second") })
	defer timer.Stop()
	// This is a pure configuration module, without host callbacks, credentials,
	// filesystem, process or network. General JavaScript expressions remain useful
	// for shared commands, generated jobs and reusable project configuration.
	program := strings.Replace(source, "export default", "globalThis.definition =", 1)
	if _, err := runtime.RunString(program); err != nil {
		return result, fmt.Errorf("invalid Actions JavaScript: %w", err)
	}
	encoded, err := runtime.RunString("JSON.stringify(globalThis.definition)")
	if err != nil {
		return result, fmt.Errorf("invalid Actions export: %w", err)
	}
	body := encoded.String()
	if len(body) > 128<<10 {
		return result, fmt.Errorf("Actions definition exceeds 128 KiB")
	}
	if err = json.Unmarshal([]byte(body), &result); err != nil {
		return result, fmt.Errorf("Actions default export must contain refs and jobs")
	}
	if err = result.Validate(); err != nil {
		return result, err
	}
	sum := sha256.Sum256([]byte(source))
	result.Source = source
	result.Revision = hex.EncodeToString(sum[:])
	return result, nil
}
func (d Definition) Validate() error {
	if len(d.Refs) == 0 || len(d.Refs) > 16 || len(d.Jobs) == 0 || len(d.Jobs) > 8 {
		return fmt.Errorf("Actions needs 1–16 refs and 1–8 jobs")
	}
	for _, ref := range d.Refs {
		if !actionRef.MatchString(ref) || strings.Contains(ref, "..") {
			return fmt.Errorf("invalid Actions ref")
		}
	}
	labels := map[string]bool{}
	jobs := map[string]bool{}
	count := 0
	for _, job := range d.Jobs {
		if !actionID.MatchString(job.ID) || jobs[job.ID] || len(job.Steps) == 0 || len(job.Steps) > 16 {
			return fmt.Errorf("invalid or duplicate Actions job")
		}
		jobs[job.ID] = true
		steps := map[string]bool{}
		for _, step := range job.Steps {
			count++
			if !actionID.MatchString(step.ID) || steps[step.ID] || len(step.Command) == 0 || len(step.Command) > 8192 || step.TimeoutMS < 1000 || step.TimeoutMS > 120000 {
				return fmt.Errorf("invalid or duplicate Actions step")
			}
			label := job.ID + "-" + step.ID
			if labels[label] {
				return fmt.Errorf("ambiguous Actions step identity")
			}
			labels[label] = true
			steps[step.ID] = true
		}
	}
	if count > 32 {
		return fmt.Errorf("Actions exceeds 32 steps")
	}
	return nil
}
func (c *Client) PutDefinition(ctx context.Context, repo string, definition Definition) error {
	if err := definition.Validate(); err != nil {
		return err
	}
	return c.Request(ctx, "PUT", "/definitions/"+url.PathEscape(repo), definition, nil)
}
