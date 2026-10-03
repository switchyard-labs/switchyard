package app

import (
	"net/http"
	"sort"
	"strconv"
)

// List and detail share the existing repository/Work authorization boundary.
func (a *App) handleListAttempts(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if text := r.URL.Query().Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 1 || n > 200 {
			writeJSON(w, 400, map[string]any{"error": "invalid_limit"})
			return
		}
		limit = n
	}
	items, err := a.Trestle.ListRecords("attempts", "")
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "attempts_unavailable"})
		return
	}
	items = a.visibleRecords("attempts", items, a.currentUser(r))
	sort.Slice(items, func(i, j int) bool { return strOf(items[i]["id"]) < strOf(items[j]["id"]) })
	result := []map[string]any{}
	cursor := r.URL.Query().Get("cursor")
	next := ""
	for _, item := range items {
		if strOf(item["id"]) <= cursor {
			continue
		}
		matches := true
		for _, key := range []string{"work_id", "repo", "status"} {
			if value := r.URL.Query().Get(key); value != "" && strOf(item[key]) != value {
				matches = false
			}
		}
		if !matches {
			continue
		}
		if len(result) == limit {
			next = strOf(result[len(result)-1]["id"])
			break
		}
		result = append(result, item)
	}
	writeJSON(w, 200, map[string]any{"items": result, "next_cursor": next})
}
func (a *App) handleGetAttempt(w http.ResponseWriter, r *http.Request) {
	_, _, item, err := a.Trestle.FindRecord("attempts", filterEq("id", r.PathValue("id")))
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "attempts_unavailable"})
		return
	}
	if item == nil || !a.recordAccess("attempts", item, a.currentUser(r), ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "attempt_not_found"})
		return
	}
	writeJSON(w, 200, item)
}
