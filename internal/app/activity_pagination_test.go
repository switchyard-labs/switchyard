package app

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestActivityPaginationSortsBeforeSlicingAndKeepsPublicBoundary(t *testing.T) {
	a, _, _ := actionFixture(t)
	_, _, err := a.Trestle.CreateRecord("users", map[string]any{"username": "alice"}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 13; i++ {
		_, _, err = a.Trestle.CreateRecord("work_details", map[string]any{"work_id": fmt.Sprint(i), "repo": "repo"}, "details-"+fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = a.Trestle.CreateRecord("work", map[string]any{"id": fmt.Sprint(i), "owner": "alice", "title": fmt.Sprint(i), "updated_at": fmt.Sprintf("2026-10-03T%02d:00:00Z", i), "repo": "repo"}, fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _, err = a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "repo", "artifact_name": "repo", "visibility": "public", "owner_type": "user", "owner_id": "alice"}, "repo")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.Trestle.CreateRecord("work", map[string]any{"id": "private", "owner": "alice", "title": "must stay private", "updated_at": "2026-10-03T23:00:00Z"}, "private")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		offset string
		want   int
		first  string
		more   bool
	}{{"0", 5, "12", true}, {"5", 5, "7", true}, {"10", 3, "2", false}, {"99", 0, "", false}} {
		r := httptest.NewRequest("GET", "/api/users/alice/activity?limit=5&offset="+tc.offset, nil)
		r.SetPathValue("username", "alice")
		w := httptest.NewRecorder()
		a.handleUserActivity(w, r)
		var data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
			More  bool             `json:"has_more"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(data.Items) != tc.want || data.Total != 13 || data.More != tc.more {
			t.Fatalf("page %s: %s", tc.offset, w.Body.String())
		}
		if tc.want > 0 && data.Items[0]["title"] != tc.first {
			t.Fatal("activity sliced before sorting")
		}
	}
}
