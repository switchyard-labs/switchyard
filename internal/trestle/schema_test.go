package trestle

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type schemaFixture struct {
	mu           sync.Mutex
	schemas      map[string]Collection
	writes       int
	acknowledged bool
	failure      int
	loop         bool
}

func (f *schemaFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/admin/v1/session" {
		fmt.Fprint(w, `{"csrfToken":"fixture"}`)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/v1/collections") {
		name := strings.TrimPrefix(r.URL.Path, "/admin/v1/collections/")
		switch r.Method {
		case "GET":
			collection, ok := f.schemas[name]
			if !ok {
				w.WriteHeader(404)
				fmt.Fprint(w, `{"error":{"code":"not_found"}}`)
				return
			}
			json.NewEncoder(w).Encode(collection)
		case "POST", "PATCH":
			f.writes++
			f.acknowledged = f.acknowledged || r.Header.Get("X-Trestle-Acknowledge-Schema") == "true"
			var payload struct {
				Name   string            `json:"name"`
				Fields []CollectionField `json:"fields"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			collection := Collection{Name: payload.Name, Fields: payload.Fields}
			collection.Kind = "base"
			f.schemas[collection.Name] = collection
			if r.Method == "POST" {
				w.WriteHeader(201)
			}
			json.NewEncoder(w).Encode(collection)
		}
		return
	}
	if f.failure != 0 {
		w.WriteHeader(f.failure)
		fmt.Fprint(w, `{"error":{"code":"fixture_error","message":"coordination rejected","details":{"sensitive":"do not echo"}}}`)
		return
	}
	if r.Method == "GET" {
		offset, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		end := min(offset+limit, 237)
		items := []Record{}
		for i := offset; i < end; i++ {
			items = append(items, Record{ID: strconv.Itoa(i), Version: 1, Values: map[string]any{"id": strconv.Itoa(i)}})
		}
		next := ""
		if end < 237 {
			next = strconv.Itoa(end)
		}
		if f.loop {
			next = "0"
		}
		json.NewEncoder(w).Encode(RecordPage{Items: items, NextCursor: next})
		return
	}
	fmt.Fprint(w, `{}`)
}
func schemaClient(t *testing.T, fixture *schemaFixture) *Client {
	t.Helper()
	server := httptest.NewServer(fixture)
	t.Cleanup(server.Close)
	return New(server.URL, "fixture", "fixture")
}

func TestExistingNameDoesNotProveCompatibleSchema(t *testing.T) {
	wanted := []CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "state", Type: "json"}}
	for _, test := range []struct {
		name   string
		fields []CollectionField
		valid  bool
	}{
		{"compatible", wanted, true},
		{"missing", wanted[:1], false},
		{"wrong_type", []CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "state", Type: "text"}}, false},
		{"weak_unique", []CollectionField{{Name: "id", Type: "text"}, {Name: "state", Type: "json"}}, false},
		{"unexpected_required", append(append([]CollectionField(nil), wanted...), CollectionField{Name: "extra", Type: "text", Required: true}), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &schemaFixture{schemas: map[string]Collection{"items": {Name: "items", Kind: "base", Fields: test.fields}}}
			err := schemaClient(t, f).EnsureCollection("items", wanted)
			if (err == nil) != test.valid {
				t.Fatalf("schema compatibility: %v", err)
			}
			if !test.valid {
				var mismatch *SchemaMismatch
				if !errors.As(err, &mismatch) {
					t.Fatal("untyped schema mismatch")
				}
			}
			if f.writes != 0 {
				t.Fatal("startup edited existing schema")
			}
		})
	}
}

func TestAdditiveMigrationPreservesFieldsAndRequiresStablePlan(t *testing.T) {
	original := Collection{Name: "items", Kind: "base", Fields: []CollectionField{{ID: "field-original", Name: "id", Type: "text", Unique: true, Default: json.RawMessage(`"default"`)}}}
	target := []CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "note", Type: "text"}}
	plan, err := PlanAdditiveMigration(original, target)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fields[0].ID != "field-original" || string(plan.Fields[0].Default) != `"default"` {
		t.Fatal("lost existing field identity/default")
	}
	f := &schemaFixture{schemas: map[string]Collection{"items": original}}
	client := schemaClient(t, f)
	if err := client.ApplyAdditiveMigration(original, plan); err != nil {
		t.Fatal(err)
	}
	if f.writes != 1 || f.acknowledged {
		t.Fatal("unexpected destructive acknowledgement")
	}
	if err := client.ApplyAdditiveMigration(original, plan); err == nil {
		t.Fatal("accepted stale plan")
	}
	if f.writes != 1 {
		t.Fatal("stale plan mutated schema")
	}
	for _, field := range []CollectionField{{Name: "required", Type: "text", Required: true}, {Name: "unique", Type: "text", Unique: true}} {
		if _, err := PlanAdditiveMigration(original, append(target, field)); err == nil {
			t.Fatal("new constrained field needs no backfill")
		}
	}
}

func TestRecordPaginationAndCursorLoop(t *testing.T) {
	f := &schemaFixture{schemas: map[string]Collection{}}
	client := schemaClient(t, f)
	records, err := client.ListRecords("items", "")
	if err != nil || len(records) != 237 || records[236]["id"] != "236" {
		t.Fatalf("pagination: %d %v", len(records), err)
	}
	f.mu.Lock()
	f.loop = true
	f.mu.Unlock()
	if _, err := client.ListRecords("items", ""); err == nil {
		t.Fatal("accepted cursor loop")
	}
}

func TestTypedConflictAndValidationErrors(t *testing.T) {
	for _, status := range []int{409, 412, 422, 503} {
		f := &schemaFixture{schemas: map[string]Collection{}, failure: status}
		client := schemaClient(t, f)
		err := client.PatchRecord("items", "record", "1", map[string]any{"value": "safe"})
		if Status(err) != status || strings.Contains(err.Error(), "do not echo") {
			t.Fatalf("typed response: %v", err)
		}
		if (status == 409 || status == 412) != errors.Is(err, ErrConflict) {
			t.Fatal("conflict classification")
		}
		if (status == 422) != errors.Is(err, ErrValidation) {
			t.Fatal("validation classification")
		}
	}
}
