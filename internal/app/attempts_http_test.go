package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttemptsListingFiltersPrivateWorkAndPaginates(t *testing.T) {
	a, _, _ := actionFixture(t)
	for _, row := range []struct{ id, owner string }{{"a", "alice"}, {"b", "bob"}, {"c", "alice"}} {
		_, _, err := a.Trestle.CreateRecord("work", map[string]any{"id": "work-" + row.id, "owner": row.owner}, "work-"+row.id)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = a.Trestle.CreateRecord("attempts", map[string]any{"id": row.id, "work_id": "work-" + row.id, "status": "open"}, "attempt-"+row.id)
		if err != nil {
			t.Fatal(err)
		}
	}
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		w := httptest.NewRecorder()
		a.authorizeHandler(a.handleListAttempts)(w, r)
		return w
	}
	var first struct {
		Items []map[string]any `json:"items"`
		Next  string           `json:"next_cursor"`
	}
	w := get("/api/attempts?limit=1")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if json.Unmarshal(w.Body.Bytes(), &first) != nil || len(first.Items) != 1 || first.Items[0]["id"] != "a" || first.Next != "a" {
		t.Fatal(w.Body.String())
	}
	w = get("/api/attempts?limit=1&cursor=a")
	if json.Unmarshal(w.Body.Bytes(), &first) != nil || len(first.Items) != 1 || first.Items[0]["id"] != "c" || first.Next != "" {
		t.Fatal(w.Body.String())
	}
	if w = get("/api/attempts?limit=201"); w.Code != 400 {
		t.Fatal("invalid limit accepted")
	}
	r := httptest.NewRequest("GET", "/api/attempts/b", nil)
	r.SetPathValue("id", "b")
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w = httptest.NewRecorder()
	a.authorizeHandler(a.handleGetAttempt)(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal("private attempt detail allowed", w.Code)
	}
}
