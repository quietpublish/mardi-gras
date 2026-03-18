package data

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MetadataFieldType defines the type of a metadata field for schema validation.
type MetadataFieldType string

const (
	MetaString MetadataFieldType = "string"
	MetaInt    MetadataFieldType = "int"
	MetaFloat  MetadataFieldType = "float"
	MetaBool   MetadataFieldType = "bool"
	MetaEnum   MetadataFieldType = "enum"
)

// MetadataFieldSchema defines validation rules for a single metadata field.
type MetadataFieldSchema struct {
	Type     MetadataFieldType `yaml:"type"`
	Required bool              `yaml:"required"`
	Values   []string          `yaml:"values"`
	Min      *float64          `yaml:"min"`
	Max      *float64          `yaml:"max"`
}

// MetadataSchema holds the parsed metadata validation configuration.
type MetadataSchema struct {
	Mode   string                         `yaml:"mode"`
	Fields map[string]MetadataFieldSchema `yaml:"fields"`
}

// beadsConfig represents the top-level .beads/config.yaml structure,
// only parsing the validation.metadata section we care about.
type beadsConfig struct {
	IssuePrefix string `yaml:"issue-prefix"`
	Validation  struct {
		Metadata MetadataSchema `yaml:"metadata"`
	} `yaml:"validation"`
}

type beadsMetadata struct {
	DoltDatabase string `json:"dolt_database"`
}

// LoadMetadataSchema loads validation.metadata from .beads/config.yaml,
// resolving redirect if present. Returns nil if not configured.
func LoadMetadataSchema(projectDir string) *MetadataSchema {
	if projectDir == "" {
		return nil
	}

	beadsDir := ResolveBeadsDir(filepath.Join(projectDir, ".beads"))
	configPath := filepath.Join(beadsDir, "config.yaml")

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var cfg beadsConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil
	}

	schema := &cfg.Validation.Metadata
	if len(schema.Fields) == 0 {
		return nil
	}

	if schema.Mode == "" {
		schema.Mode = "none"
	}

	return schema
}

// LoadIssuePrefix returns the configured issue prefix when it can be resolved
// from the local Beads metadata. It prefers config.yaml and falls back to the
// conventional dolt database name in metadata.json (beads_<prefix>).
func LoadIssuePrefix(projectDir string) string {
	if projectDir == "" {
		return ""
	}

	beadsDir := ResolveBeadsDir(filepath.Join(projectDir, ".beads"))
	configPath := filepath.Join(beadsDir, "config.yaml")
	raw, err := os.ReadFile(configPath)
	if err == nil {
		var cfg beadsConfig
		if yaml.Unmarshal(raw, &cfg) == nil {
			if prefix := strings.TrimSpace(cfg.IssuePrefix); prefix != "" {
				return prefix
			}
		}
	}

	metadataPath := filepath.Join(beadsDir, "metadata.json")
	raw, err = os.ReadFile(metadataPath)
	if err != nil {
		return ""
	}

	var meta beadsMetadata
	if json.Unmarshal(raw, &meta) != nil {
		return ""
	}
	return issuePrefixFromDatabase(meta.DoltDatabase)
}

func issuePrefixFromDatabase(name string) string {
	if !strings.HasPrefix(name, "beads_") {
		return ""
	}
	return strings.TrimPrefix(name, "beads_")
}

// ResolveBeadsDir follows a redirect file if present.
// .beads/redirect contains a relative path to the actual beads directory.
// Rejects targets containing ".." path components to prevent directory traversal.
func ResolveBeadsDir(beadsDir string) string {
	redirectPath := filepath.Join(beadsDir, "redirect")
	content, err := os.ReadFile(redirectPath)
	if err != nil {
		return beadsDir
	}

	target := strings.TrimSpace(string(content))
	if target == "" {
		return beadsDir
	}

	resolved := filepath.Clean(filepath.Join(beadsDir, target))

	// Reject directory traversal: the resolved path must stay within
	// the project root (the parent of beadsDir). This allows redirects
	// like "../actual-beads" (sibling of .beads) but blocks escaping
	// above the project root.
	projectRoot := filepath.Dir(beadsDir)
	if !strings.HasPrefix(resolved, projectRoot+string(filepath.Separator)) && resolved != projectRoot {
		return beadsDir
	}

	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved
	}

	return beadsDir
}

// FieldTypeLabel returns a display label for a metadata field type.
func (f MetadataFieldSchema) FieldTypeLabel() string {
	label := string(f.Type)
	if f.Type == MetaEnum && len(f.Values) > 0 {
		label += "[" + strings.Join(f.Values, "|") + "]"
	}
	return label
}

// ConstraintLabel returns a display label for numeric constraints (min/max).
func (f MetadataFieldSchema) ConstraintLabel() string {
	if f.Min == nil && f.Max == nil {
		return ""
	}
	minStr := ""
	maxStr := ""
	if f.Min != nil {
		minStr = compactFloat(*f.Min)
	}
	if f.Max != nil {
		maxStr = compactFloat(*f.Max)
	}
	return minStr + ".." + maxStr
}

// compactFloat formats a float64 without trailing zeros.
func compactFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%d", int64(f))
	}
	return strings.TrimRight(fmt.Sprintf("%.6f", f), "0")
}

// SortedFieldNames returns field names sorted alphabetically,
// with required fields first.
func (s *MetadataSchema) SortedFieldNames() []string {
	required := make([]string, 0, len(s.Fields))
	optional := make([]string, 0, len(s.Fields))
	for name, field := range s.Fields {
		if field.Required {
			required = append(required, name)
		} else {
			optional = append(optional, name)
		}
	}
	sort.Strings(required)
	sort.Strings(optional)
	return append(required, optional...)
}
