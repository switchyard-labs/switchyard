// Package artifacts is a minimal client for the Cloudflare Artifacts REST API
// and the git operations the control plane needs. It obtains a fresh account
// token through an approved helper script (the cf OAuth token on the CP0
// Linode) rather than embedding any credential.
package artifacts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Client struct {
	AccountID string
	Namespace string
	TokenCmd  string // e.g. /opt/cp0/switchyard/token.sh
	http      *http.Client

	mu         sync.Mutex
	tokenCache string
	tokenAt    time.Time
}

func New(accountID, namespace, tokenCmd string) *Client {
	return &Client{
		AccountID: accountID,
		Namespace: namespace,
		TokenCmd:  tokenCmd,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// NewWithHTTP supplies an explicit HTTP transport for embedded deployments
// and deterministic protocol fixtures; the API origin stays Cloudflare.
func NewWithHTTP(accountID, namespace, tokenCmd string, client *http.Client) *Client {
	c := New(accountID, namespace, tokenCmd)
	if client != nil {
		c.http = client
	}
	return c
}

func (c *Client) baseURL() string {
	return "https://api.cloudflare.com/client/v4/accounts/" + c.AccountID + "/artifacts/namespaces/" + c.Namespace
}

// token returns a cached fresh account token (refreshed via the helper).
func (c *Client) token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokenCache != "" && time.Since(c.tokenAt) < 2*time.Minute {
		return c.tokenCache, nil
	}
	cmd := exec.Command(c.TokenCmd)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("artifacts token: %v", err)
	}
	t := strings.TrimSpace(string(out))
	if t == "" {
		return "", fmt.Errorf("artifacts token: empty")
	}
	c.tokenCache = t
	c.tokenAt = time.Now()
	return t, nil
}

func (c *Client) get(path string, out any) error {
	tok, err := c.token()
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodGet, c.baseURL()+path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artifacts %s: %d %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return err
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("artifacts %s: %s", path, env.Errors[0].Message)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

type Repo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Remote        string `json:"remote"`
	Source        string `json:"source"`
	ReadOnly      bool   `json:"read_only"`
}

func (c *Client) ListRepos() ([]Repo, error) {
	var repos []Repo
	if err := c.get("/repos?limit=200", &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (c *Client) GetRepo(name string) (*Repo, error) {
	var r Repo
	if err := c.get("/repos/"+url.PathEscape(name), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

type Commit struct {
	Hash      string `json:"hash"`
	TreeHash  string `json:"treeHash"`
	Message   string `json:"message"`
	Timestamp int64  `json:"committedAt"`
}

type TreeEntry struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
	Hash string `json:"hash"`
	Type string `json:"type"`
}

// Tree returns the immediate children of a tree hash (immutable Git object).
func (c *Client) Tree(name, treeHash string) ([]TreeEntry, error) {
	var entries []TreeEntry
	if err := c.get("/repos/"+url.PathEscape(name)+"/tree/"+treeHash, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) Log(name, ref string, limit int) ([]Commit, error) {
	var commits []Commit
	p := "/repos/" + url.PathEscape(name) + "/log?limit=" + fmt.Sprint(limit)
	if ref != "" {
		p += "&ref=" + url.QueryEscape(ref)
	}
	if err := c.get(p, &commits); err != nil {
		return nil, err
	}
	return commits, nil
}

// RawFile returns file bytes at ref/path (sniffed content type).
func (c *Client) RawFile(name, ref, path string) ([]byte, error) {
	tok, err := c.token()
	if err != nil {
		return nil, err
	}
	u := c.baseURL() + "/repos/" + url.PathEscape(name) + "/raw/" + url.PathEscape(ref) + "/" + strings.TrimPrefix(path, "/")
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raw %s@%s:%s: %d %s", name, ref, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// LsRemoteWithToken is the client method wrapper for the package LsRemote.
func (c *Client) LsRemoteWithToken(remote, token string) (map[string]string, error) {
	return LsRemote(remote, token)
}

// AccountToken returns the cached fresh account token (used for git
// subprocess calls that need the bearer header).
func (c *Client) AccountToken() (string, error) {
	return c.token()
}

// MintToken creates a repo-scoped git token (used for git protocol operations;
// git auth uses repo tokens, unlike the REST API which uses the account token).
func (c *Client) MintToken(repo, scope string, ttlSeconds int) (string, error) {
	tok, err := c.token()
	if err != nil {
		return "", err
	}
	body := strings.NewReader(fmt.Sprintf(`{"repo":%q,"scope":%q,"ttl":%d}`, repo, scope, ttlSeconds))
	req, _ := http.NewRequest(http.MethodPost, c.baseURL()+"/tokens", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("mint token %s: %d %s", repo, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var env struct {
		Result struct {
			Plaintext string `json:"plaintext"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return "", err
	}
	if env.Result.Plaintext == "" {
		return "", fmt.Errorf("mint token %s: empty", repo)
	}
	return env.Result.Plaintext, nil
}

// LsRemote returns the ref->sha map for a git remote (read path for CAS).
func LsRemote(remote, token string) (map[string]string, error) {
	cmd := exec.Command("git", "ls-remote", remote)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	// pass the token as an extra header
	cmd.Args = append(cmd.Args[:0:0], "git", "-c", "http.extraHeader=Authorization: Bearer "+token, "ls-remote", remote)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ls-remote: %v", err)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			refs[parts[1]] = parts[0]
		}
	}
	return refs, nil
}
