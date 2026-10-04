package actions

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

func (c *Client) GetBuildAsset(ctx context.Context, repo, run, job, name string) (*http.Response, error) {
	if !assetRepoPattern.MatchString(repo) || !assetRepoPattern.MatchString(run) || !actionID.MatchString(job) || name == "" {
		return nil, fmt.Errorf("invalid build asset identity")
	}
	hash := sha256.Sum256([]byte(name))
	path := "/build-output/" + repo + "/" + run + "/" + job + "/" + hex.EncodeToString(hash[:])
	r, err := http.NewRequestWithContext(ctx, "GET", c.base+path, nil)
	if err != nil {
		return nil, err
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	r.Header.Set("X-Switchyard-Time", stamp)
	r.Header.Set("X-Switchyard-Signature", Signature(c.secret, stamp, "GET", path, nil))
	client := *c.http
	client.Timeout = 2 * time.Minute
	response, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		response.Body.Close()
		return nil, &HTTPError{Status: response.StatusCode}
	}
	return response, nil
}

const ReleaseAssetLimit int64 = 64 << 20

var assetRepoPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,90}$`)
var assetIDPattern = regexp.MustCompile(`^ast_[a-f0-9]{24}$`)
var assetHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// assetRequest signs a bounded descriptor containing the expected size/hash.
// R2 validates the streamed bytes against that signed hash. Tokens never travel
// in URLs and redirect following stays disabled.
func (c *Client) assetRequest(ctx context.Context, method, repo, id, descriptor string, body io.Reader, size int64) (*http.Response, error) {
	if !assetRepoPattern.MatchString(repo) || !assetIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid release asset identity")
	}
	path := "/release-assets/" + repo + "/" + id
	r, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	r.Header.Set("X-Switchyard-Time", stamp)
	r.Header.Set("X-Switchyard-Asset", descriptor)
	r.Header.Set("X-Switchyard-Signature", Signature(c.secret, stamp, method, path, []byte(descriptor)))
	r.Header.Set("Content-Type", "application/octet-stream")
	r.ContentLength = size
	client := *c.http
	client.Timeout = 2 * time.Minute
	response, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, &HTTPError{Status: response.StatusCode}
	}
	return response, nil
}

func (c *Client) UploadReleaseAsset(ctx context.Context, repo, id, hash string, size int64, body io.Reader) error {
	if size < 1 || size > ReleaseAssetLimit || !assetHashPattern.MatchString(hash) {
		return fmt.Errorf("invalid release asset size/checksum")
	}
	data, _ := json.Marshal(map[string]any{"size": size, "sha256": hash, "content_type": "application/octet-stream"})
	response, err := c.assetRequest(ctx, "PUT", repo, id, base64.RawURLEncoding.EncodeToString(data), body, size)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var receipt struct {
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&receipt); err != nil {
		return err
	}
	if receipt.Size != size || receipt.SHA256 != hash {
		return fmt.Errorf("release asset receipt mismatch")
	}
	return nil
}

func (c *Client) GetReleaseAsset(ctx context.Context, repo, id string) (*http.Response, error) {
	return c.assetRequest(ctx, "GET", repo, id, "", nil, 0)
}

func (c *Client) DeleteReleaseAsset(ctx context.Context, repo, id string) error {
	response, err := c.assetRequest(ctx, "DELETE", repo, id, "", nil, 0)
	if err == nil {
		response.Body.Close()
	}
	return err
}
