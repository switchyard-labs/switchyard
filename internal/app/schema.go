package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"switchyard/internal/trestle"
)

// Bump whenever the declared schema changes; old versions remain historical.
const switchyardSchemaVersion = 16

func schemaFingerprint(collections [][2]any) (string, error) {
	type definition struct {
		Name   string                    `json:"name"`
		Fields []trestle.CollectionField `json:"fields"`
	}
	definitions := []definition{}
	names := map[string]bool{}
	for _, item := range collections {
		name, ok := item[0].(string)
		if !ok || names[name] {
			return "", fmt.Errorf("invalid/duplicate schema identity %v", item[0])
		}
		names[name] = true
		fields, ok := item[1].([]trestle.CollectionField)
		if !ok {
			return "", fmt.Errorf("invalid schema fields %s", name)
		}
		fields = append([]trestle.CollectionField(nil), fields...)
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		definitions = append(definitions, definition{name, fields})
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
	encoded, err := json.Marshal(definitions)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func (a *App) provisionSchema() error {
	ledgerFields := []trestle.CollectionField{{Name: "version", Type: "text", Unique: true}, {Name: "fingerprint", Type: "text", Required: true}, {Name: "applied_at", Type: "text"}}
	if err := a.Trestle.EnsureCollection("switchyard_schema", ledgerFields); err != nil {
		return err
	}
	collections := switchyardCollections()
	fingerprint, err := schemaFingerprint(collections)
	if err != nil {
		return err
	}
	versions, err := a.Trestle.ListRecords("switchyard_schema", "")
	if err != nil {
		return err
	}
	currentRecorded := false
	for _, version := range versions {
		n, err := strconv.Atoi(strOf(version["version"]))
		if err != nil || n < 1 {
			return fmt.Errorf("invalid Switchyard migration version")
		}
		if n > switchyardSchemaVersion {
			return fmt.Errorf("database schema version %d is newer than supported %d", n, switchyardSchemaVersion)
		}
		if n == switchyardSchemaVersion {
			if version["fingerprint"] != fingerprint {
				return fmt.Errorf("schema manifest changed without migration version bump")
			}
			currentRecorded = true
		}
	}
	for _, collection := range collections {
		if err := a.Trestle.EnsureCollection(collection[0].(string), collection[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	if currentRecorded {
		return nil
	}
	version := strconv.Itoa(switchyardSchemaVersion)
	_, _, err = a.Trestle.CreateRecord("switchyard_schema", map[string]any{"version": version, "fingerprint": fingerprint, "applied_at": nowStr()}, "switchyard-schema-"+version)
	if err != nil {
		_, _, record, readErr := a.Trestle.FindRecord("switchyard_schema", filterEq("version", version))
		if readErr == nil && record["fingerprint"] == fingerprint {
			return nil
		}
	}
	return err
}

type SchemaMigration struct {
	Name      string   `json:"name"`
	Create    bool     `json:"create"`
	AddFields []string `json:"add_fields,omitempty"`
	before    *trestle.Collection
	after     trestle.Collection
}

// SchemaMigrationPlan is read-only. Type/constraint/backfill changes are
// blockers, not implicitly acknowledged destructive alterations.
func (a *App) SchemaMigrationPlan() ([]SchemaMigration, error) {
	collections := switchyardCollections()
	fingerprint, err := schemaFingerprint(collections)
	if err != nil {
		return nil, err
	}
	ledger, err := a.Trestle.ReadCollection("switchyard_schema")
	if err != nil {
		return nil, err
	}
	if ledger != nil {
		records, err := a.Trestle.ListRecords("switchyard_schema", "")
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			version, err := strconv.Atoi(strOf(record["version"]))
			if err != nil {
				return nil, fmt.Errorf("invalid schema version")
			}
			if version > switchyardSchemaVersion {
				return nil, fmt.Errorf("database schema is newer than this binary")
			}
			if version == switchyardSchemaVersion && record["fingerprint"] != fingerprint {
				return nil, fmt.Errorf("schema manifest changed without version bump")
			}
		}
	}
	plan := []SchemaMigration{}
	for _, item := range collections {
		name, fields := item[0].(string), item[1].([]trestle.CollectionField)
		actual, err := a.Trestle.ReadCollection(name)
		if err != nil {
			return nil, err
		}
		if actual == nil {
			plan = append(plan, SchemaMigration{Name: name, Create: true, after: trestle.Collection{Name: name, Fields: fields}})
			continue
		}
		if err := trestle.CompatibleSchema(*actual, fields); err == nil {
			continue
		}
		after, err := trestle.PlanAdditiveMigration(*actual, fields)
		if err != nil {
			return nil, err
		}
		names := map[string]bool{}
		for _, field := range actual.Fields {
			names[field.Name] = true
		}
		added := []string{}
		for _, field := range after.Fields {
			if !names[field.Name] {
				added = append(added, field.Name)
			}
		}
		plan = append(plan, SchemaMigration{Name: name, AddFields: added, before: actual, after: after})
	}
	return plan, nil
}

// ApplySchemaMigrations is only exposed through an explicit operator CLI flag.
// No destructive-acknowledgement header is ever sent to Trestle.
func (a *App) ApplySchemaMigrations(plan []SchemaMigration) error {
	for _, migration := range plan {
		if migration.Create {
			if err := a.Trestle.EnsureCollection(migration.Name, migration.after.Fields); err != nil {
				return err
			}
		} else {
			if migration.before == nil {
				return fmt.Errorf("migration plan missing source schema")
			}
			if err := a.Trestle.ApplyAdditiveMigration(*migration.before, migration.after); err != nil {
				return err
			}
		}
	}
	return a.provisionSchema()
}
