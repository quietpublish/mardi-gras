package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMetadataSchemaEmpty(t *testing.T) {
	schema := LoadMetadataSchema("")
	if schema != nil {
		t.Fatal("expected nil schema for empty projectDir")
	}
}

func TestLoadMetadataSchemaNoConfig(t *testing.T) {
	dir := t.TempDir()
	schema := LoadMetadataSchema(dir)
	if schema != nil {
		t.Fatal("expected nil schema when no .beads/config.yaml")
	}
}

func TestLoadMetadataSchemaNoValidation(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	os.MkdirAll(beadsDir, 0o755)
	os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("no-db: true\n"), 0o644)

	schema := LoadMetadataSchema(dir)
	if schema != nil {
		t.Fatal("expected nil schema when no validation.metadata section")
	}
}

func TestLoadMetadataSchemaBasic(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	os.MkdirAll(beadsDir, 0o755)

	config := `
validation:
  metadata:
    mode: warn
    fields:
      team:
        type: enum
        values: [platform, frontend, backend]
        required: true
      priority_score:
        type: int
        min: 0
        max: 100
      confidence:
        type: float
        min: 0.0
        max: 1.0
      tool:
        type: string
        required: true
      urgent:
        type: bool
`
	os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(config), 0o644)

	schema := LoadMetadataSchema(dir)
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Mode != "warn" {
		t.Fatalf("mode = %q, want %q", schema.Mode, "warn")
	}
	if len(schema.Fields) != 5 {
		t.Fatalf("fields count = %d, want 5", len(schema.Fields))
	}

	// Check team field
	team, ok := schema.Fields["team"]
	if !ok {
		t.Fatal("missing 'team' field")
	}
	if team.Type != MetaEnum {
		t.Fatalf("team.Type = %q, want %q", team.Type, MetaEnum)
	}
	if !team.Required {
		t.Fatal("team should be required")
	}
	if len(team.Values) != 3 {
		t.Fatalf("team.Values count = %d, want 3", len(team.Values))
	}

	// Check priority_score field
	ps := schema.Fields["priority_score"]
	if ps.Type != MetaInt {
		t.Fatalf("priority_score.Type = %q, want %q", ps.Type, MetaInt)
	}
	if ps.Min == nil || *ps.Min != 0 {
		t.Fatal("priority_score.Min should be 0")
	}
	if ps.Max == nil || *ps.Max != 100 {
		t.Fatal("priority_score.Max should be 100")
	}

	// Check confidence field
	conf := schema.Fields["confidence"]
	if conf.Type != MetaFloat {
		t.Fatalf("confidence.Type = %q, want %q", conf.Type, MetaFloat)
	}

	// Check bool field
	urg := schema.Fields["urgent"]
	if urg.Type != MetaBool {
		t.Fatalf("urgent.Type = %q, want %q", urg.Type, MetaBool)
	}
}

func TestLoadMetadataSchemaRedirect(t *testing.T) {
	dir := t.TempDir()

	// Create the actual beads dir elsewhere
	actualBeads := filepath.Join(dir, "actual-beads")
	os.MkdirAll(actualBeads, 0o755)
	config := `
validation:
  metadata:
    mode: error
    fields:
      env:
        type: enum
        values: [dev, staging, prod]
        required: true
`
	os.WriteFile(filepath.Join(actualBeads, "config.yaml"), []byte(config), 0o644)

	// Create .beads/ with redirect
	beadsDir := filepath.Join(dir, ".beads")
	os.MkdirAll(beadsDir, 0o755)
	// Redirect is relative to .beads/
	os.WriteFile(filepath.Join(beadsDir, "redirect"), []byte("../actual-beads"), 0o644)

	schema := LoadMetadataSchema(dir)
	if schema == nil {
		t.Fatal("expected non-nil schema via redirect")
	}
	if schema.Mode != "error" {
		t.Fatalf("mode = %q, want %q", schema.Mode, "error")
	}
	if len(schema.Fields) != 1 {
		t.Fatalf("fields count = %d, want 1", len(schema.Fields))
	}
}

func TestFieldTypeLabel(t *testing.T) {
	tests := []struct {
		name   string
		field  MetadataFieldSchema
		expect string
	}{
		{"string", MetadataFieldSchema{Type: MetaString}, "string"},
		{"int", MetadataFieldSchema{Type: MetaInt}, "int"},
		{"bool", MetadataFieldSchema{Type: MetaBool}, "bool"},
		{"enum with values", MetadataFieldSchema{Type: MetaEnum, Values: []string{"a", "b"}}, "enum[a|b]"},
		{"enum no values", MetadataFieldSchema{Type: MetaEnum}, "enum"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.field.FieldTypeLabel()
			if got != tc.expect {
				t.Fatalf("FieldTypeLabel() = %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestConstraintLabel(t *testing.T) {
	min0 := 0.0
	max100 := 100.0
	max1 := 1.0

	tests := []struct {
		name   string
		field  MetadataFieldSchema
		expect string
	}{
		{"no constraints", MetadataFieldSchema{}, ""},
		{"min only", MetadataFieldSchema{Min: &min0}, "0.."},
		{"max only", MetadataFieldSchema{Max: &max100}, "..100"},
		{"both", MetadataFieldSchema{Min: &min0, Max: &max100}, "0..100"},
		{"float max", MetadataFieldSchema{Min: &min0, Max: &max1}, "0..1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.field.ConstraintLabel()
			if got != tc.expect {
				t.Fatalf("ConstraintLabel() = %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestSortedFieldNames(t *testing.T) {
	schema := &MetadataSchema{
		Fields: map[string]MetadataFieldSchema{
			"zebra":    {Type: MetaString},
			"alpha":    {Type: MetaString, Required: true},
			"beta":     {Type: MetaString},
			"required": {Type: MetaString, Required: true},
		},
	}
	names := schema.SortedFieldNames()

	// Required fields first (alphabetical), then optional (alphabetical)
	expected := []string{"alpha", "required", "beta", "zebra"}
	if len(names) != len(expected) {
		t.Fatalf("len = %d, want %d", len(names), len(expected))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Fatalf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestDefaultModeIsNone(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	os.MkdirAll(beadsDir, 0o755)

	config := `
validation:
  metadata:
    fields:
      name:
        type: string
`
	os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte(config), 0o644)

	schema := LoadMetadataSchema(dir)
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if schema.Mode != "none" {
		t.Fatalf("mode = %q, want %q", schema.Mode, "none")
	}
}
