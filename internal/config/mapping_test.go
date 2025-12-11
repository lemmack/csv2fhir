package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMapping tests loading a valid YAML mapping file
func TestLoadMapping(t *testing.T) {
	// Create a temporary YAML file
	content := `resource: Observation
id_column: record_id
mappings:
  status: "final"
  code.coding[0].system: "http://loinc.org"
defaults:
  status: "preliminary"
`
	tmpFile := createTempYAMLFile(t, content)
	defer os.Remove(tmpFile)

	config, err := LoadMapping(tmpFile)
	if err != nil {
		t.Fatalf("LoadMapping failed: %v", err)
	}

	if config.Resource != "Observation" {
		t.Errorf("Expected resource Observation, got %s", config.Resource)
	}
	if config.IDColumn != "record_id" {
		t.Errorf("Expected id_column record_id, got %s", config.IDColumn)
	}
	if len(config.Mappings) != 2 {
		t.Errorf("Expected 2 mappings, got %d", len(config.Mappings))
	}
	if len(config.Defaults) != 1 {
		t.Errorf("Expected 1 default, got %d", len(config.Defaults))
	}
}

// TestLoadMapping_FileNotFound tests error handling for missing files
func TestLoadMapping_FileNotFound(t *testing.T) {
	_, err := LoadMapping("/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
}

// TestLoadMapping_InvalidYAML tests error handling for malformed YAML
func TestLoadMapping_InvalidYAML(t *testing.T) {
	content := `resource: Observation
mappings:
  - this is invalid yaml without proper key: value
    bad indentation
`
	tmpFile := createTempYAMLFile(t, content)
	defer os.Remove(tmpFile)

	_, err := LoadMapping(tmpFile)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

// TestLoadMapping_MissingResource tests error when resource type is missing
func TestLoadMapping_MissingResource(t *testing.T) {
	content := `id_column: record_id
mappings:
  status: "final"
`
	tmpFile := createTempYAMLFile(t, content)
	defer os.Remove(tmpFile)

	_, err := LoadMapping(tmpFile)
	if err == nil {
		t.Fatal("Expected error for missing resource, got nil")
	}
}

// TestSetCSVColumns tests setting CSV columns for validation
func TestSetCSVColumns(t *testing.T) {
	config := &MappingConfig{}
	columns := []string{"col1", "col2", "col3"}

	config.SetCSVColumns(columns)

	if len(config.csvColumns) != 3 {
		t.Errorf("Expected 3 CSV columns, got %d", len(config.csvColumns))
	}
	if !config.csvColumns["col1"] {
		t.Error("Expected col1 to be set")
	}
	if !config.csvColumns["col2"] {
		t.Error("Expected col2 to be set")
	}
}

// TestValidateColumns tests validation of all referenced columns
func TestValidateColumns(t *testing.T) {
	config := &MappingConfig{
		IDColumn: "id",
		Mappings: map[string]string{
			"field1": "${col1}",
			"field2": "static value",
			"field3": "Patient/${patient_id}",
		},
	}

	config.SetCSVColumns([]string{"id", "col1", "patient_id"})

	err := config.ValidateColumns()
	if err != nil {
		t.Errorf("ValidateColumns failed: %v", err)
	}
}

// TestValidateColumns_MissingColumns tests error for missing CSV columns
func TestValidateColumns_MissingColumns(t *testing.T) {
	config := &MappingConfig{
		Mappings: map[string]string{
			"field1": "${col1}",
			"field2": "${col2}",
			"field3": "${missing_col}",
		},
	}

	config.SetCSVColumns([]string{"col1", "col2"})

	err := config.ValidateColumns()
	if err == nil {
		t.Fatal("Expected error for missing columns, got nil")
	}
}

// TestValidateColumns_MissingIDColumn tests error for missing ID column
func TestValidateColumns_MissingIDColumn(t *testing.T) {
	config := &MappingConfig{
		IDColumn: "missing_id",
		Mappings: map[string]string{
			"field1": "${col1}",
		},
	}

	config.SetCSVColumns([]string{"col1"})

	err := config.ValidateColumns()
	if err == nil {
		t.Fatal("Expected error for missing ID column, got nil")
	}
}

// TestSubstituteVariables tests variable substitution
func TestSubstituteVariables(t *testing.T) {
	row := map[string]string{
		"name":       "John",
		"patient_id": "123",
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "single variable",
			template: "${name}",
			want:     "John",
		},
		{
			name:     "multiple variables",
			template: "Patient/${patient_id}",
			want:     "Patient/123",
		},
		{
			name:     "no variables",
			template: "static text",
			want:     "static text",
		},
		{
			name:     "mixed content",
			template: "Hello ${name}, your ID is ${patient_id}",
			want:     "Hello John, your ID is 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SubstituteVariables(tt.template, row)
			if err != nil {
				t.Errorf("SubstituteVariables failed: %v", err)
			}
			if result != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, result)
			}
		})
	}
}

// TestSubstituteVariables_MissingColumn tests error for missing variables
func TestSubstituteVariables_MissingColumn(t *testing.T) {
	row := map[string]string{
		"name": "John",
	}

	_, err := SubstituteVariables("${name} ${missing}", row)
	if err == nil {
		t.Fatal("Expected error for missing column, got nil")
	}
}

// TestSubstituteVariables_EmptyValue tests handling of empty values
func TestSubstituteVariables_EmptyValue(t *testing.T) {
	row := map[string]string{
		"name":  "John",
		"empty": "",
	}

	result, err := SubstituteVariables("${name}-${empty}", row)
	if err != nil {
		t.Fatalf("SubstituteVariables failed: %v", err)
	}
	if result != "John-" {
		t.Errorf("Expected 'John-', got %s", result)
	}
}

// TestParsePath tests parsing valid FHIR paths
func TestParsePath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantSegments int
	}{
		{
			name:         "simple field",
			path:         "status",
			wantSegments: 1,
		},
		{
			name:         "nested field",
			path:         "code.coding",
			wantSegments: 2,
		},
		{
			name:         "array index",
			path:         "coding[0]",
			wantSegments: 1,
		},
		{
			name:         "complex path",
			path:         "code.coding[0].system",
			wantSegments: 3,
		},
		{
			name:         "multiple array indices",
			path:         "coding[0].display[1]",
			wantSegments: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := ParsePath(tt.path)
			if err != nil {
				t.Errorf("ParsePath failed: %v", err)
			}
			if len(segments) != tt.wantSegments {
				t.Errorf("Expected %d segments, got %d", tt.wantSegments, len(segments))
			}
		})
	}
}

// TestParsePath_InvalidFormats tests error handling for invalid paths
func TestParsePath_InvalidFormats(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "empty path",
			path: "",
		},
		{
			name: "leading dot",
			path: ".code",
		},
		{
			name: "trailing dot",
			path: "code.",
		},
		{
			name: "consecutive dots",
			path: "code..system",
		},
		{
			name: "empty field name",
			path: "code..system",
		},
		{
			name: "invalid array notation - missing close bracket",
			path: "code[0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePath(tt.path)
			if err == nil {
				t.Errorf("Expected error for path %s, got nil", tt.path)
			}
		})
	}
}

// TestParsePath_ArrayIndices tests array index handling
func TestParsePath_ArrayIndices(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantIndex  int
		wantField  string
		shouldFail bool
	}{
		{
			name:       "zero index",
			path:       "coding[0]",
			wantIndex:  0,
			wantField:  "coding",
			shouldFail: false,
		},
		{
			name:       "positive index",
			path:       "coding[5]",
			wantIndex:  5,
			wantField:  "coding",
			shouldFail: false,
		},
		{
			name:       "max allowed index",
			path:       "coding[1000]",
			wantIndex:  1000,
			wantField:  "coding",
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := ParsePath(tt.path)
			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePath failed: %v", err)
			}
			if len(segments) != 1 {
				t.Fatalf("Expected 1 segment, got %d", len(segments))
			}
			if segments[0].Field != tt.wantField {
				t.Errorf("Expected field %s, got %s", tt.wantField, segments[0].Field)
			}
			if segments[0].Index == nil {
				t.Fatal("Expected index, got nil")
			}
			if *segments[0].Index != tt.wantIndex {
				t.Errorf("Expected index %d, got %d", tt.wantIndex, *segments[0].Index)
			}
		})
	}
}

// TestParsePath_NegativeIndex tests error for negative array indices
func TestParsePath_NegativeIndex(t *testing.T) {
	_, err := ParsePath("coding[-1]")
	if err == nil {
		t.Error("Expected error for negative index, got nil")
	}
}

// TestParsePath_ExceedsLimit tests error for array index > 1000
func TestParsePath_ExceedsLimit(t *testing.T) {
	_, err := ParsePath("coding[1001]")
	if err == nil {
		t.Error("Expected error for index > 1000, got nil")
	}
}

// TestParsePath_InvalidArrayIndex tests error for non-numeric index
func TestParsePath_InvalidArrayIndex(t *testing.T) {
	_, err := ParsePath("coding[abc]")
	if err == nil {
		t.Error("Expected error for non-numeric index, got nil")
	}
}

// Helper function to create a temporary YAML file
func createTempYAMLFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-mapping.yaml")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}
