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
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	BaseURL   string
	adminUser string
	adminPass string
	http      *http.Client
	mu        sync.Mutex
	csrf      string
	loginAt   time.Time
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
	if resp2.StatusCode != 200 {
		return fmt.Errorf("Trestle CSRF session: %d", resp2.StatusCode)
	}
	var s struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&s); err != nil {
		return err
	}
	if s.CSRFToken == "" {
		return fmt.Errorf("Trestle session missing CSRF token")
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
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf := c.csrfToken(); csrf != "" {
		req.Header.Set("X-Trestle-CSRF", csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := readResponse(resp.Body)
	return resp, b, err
}

// CollectionField describes a Trestle collection field for provisioning.
type CollectionField struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Required bool            `json:"required,omitempty"`
	Unique   bool            `json:"unique,omitempty"`
	ID       string          `json:"id,omitempty"`
	Default  json.RawMessage `json:"default,omitempty"`
}

// CreateRecord inserts a record with an idempotency key (at-least-once ingest).
func (c *Client) CreateRecord(collection string, values map[string]any, idemKey string) (string, bool, error) {
	req := map[string]any{"values": values}
	// use the app API with an Idempotency-Key
	if err := c.ensureLogin(); err != nil {
		return "", false, err
	}
	b, err := json.Marshal(req)
	if err != nil {
		return "", false, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/v1/collections/"+collection+"/records", bytes.NewReader(b))
	if err != nil {
		return "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Trestle-CSRF", c.csrfToken())
	if idemKey != "" {
		httpReq.Header.Set("Idempotency-Key", idemKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	rb, readErr := readResponse(resp.Body)
	if readErr != nil {
		return "", false, readErr
	}
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
	return "", false, responseError("create", collection, resp.StatusCode, rb)
}

// ListRecords consumes cursor pages and returns values for existing callers.
// It fails on cursor loops or excessive results rather than silently truncating.
func (c *Client) ListRecords(collection, filter string) ([]map[string]any, error) {
	values := []map[string]any{}
	cursor := ""
	seen := map[string]bool{}
	for {
		page, err := c.ListRecordsPage(collection, filter, cursor, 100)
		if err != nil {
			return nil, err
		}
		for _, record := range page.Items {
			values = append(values, record.Values)
		}
		if len(values) > 50000 {
			return nil, fmt.Errorf("coordination result exceeds 50000 records; use paged query")
		}
		if page.NextCursor == "" {
			return values, nil
		}
		if seen[page.NextCursor] {
			return nil, fmt.Errorf("Trestle cursor loop")
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
}

// FindRecord returns the first record matching the filter, including its id
// and version (needed for conditional PATCH).
func (c *Client) FindRecord(collection, filter string) (id, version string, values map[string]any, err error) {
	path := "/api/v1/collections/" + collection + "/records"
	if filter != "" {
		path += "?filter=" + urlQueryEscape(filter)
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	path += separator + "limit=2"
	resp, b, e := c.do(http.MethodGet, path, nil)
	if e != nil {
		return "", "", nil, e
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", nil, responseError("find", collection, resp.StatusCode, b)
	}
	var out struct {
		Items []struct {
			ID      string         `json:"id"`
			Version int            `json:"version"`
			Values  map[string]any `json:"values"`
		} `json:"items"`
	}
	if e := json.Unmarshal(b, &out); e != nil {
		return "", "", nil, e
	}
	if len(out.Items) == 0 {
		return "", "", nil, nil
	}
	if len(out.Items) > 1 {
		return "", "", nil, &APIError{Status: 409, Operation: "find", Collection: collection, Code: "ambiguous_record", Message: "expected one matching record"}
	}
	it := out.Items[0]
	return it.ID, fmt.Sprint(it.Version), it.Values, nil
}

// PatchRecord updates a record with optimistic concurrency (If-Match version).
func (c *Client) PatchRecord(collection, id, version string, values map[string]any) error {
	if err := c.ensureLogin(); err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"values": values})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, c.BaseURL+"/api/v1/collections/"+collection+"/records/"+id, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trestle-CSRF", c.csrfToken())
	req.Header.Set("If-Match", version)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, readErr := readResponse(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode != http.StatusOK {
		return responseError("patch", collection, resp.StatusCode, rb)
	}
	return nil
}

// DeleteRecord removes a record with optimistic concurrency (If-Match version).
func (c *Client) DeleteRecord(collection, id, version string) error {
	if err := c.ensureLogin(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+"/api/v1/collections/"+collection+"/records/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Trestle-CSRF", c.csrfToken())
	req.Header.Set("If-Match", version)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, err := readResponse(resp.Body)
		if err != nil {
			return err
		}
		return responseError("delete", collection, resp.StatusCode, body)
	}
	return nil
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

func (c *Client) csrfToken() string { c.mu.Lock(); defer c.mu.Unlock(); return c.csrf }
