package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"switchyard/internal/actions"
)

type actionLogOutput struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}
type actionLogCapture struct {
	Stdout actionLogOutput `json:"stdout"`
	Stderr actionLogOutput `json:"stderr"`
	SHA    string          `json:"sha"`
	Kind   string          `json:"kind"`
}
type actionLogLine struct {
	ID     int    `json:"id"`
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

func boundedActionLines(c actionLogCapture) ([]actionLogLine, bool) {
	lines := []actionLogLine{}
	truncated := c.Stdout.Truncated || c.Stderr.Truncated
	for _, output := range []struct{ name, text string }{{"stdout", c.Stdout.Text}, {"stderr", c.Stderr.Text}} {
		if output.text == "" {
			continue
		}
		for _, line := range strings.Split(strings.TrimSuffix(output.text, "\n"), "\n") {
			if len(lines) >= 10000 {
				return lines, true
			}
			if len(line) > 8192 {
				line = line[:8192] + " [line truncated]"
				truncated = true
			}
			lines = append(lines, actionLogLine{len(lines) + 1, output.name, line})
		}
	}
	return lines, truncated
}

func (a *App) handleActionLogs(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	id := r.PathValue("id")
	label := r.URL.Query().Get("step")
	_, _, run, err := a.Trestle.FindRecord("action_runs", filterEq("id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_unavailable"})
		return
	}
	if run == nil || strOf(run["repo"]) != repo {
		writeJSON(w, 404, map[string]any{"error": "action_not_found"})
		return
	}
	var state struct {
		Status   string           `json:"status"`
		Manifest actions.Manifest `json:"manifest"`
	}
	if decodeAction(run["state"], &state) != nil {
		writeJSON(w, 502, map[string]any{"error": "invalid_run"})
		return
	}
	valid := false
	capturedAt := ""
	for _, job := range state.Manifest.Run.Jobs {
		for _, step := range job.Steps {
			if job.ID+"-"+step.ID == label {
				valid = true
			}
		}
	}
	for _, job := range state.Manifest.Jobs {
		for _, step := range job.Steps {
			if job.ID+"-"+step.ID == label {
				capturedAt = step.FinishedAt
			}
		}
	}
	if !valid {
		writeJSON(w, 404, map[string]any{"error": "step_not_found"})
		return
	}
	if a.Actions == nil {
		writeJSON(w, 503, map[string]any{"error": "logs_unavailable"})
		return
	}
	logRunID := id
	for _, job := range state.Manifest.Jobs {
		if job.ReusedFrom == "" {
			continue
		}
		for _, command := range state.Manifest.Run.Jobs {
			if command.ID != job.ID {
				continue
			}
			for _, step := range command.Steps {
				if command.ID+"-"+step.ID != label {
					continue
				}
				_, _, parent, lookupErr := a.Trestle.FindRecord("action_runs", filterEq("id", job.ReusedFrom))
				if lookupErr != nil || parent == nil || parent["repo"] != repo || parent["source_sha"] != run["source_sha"] || parent["definition_revision"] != run["definition_revision"] {
					writeJSON(w, 502, map[string]any{"error": "invalid_reused_capture"})
					return
				}
				logRunID = job.ReusedFrom
			}
		}
	}
	cursorText := r.URL.Query().Get("cursor")
	if cursorText == "" {
		cursorText = r.Header.Get("Last-Event-ID")
	}
	if cursorText == "" {
		cursorText = "0"
	}
	cursor, err := strconv.Atoi(cursorText)
	if err != nil || cursor < 0 || cursor > 10000 {
		writeJSON(w, 400, map[string]any{"error": "invalid_cursor"})
		return
	}
	streaming := strings.HasSuffix(r.URL.Path, "/stream")
	flusher, canFlush := w.(http.Flusher)
	if streaming && !canFlush {
		writeJSON(w, 500, map[string]any{"error": "stream_unavailable"})
		return
	}
	send := func(event string, value any) {
		body, _ := json.Marshal(value)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
		flusher.Flush()
	}
	if streaming {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		send("capture", map[string]any{"mode": "captured", "sha": run["source_sha"], "captured_at": capturedAt})
	}
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()
	var capture actionLogCapture
	for {
		err = a.Actions.Logs(r.Context(), logRunID, label, &capture)
		if err == nil {
			break
		}
		var remote *actions.HTTPError
		pending := errors.As(err, &remote) && remote.Status == 404 && !actions.Terminal(state.Status)
		if !streaming {
			code := 502
			if pending {
				code = 202
			}
			name := "logs_unavailable"
			if pending {
				name = "logs_pending"
			}
			writeJSON(w, code, map[string]any{"error": name, "pending": pending})
			return
		}
		if !pending {
			send("log_error", map[string]any{"error": "logs_unavailable"})
			return
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-r.Context().Done():
			timer.Stop()
			return
		case <-deadline.C:
			timer.Stop()
			send("log_error", map[string]any{"error": "capture_wait_expired"})
			return
		case <-timer.C:
		}
	}
	if capture.SHA != strOf(run["source_sha"]) || (capture.Kind != "captured" && capture.Kind != "diagnostic") || len(capture.Stdout.Text) > 1<<20 || len(capture.Stderr.Text) > 1<<20 {
		if streaming {
			send("log_error", map[string]any{"error": "invalid_capture"})
		} else {
			writeJSON(w, 502, map[string]any{"error": "invalid_capture"})
		}
		return
	}
	if !streaming && r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=action.log")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		fmt.Fprintf(w, "Source: %s\nCapture: %s\n\n[stdout]\n%s\n[stderr]\n%s\n", capture.SHA, capture.Kind, capture.Stdout.Text, capture.Stderr.Text)
		return
	}
	lines, truncated := boundedActionLines(capture)
	if cursor > len(lines) {
		if streaming {
			send("log_error", map[string]any{"error": "invalid_cursor"})
		} else {
			writeJSON(w, 400, map[string]any{"error": "invalid_cursor"})
		}
		return
	}
	if streaming {
		for _, line := range lines[cursor:] {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			body, _ := json.Marshal(line)
			fmt.Fprintf(w, "id: %d\nevent: line\ndata: %s\n\n", line.ID, body)
			flusher.Flush()
		}
		send("complete", map[string]any{"cursor": len(lines), "truncated": truncated, "kind": capture.Kind})
		return
	}
	end := cursor
	size := 0
	for end < len(lines) && end-cursor < 100 && size+len(lines[end].Text) <= 64<<10 {
		size += len(lines[end].Text)
		end++
	}
	writeJSON(w, 200, map[string]any{"lines": lines[cursor:end], "cursor": end, "complete": end == len(lines), "truncated": truncated, "kind": capture.Kind, "sha": capture.SHA})
}
