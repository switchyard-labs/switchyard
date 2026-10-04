package actions

import (
	"context"
	"net/url"
)

type StepState struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	LogKey     string `json:"log_key"`
	Truncated  bool   `json:"truncated"`
}
type JobState struct {
	ID         string              `json:"id"`
	ReusedFrom string              `json:"reused_from,omitempty"`
	Name       string              `json:"name"`
	Status     string              `json:"status"`
	Steps      []StepState         `json:"steps"`
	Assets     []BuildAssetReceipt `json:"assets,omitempty"`
}
type BuildAssetReceipt struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	SourceSHA string `json:"source_sha"`
	Key       string `json:"key"`
	JobID     string `json:"job_id"`
}
type Manifest struct {
	Run        Run               `json:"run"`
	Source     *SourceInspection `json:"source,omitempty"`
	Status     string            `json:"status"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Jobs       []JobState        `json:"jobs"`
}
type SourceInspection struct {
	Repo          string        `json:"repo"`
	SHA           string        `json:"sha"`
	CommitPresent bool          `json:"commit_present"`
	Inspection    string        `json:"inspection"`
	Config        *SourceConfig `json:"config"`
}
type SourceConfig struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Snapshot struct {
	ID             string `json:"id"`
	ExternalStatus struct {
		Status string `json:"status"`
		Error  *struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"external_status"`
	Manifest *Manifest `json:"manifest"`
}
type RunPage struct {
	Runs   []Manifest `json:"runs"`
	Cursor string     `json:"cursor"`
}
type Provider interface {
	Dispatch(context.Context, Run) error
	Status(context.Context, string, any) error
	Cancel(context.Context, string) error
	Logs(context.Context, string, string, any) error
	PutDefinition(context.Context, string, Definition) error
	List(context.Context, string) (RunPage, error)
}

func (c *Client) List(ctx context.Context, cursor string) (RunPage, error) {
	var page RunPage
	err := c.Request(ctx, "GET", "/runs?cursor="+url.QueryEscape(cursor), nil, &page)
	return page, err
}
func NormalizeStatus(provider, manifest string) string {
	switch provider {
	case "terminated", "cancelled":
		return "cancelled"
	case "queued", "waiting", "waitingForPause", "paused":
		return "queued"
	case "running":
		return "running"
	case "errored":
		return "failure"
	case "complete":
		if manifest == "succeeded" {
			return "success"
		}
		return "failure"
	}
	switch manifest {
	case "succeeded":
		return "success"
	case "failed":
		return "failure"
	case "running":
		return "running"
	case "timed_out":
		return "timed_out"
	case "cancelled":
		return "cancelled"
	}
	return "queued"
}
func Terminal(status string) bool {
	return status == "success" || status == "failure" || status == "cancelled" || status == "timed_out"
}
