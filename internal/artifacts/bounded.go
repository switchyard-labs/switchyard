package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ReadBounded pins workspace reads to immutable objects, with cancellation and
// a byte ceiling before data reaches search or an editor buffer.
func (c *Client) ReadBounded(ctx context.Context, name, ref, path string, limit int64) ([]byte, error) {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return c.readBounded(ctx, "/repos/"+url.PathEscape(name)+"/raw/"+url.PathEscape(ref)+"/"+strings.Join(parts, "/"), limit)
}
func (c *Client) readBounded(ctx context.Context, path string, limit int64) ([]byte, error) {
	token, err := c.token()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := c.http.Do(req)
	if err != nil {
		return nil, transportError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, responseError(response)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("object exceeds byte limit")
	}
	return data, nil
}
func (c *Client) TreeContext(ctx context.Context, name, hash string) ([]TreeEntry, error) {
	data, err := c.readBounded(ctx, "/repos/"+url.PathEscape(name)+"/tree/"+url.PathEscape(hash), 4<<20)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	err = decodeBoundedResult(data, &entries)
	return entries, err
}
func (c *Client) LogContext(ctx context.Context, name, ref string) ([]Commit, error) {
	data, err := c.readBounded(ctx, "/repos/"+url.PathEscape(name)+"/log?limit=1&ref="+url.QueryEscape(ref), 1<<20)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	err = decodeBoundedResult(data, &commits)
	return commits, err
}

func decodeBoundedResult(data []byte, out any) error {
	var envelope struct {
		Result json.RawMessage   `json:"result"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("Artifacts result unavailable")
	}
	return json.Unmarshal(envelope.Result, out)
}
