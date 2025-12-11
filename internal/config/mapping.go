package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// MappingConfig represents the YAML mapping configuration
type MappingConfig struct {
	Resource  string            `yaml:"resource"`
	IDColumn  string            `yaml:"id_column"`
	Mappings  map[string]string `yaml:"mappings"`
	Defaults  map[string]string `yaml:"defaults"`
	csvColumns map[string]bool  // Track available CSV columns for validation
}

// PathSegment represents a part of a FHIR path (field name or array index)
type PathSegment struct {
	Field string
	Index *int
}

var variableRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// LoadMapping loads and parses a YAML mapping file
func LoadMapping(path string) (*MappingConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read mapping file: %w", err)
	}

	var config MappingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if config.Resource == "" {
		return nil, fmt.Errorf("resource type is required in mapping file")
	}

	if config.Mappings == nil {
		config.Mappings = make(map[string]string)
	}

	if config.Defaults == nil {
		config.Defaults = make(map[string]string)
	}

	config.csvColumns = make(map[string]bool)

	return &config, nil
}

// SetCSVColumns sets the available CSV columns for validation
func (m *MappingConfig) SetCSVColumns(columns []string) {
	m.csvColumns = make(map[string]bool)
	for _, col := range columns {
		m.csvColumns[col] = true
	}
}

// ValidateColumns checks that all referenced CSV columns exist
func (m *MappingConfig) ValidateColumns() error {
	missingColumns := make(map[string]bool)

	// Check ID column
	if m.IDColumn != "" && !m.csvColumns[m.IDColumn] {
		missingColumns[m.IDColumn] = true
	}

	// Check mappings
	for _, value := range m.Mappings {
		cols := extractVariables(value)
		for _, col := range cols {
			if !m.csvColumns[col] {
				missingColumns[col] = true
			}
		}
	}

	if len(missingColumns) > 0 {
		missing := make([]string, 0, len(missingColumns))
		for col := range missingColumns {
			missing = append(missing, col)
		}
		return fmt.Errorf("missing CSV columns: %v", missing)
	}

	return nil
}

// SubstituteVariables replaces ${column_name} with values from the CSV row
func SubstituteVariables(template string, row map[string]string) string {
	return variableRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract column name from ${column_name}
		colName := match[2 : len(match)-1]
		if value, ok := row[colName]; ok {
			return value
		}
		return match // Keep original if column not found
	})
}

// extractVariables extracts all variable names from a template string
func extractVariables(template string) []string {
	matches := variableRegex.FindAllStringSubmatch(template, -1)
	vars := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			vars = append(vars, match[1])
		}
	}
	return vars
}

// ParsePath parses a FHIR path like "code.coding[0].system" into segments
func ParsePath(path string) ([]PathSegment, error) {
	segments := []PathSegment{}
	parts := strings.Split(path, ".")

	for _, part := range parts {
		// Check for array index notation: field[index]
		if strings.Contains(part, "[") {
			openIdx := strings.Index(part, "[")
			closeIdx := strings.Index(part, "]")

			if closeIdx == -1 || closeIdx < openIdx {
				return nil, fmt.Errorf("invalid array notation in path: %s", part)
			}

			field := part[:openIdx]
			indexStr := part[openIdx+1 : closeIdx]

			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return nil, fmt.Errorf("invalid array index in path: %s", part)
			}

			segments = append(segments, PathSegment{
				Field: field,
				Index: &index,
			})
		} else {
			segments = append(segments, PathSegment{
				Field: part,
				Index: nil,
			})
		}
	}

	return segments, nil
}
