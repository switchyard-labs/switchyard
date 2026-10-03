// Package actions speaks the signed control protocol to the Cloudflare Worker.
// The control plane never executes CI shell commands itself.
package actions

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Step struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Command   string `json:"command"`
	TimeoutMS int    `json:"timeout_ms"`
}
type Job struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Steps []Step `json:"steps"`
}
type Run struct {
	Provider           string            `json:"provider"`
	ProviderData       map[string]string `json:"providerData"`
	Event              map[string]string `json:"event"`
	Owner              string            `json:"owner"`
	Repo               string            `json:"repo"`
	SHA                string            `json:"sha"`
	Ref                string            `json:"ref"`
	Trigger            string            `json:"trigger"`
	Actor              string            `json:"actor,omitempty"`
	ID                 string            `json:"run_id"`
	DefinitionRevision string            `json:"definition_revision"`
	Jobs               []Job             `json:"jobs"`
	RerunOf            string            `json:"rerun_of,omitempty"`
	SelectedJobs       []string          `json:"selected_jobs,omitempty"`
}
type Client struct {
	base   string
	secret string
	http   *http.Client
}

func New(base, secret string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, fmt.Errorf("Actions Worker origin must be HTTPS without credentials or path")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("Actions control secret must have at least 32 bytes")
	}
	return &Client{base: strings.TrimRight(base, "/"), secret: secret, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func Signature(secret, timestamp, method, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%s\n%s\n", timestamp, method, path)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type HTTPError struct{ Status int }

func (e *HTTPError) Error() string { return fmt.Sprintf("Actions Worker returned HTTP %d", e.Status) }
func (c *Client) Request(ctx context.Context, method, path string, payload, out any) error {
	return c.requestBound(ctx, method, path, payload, out, 3<<20)
}
func (c *Client) requestBound(ctx context.Context, method, path string, payload, out any, bound int) error {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return fmt.Errorf("invalid Actions path")
	}
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	if len(body) > 128<<10 {
		return fmt.Errorf("Actions request exceeds 128 KiB")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("X-Switchyard-Time", stamp)
	req.Header.Set("X-Switchyard-Signature", Signature(c.secret, stamp, method, path, body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Switchyard-CI/1.0")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(bound)+1))
	if err != nil {
		return err
	}
	if len(data) > bound {
		return fmt.Errorf("Actions response exceeds bound")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPError{Status: response.StatusCode}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
func (c *Client) Dispatch(ctx context.Context, run Run) error {
	return c.Request(ctx, http.MethodPost, "/dispatch", run, nil)
}
func (c *Client) Status(ctx context.Context, id string, out any) error {
	return c.Request(ctx, http.MethodGet, "/runs/"+url.PathEscape(id), nil, out)
}
func (c *Client) Cancel(ctx context.Context, id string) error {
	return c.Request(ctx, http.MethodPost, "/runs/"+url.PathEscape(id)+"/cancel", nil, nil)
}
func (c *Client) Logs(ctx context.Context, id, step string, out any) error {
	return c.requestBound(ctx, http.MethodGet, "/runs/"+url.PathEscape(id)+"/logs?step="+url.QueryEscape(step), nil, out, 16<<20)
}
