// Package trestle is a minimal client for the single-node Trestle coordination
// truth used by the Switchyard control plane. It provisions collections and
// reads/writes records through the Trestle admin + application HTTP APIs.
package trestle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

type Client struct {
	BaseURL    string
	adminUser  string
	adminPass  string
	http       *http.Client
	mu         sync.Mutex
	csrf       string
	loginAt    time.Time
}

func New(baseURL, adminUser, adminPass string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		adminUser: adminUser,
		adminPass: adminPass,
		http:      &http.Client{Jar: jar, Timeout: 30 * time.Second},
	}
}

// ensureLogin logs in as admin and keeps a fresh CSRF token. Safe to call
// concurrently (mutex-guarded).
func (c *Client) ensureLogin() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.csrf != "" && time.Since(c.loginAt) < 25*time.Minute {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"username": c.adminUser, "password": c.adminPass})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/admin/v1/session", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("trestle admin login: %d", resp.StatusCode)
	}
	// fetch CSRF
	req2, _ := http.NewRequest(http.MethodGet, c.BaseURL+"/admin/v1/session", nil)
	resp2, err := c.http.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	var s struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&s); err != nil {
		return err
	}
	c.csrf = s.CSRFToken
	c.loginAt = time.Now()
	return nil
}

// do performs an admin/mutation request with CSRF. Method GET/POST.
func (c *Client) do(method, path string, payload any) (*http.Response, []byte, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, nil, err
	}
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set("X-Trestle-CSRF", c.csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b, nil
}

// CollectionField describes a Trestle collection field for provisioning.
type CollectionField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Unique   bool   `json:"unique,omitempty"`
}

// EnsureCollection creates a collection if it does not exist.
func (c *Client) EnsureCollection(name string, fields []CollectionField) error {
	resp, b, err := c.do(http.MethodGet, "/admin/v1/collections", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		var list struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &list) == nil {
			for _, it := range list.Items {
				if it.Name == name {
					return nil
				}
			}
		}
	}
	req := map[string]any{"name": name, "fields": fields}
	resp2, b2, err := c.do(http.MethodPost, "/admin/v1/collections", req)
	if err != nil {
		return err
	}
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		return fmt.Errorf("ensure collection %s: %d %s", name, resp2.StatusCode, strings.TrimSpace(string(b2)))
	}
	return nil
}

// CreateRecord inserts a record with an idempotency key (at-least-once ingest).
func (c *Client) CreateRecord(collection string, values map[string]any, idemKey string) (string, bool, error) {
	req := map[string]any{"values": values}
	// use the app API with an Idempotency-Key
	if err := c.ensureLogin(); err != nil {
		return "", false, err
	}
	b, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/v1/collections/"+collection+"/records", bytes.NewReader(b))
	if err != nil {
		return "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Trestle-CSRF", c.csrf)
	if idemKey != "" {
		httpReq.Header.Set("Idempotency-Key", idemKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var rec struct {
			ID      string         `json:"id"`
			Version int            `json:"version"`
			Values  map[string]any `json:"values"`
		}
		if err := json.Unmarshal(rb, &rec); err != nil {
			return "", false, err
		}
		replayed := resp.Header.Get("Idempotency-Replayed") == "true"
		return rec.ID, replayed, nil
	}
	return "", false, fmt.Errorf("create record %s: %d %s", collection, resp.StatusCode, strings.TrimSpace(string(rb)))
}

// ListRecords returns records, optionally filtered (Trestle filter syntax).
func (c *Client) ListRecords(collection, filter string) ([]map[string]any, error) {
	path := "/api/v1/collections/" + collection + "/records"
	if filter != "" {
		path += "?filter=" + urlQueryEscape(filter)
	}
	resp, b, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list %s: %d %s", collection, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Items []struct {
			Values map[string]any `json:"values"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(out.Items))
	for _, it := range out.Items {
		items = append(items, it.Values)
	}
	return items, nil
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "&", "%26")
}