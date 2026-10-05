package app

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSemanticInputBounds(t *testing.T) {
	dir := t.TempDir()
	rules := make([]contractRule, 101)
	for i := range rules {
		rules[i] = contractRule{Kind: "field_equals", A: "a.json:value", B: "b.json:value"}
	}
	raw, _ := json.Marshal(map[string]any{"rules": rules})
	os.WriteFile(filepath.Join(dir, "switchyard.contract.json"), raw, 0600)
	if _, err := semanticFindingsInTree(dir); err == nil {
		t.Fatal("unbounded rule count")
	}
	os.WriteFile(filepath.Join(dir, "switchyard.contract.json"), []byte(strings.Repeat(" ", 128<<10+1)), 0600)
	if _, err := semanticFindingsInTree(dir); err == nil {
		t.Fatal("unbounded contract")
	}
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(strings.Repeat(" ", 1<<20+1)), 0600)
	if _, err := mergedJSONField(dir, "a.json:value"); err == nil {
		t.Fatal("unbounded input")
	}
	if _, err := mergedJSONField(dir, "a.json:"+strings.Repeat("a.", 33)); err == nil {
		t.Fatal("unbounded selector")
	}
}
func TestThrottleSaturationPreservesExistingBuckets(t *testing.T) {
	a := &App{loginAttempts: map[string]loginWindow{}}
	for i := 0; i < 10000; i++ {
		a.loginAttempts[fmt.Sprintf("login:%d", i)] = loginWindow{Until: time.Now().Add(time.Minute)}
	}
	if !a.allowAuthAttempt("1", "login") {
		t.Fatal("saturation denied existing caller")
	}
	if a.allowAuthAttempt("new", "login") {
		t.Fatal("saturation grew bookkeeping")
	}
	for i := 0; i < 10; i++ {
		a.allowAuthAttempt("1", "login")
	}
	if a.allowAuthAttempt("1", "login") {
		t.Fatal("existing bucket bypassed limit")
	}
}
func TestReleaseViewOmitsInternalStorageKey(t *testing.T) {
	raw, _ := json.Marshal([]releaseAsset{{ID: "asset", Name: "file", StorageKey: "private/r2/key", State: "ready"}})
	a := &App{}
	view := a.releaseView(map[string]any{"owner_slug": "alice", "slug": "repo"}, map[string]any{"target_sha": strings.Repeat("a", 40), "assets_json": string(raw)})
	body, _ := json.Marshal(view)
	if strings.Contains(string(body), "storage_key") || strings.Contains(string(body), "private/r2") {
		t.Fatal(string(body))
	}
	stored, err := releaseAssets(map[string]any{"assets_json": string(raw)})
	if err != nil || stored[0].StorageKey != "private/r2/key" {
		t.Fatal("storage functionality changed")
	}
}
func TestRefCursorRepeatedTransitionsAndDeletion(t *testing.T) {
	store := &queueStore{records: map[string][]*queueRecord{}, enforceUnique: true, idempotency: map[string]*queueRecord{}}
	server := httptest.NewServer(store)
	defer server.Close()
	a := queueCrashApp(server.URL, "", t.TempDir())
	aa, bb := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, sha := range []string{aa, bb, aa, bb, "", aa} {
		if err := a.reconcileRef("repo", "main", sha); err != nil {
			t.Fatal(err)
		}
		rows, err := a.Trestle.ListRecords("ref_current", filterEq("repo", "repo"))
		if err != nil || len(rows) != 1 || rows[0]["sha"] != sha {
			t.Fatal(rows, err)
		}
	}
	events, _ := a.Trestle.ListRecords("events", "")
	if len(events) != 4 {
		t.Fatal("immutable transition dedup changed", len(events))
	}
}
