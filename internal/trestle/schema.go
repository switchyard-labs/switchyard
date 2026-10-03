package trestle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Collection struct {
	Name   string            `json:"name"`
	Kind   string            `json:"kind"`
	Fields []CollectionField `json:"fields"`
}
type SchemaMismatch struct{ Collection, Field, Reason string }

func (e *SchemaMismatch) Error() string {
	return fmt.Sprintf("incompatible Switchyard schema %s.%s: %s; explicit migration required", e.Collection, e.Field, e.Reason)
}

func CompatibleSchema(actual Collection, expected []CollectionField) error {
	if actual.Kind != "" && actual.Kind != "base" {
		return &SchemaMismatch{actual.Name, "", "collection is not writable base kind"}
	}
	fields := map[string]CollectionField{}
	for _, field := range actual.Fields {
		if _, exists := fields[field.Name]; exists {
			return &SchemaMismatch{actual.Name, field.Name, "duplicate field"}
		}
		fields[field.Name] = field
	}
	for _, wanted := range expected {
		got, ok := fields[wanted.Name]
		if !ok {
			return &SchemaMismatch{actual.Name, wanted.Name, "field missing"}
		}
		if got.Type != wanted.Type || got.Required != wanted.Required || got.Unique != wanted.Unique {
			return &SchemaMismatch{actual.Name, wanted.Name, "type or required/unique constraint differs"}
		}
		delete(fields, wanted.Name)
	}
	for _, extra := range fields {
		if extra.Required {
			return &SchemaMismatch{actual.Name, extra.Name, "unknown required field"}
		}
	}
	return nil
}

// EnsureCollection creates new collections, but never edits an existing
// schema implicitly. Name-only existence is insufficient for compatibility.
func (c *Client) EnsureCollection(name string, fields []CollectionField) error {
	path := "/admin/v1/collections/" + url.PathEscape(name)
	resp, body, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == 404 {
		resp, body, err = c.do(http.MethodPost, "/admin/v1/collections", map[string]any{"name": name, "fields": fields})
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 409 {
			return responseError("create schema", name, resp.StatusCode, body)
		}
		resp, body, err = c.do(http.MethodGet, path, nil)
		if err != nil {
			return err
		}
	}
	if resp.StatusCode != 200 {
		return responseError("read schema", name, resp.StatusCode, body)
	}
	var actual Collection
	if err = json.Unmarshal(body, &actual); err != nil {
		return err
	}
	if actual.Name != name {
		return &SchemaMismatch{name, "", "unexpected collection identity"}
	}
	return CompatibleSchema(actual, fields)
}

// PlanAdditiveMigration preserves field identity/defaults and refuses every
// constraint/type change or unbackfilled required/unique field.
func PlanAdditiveMigration(actual Collection, target []CollectionField) (Collection, error) {
	if actual.Kind != "" && actual.Kind != "base" {
		return Collection{}, &SchemaMismatch{actual.Name, "", "not a base collection"}
	}
	result := actual
	result.Fields = append([]CollectionField(nil), actual.Fields...)
	existing := map[string]CollectionField{}
	for _, field := range actual.Fields {
		existing[field.Name] = field
	}
	for _, field := range target {
		got, exists := existing[field.Name]
		if exists {
			if got.Type != field.Type || got.Required != field.Required || got.Unique != field.Unique {
				return Collection{}, &SchemaMismatch{actual.Name, field.Name, "constraint/type change requires reviewed backfill"}
			}
		} else {
			if field.Required || field.Unique {
				return Collection{}, &SchemaMismatch{actual.Name, field.Name, "new required/unique field requires reviewed backfill"}
			}
			result.Fields = append(result.Fields, field)
		}
	}
	if err := CompatibleSchema(result, target); err != nil {
		return Collection{}, err
	}
	return result, nil
}

func (c *Client) ReadCollection(name string) (*Collection, error) {
	resp, body, err := c.do(http.MethodGet, "/admin/v1/collections/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, responseError("schema read", name, resp.StatusCode, body)
	}
	var collection Collection
	if err = json.Unmarshal(body, &collection); err != nil {
		return nil, err
	}
	if collection.Name != name {
		return nil, &SchemaMismatch{name, "", "wrong collection identity"}
	}
	return &collection, nil
}

// ApplyAdditiveMigration is an operator maintenance-window operation. The
// upstream schema API has no If-Match contract; callers must quiesce schema
// administrators. Startup does not call this method.
func (c *Client) ApplyAdditiveMigration(before, after Collection) error {
	if before.Name != after.Name {
		return fmt.Errorf("migration collection identity changed")
	}
	planned, err := PlanAdditiveMigration(before, after.Fields)
	if err != nil {
		return err
	}
	plannedJSON, err := json.Marshal(planned)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	if string(plannedJSON) != string(afterJSON) {
		return fmt.Errorf("migration must preserve existing fields and defaults")
	}
	current, err := c.ReadCollection(before.Name)
	if err != nil {
		return err
	}
	oldJSON, _ := json.Marshal(before)
	currentJSON, _ := json.Marshal(current)
	if string(oldJSON) != string(currentJSON) {
		return &SchemaMismatch{before.Name, "", "schema changed since migration planning"}
	}
	// The update contract accepts name and fields only. Kind is read-only;
	// Trestle rejects it rather than silently dropping unknown JSON fields.
	resp, body, err := c.do(http.MethodPatch, "/admin/v1/collections/"+url.PathEscape(before.Name), map[string]any{"name": after.Name, "fields": after.Fields})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return responseError("schema migrate", before.Name, resp.StatusCode, body)
	}
	verified, err := c.ReadCollection(before.Name)
	if err != nil {
		return err
	}
	if verified == nil {
		return fmt.Errorf("migration collection disappeared")
	}
	return CompatibleSchema(*verified, after.Fields)
}
