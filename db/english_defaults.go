package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
)

// MigratePristineEnglishPrompt appends an English version only for an exact,
// untouched first system seed. It preserves prior versions and every customized
// prompt, including user-authored copies of default text. Non-builtin agents
// require the caller to establish separate seed provenance (for example reporter).
func (d *DB) MigratePristineEnglishPrompt(agentID int64, legacy []string, english string, requireBuiltin bool) (bool, error) {
	if english == "" || len(legacy) == 0 {
		return false, errors.New("English prompt migration requires original and translated defaults")
	}
	tx, err := d.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var builtin bool
	var version int
	var current, by, note string
	err = tx.QueryRow(`SELECT a.builtin,p.version,p.template_text,COALESCE(p.updated_by,''),COALESCE(p.note,'')
FROM agents a JOIN agent_prompts p ON p.id=a.current_prompt_id
WHERE a.id=$1 FOR UPDATE OF a,p`, agentID).Scan(&builtin, &version, &current, &by, &note)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if (requireBuiltin && !builtin) || version != 1 || by != "system" ||
		(note != "内置默认" && note != "Built-in default") || current == english || !slices.Contains(legacy, current) {
		return false, nil
	}
	// A previously edited agent can point back to version 1. Do not treat that
	// explicit history selection as an untouched installation.
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM agent_prompts WHERE agent_id=$1`, agentID).Scan(&count); err != nil {
		return false, err
	}
	if count != 1 {
		return false, nil
	}
	var id int64
	if err := tx.QueryRow(`INSERT INTO agent_prompts(agent_id,version,template_text,note,updated_by)
VALUES ($1,2,$2,'English default localization','system') RETURNING id`, agentID, english).Scan(&id); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE agents SET current_prompt_id=$1 WHERE id=$2`, id, agentID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// MigratePristineEnglishTool changes prose only when both stored description and
// schema exactly match a known built-in default. Bindings, enablement, execution
// data, and custom tools remain unchanged. JSONB comparison ignores key ordering.
func (d *DB) MigratePristineEnglishTool(key, legacyDescription, englishDescription string, legacySchema, englishSchema json.RawMessage) (bool, error) {
	if !sameSchemaContract(legacySchema, englishSchema) {
		return false, errors.New("English tool migration must preserve the schema contract")
	}
	result, err := d.Exec(`UPDATE tools SET description=$3,schema=$5,updated_at=now()
WHERE key=$1 AND system AND kind='builtin' AND description=$2 AND schema=$4::jsonb
AND (description IS DISTINCT FROM $3 OR schema IS DISTINCT FROM $5::jsonb)`,
		key, legacyDescription, englishDescription, legacySchema, englishSchema)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n != 0, err
}

func sameSchemaContract(before, after json.RawMessage) bool {
	var a, b any
	if json.Unmarshal(before, &a) != nil || json.Unmarshal(after, &b) != nil {
		return false
	}
	var stripProse func(any)
	stripProse = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				// These keywords contain instance data, not schemas. A field named
				// description/title inside them is part of the contract.
				if key == "default" || key == "const" || key == "enum" || key == "examples" {
					continue
				}
				// A property literally named description/title has an object value;
				// preserve that property and only remove string-valued schema prose.
				if _, ok := child.(string); ok && (key == "description" || key == "title") {
					delete(value, key)
				} else {
					stripProse(child)
				}
			}
		case []any:
			for _, child := range value {
				stripProse(child)
			}
		}
	}
	stripProse(a)
	stripProse(b)
	return reflect.DeepEqual(a, b)
}

// SameSchemaContract exposes the prose-insensitive contract comparison for
// narrow versioned tool upgrades; the prose-only migration keeps its own guard.
func SameSchemaContract(before, after json.RawMessage) bool {
	return sameSchemaContract(before, after)
}
