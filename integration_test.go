package main

import (
	"csv2fhir/internal/config"
	"csv2fhir/internal/csv"
	"csv2fhir/internal/output"
	"csv2fhir/internal/transform"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// TestFullPipeline_Observation tests the complete pipeline with sample.csv
func TestFullPipeline_Observation(t *testing.T) {
	// Use the existing sample files
	csvPath := "examples/sample.csv"
	mappingPath := "examples/sample-mapping.yaml"

	// Check if files exist
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Skip("sample.csv not found, skipping integration test")
	}
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		t.Skip("sample-mapping.yaml not found, skipping integration test")
	}

	// Load mapping
	cfg, err := config.LoadMapping(mappingPath)
	if err != nil {
		t.Fatalf("Failed to load mapping: %v", err)
	}

	// Open CSV reader
	reader, err := csv.NewReader(csvPath, ',')
	if err != nil {
		t.Fatalf("Failed to open CSV: %v", err)
	}
	defer reader.Close()

	// Set CSV columns for validation
	cfg.SetCSVColumns(reader.Headers())
	if err := cfg.ValidateColumns(); err != nil {
		t.Fatalf("Column validation failed: %v", err)
	}

	// Create transformer
	transformer := transform.NewTransformer(cfg)

	// Process rows
	resources := []interface{}{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read row: %v", err)
		}

		resource, err := transformer.Transform(row.Data, row.RowNumber)
		if err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		resources = append(resources, resource)
	}

	// Verify we got observations
	if len(resources) == 0 {
		t.Fatal("No resources created")
	}

	// Check first resource is an Observation
	obs, ok := resources[0].(*fhir.Observation)
	if !ok {
		t.Fatalf("Expected Observation, got %T", resources[0])
	}

	// Verify some fields were set
	if obs.Subject == nil || obs.Subject.Reference == nil {
		t.Error("Subject reference not set")
	}
	if obs.Id == nil {
		t.Error("ID not set")
	}
}

// TestFullPipeline_EdgeCases tests the complete pipeline with edge cases
func TestFullPipeline_EdgeCases(t *testing.T) {
	csvPath := "examples/test-edge-cases.csv"
	mappingPath := "examples/test-edge-mapping.yaml"

	// Check if files exist
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Skip("test-edge-cases.csv not found, skipping integration test")
	}
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		t.Skip("test-edge-mapping.yaml not found, skipping integration test")
	}

	// Load mapping
	cfg, err := config.LoadMapping(mappingPath)
	if err != nil {
		t.Fatalf("Failed to load mapping: %v", err)
	}

	// Open CSV reader
	reader, err := csv.NewReader(csvPath, ',')
	if err != nil {
		t.Fatalf("Failed to open CSV: %v", err)
	}
	defer reader.Close()

	// Set CSV columns for validation
	cfg.SetCSVColumns(reader.Headers())
	if err := cfg.ValidateColumns(); err != nil {
		t.Fatalf("Column validation failed: %v", err)
	}

	// Create transformer
	transformer := transform.NewTransformer(cfg)

	// Process rows
	resources := []interface{}{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read row: %v", err)
		}

		resource, err := transformer.Transform(row.Data, row.RowNumber)
		if err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		resources = append(resources, resource)
	}

	if len(resources) == 0 {
		t.Fatal("No resources created")
	}
}

// TestBundleOutput tests Bundle format output
func TestBundleOutput(t *testing.T) {
	// Create temp output file
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.json")

	// Create some sample observations
	observations := []interface{}{
		&fhir.Observation{Id: strPtr("OBS1")},
		&fhir.Observation{Id: strPtr("OBS2")},
	}

	// Create bundle writer
	writer, err := output.NewWriterWithLimit(outputPath, output.FormatBundle, 10)
	if err != nil {
		t.Fatalf("Failed to create bundle writer: %v", err)
	}

	// Write resources
	for _, obs := range observations {
		if err := writer.Write(obs); err != nil {
			t.Fatalf("Failed to write resource: %v", err)
		}
	}

	// Close writer
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Read and verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	var bundle fhir.Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("Failed to parse bundle: %v", err)
	}

	if bundle.Total == nil || *bundle.Total != 2 {
		t.Error("Expected total=2 in bundle")
	}
	if len(bundle.Entry) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(bundle.Entry))
	}
}

// TestNDJSONOutput tests NDJSON format output
func TestNDJSONOutput(t *testing.T) {
	// Create temp output file
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.ndjson")

	// Create some sample observations
	observations := []interface{}{
		&fhir.Observation{Id: strPtr("OBS1")},
		&fhir.Observation{Id: strPtr("OBS2")},
		&fhir.Observation{Id: strPtr("OBS3")},
	}

	// Create NDJSON writer
	writer, err := output.NewWriter(outputPath, output.FormatNDJSON)
	if err != nil {
		t.Fatalf("Failed to create NDJSON writer: %v", err)
	}

	// Write resources
	for _, obs := range observations {
		if err := writer.Write(obs); err != nil {
			t.Fatalf("Failed to write resource: %v", err)
		}
	}

	// Close writer
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Read and verify output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	// NDJSON should have 3 lines
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("Expected 3 lines in NDJSON, got %d", lines)
	}
}

// TestMemoryLimit tests resource limit enforcement
func TestMemoryLimit(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.json")

	// Create bundle writer with limit of 2
	writer, err := output.NewWriterWithLimit(outputPath, output.FormatBundle, 2)
	if err != nil {
		t.Fatalf("Failed to create bundle writer: %v", err)
	}

	// Write 2 resources - should succeed
	if err := writer.Write(&fhir.Observation{Id: strPtr("OBS1")}); err != nil {
		t.Fatalf("Failed to write first resource: %v", err)
	}
	if err := writer.Write(&fhir.Observation{Id: strPtr("OBS2")}); err != nil {
		t.Fatalf("Failed to write second resource: %v", err)
	}

	// Write 3rd resource - should fail
	err = writer.Write(&fhir.Observation{Id: strPtr("OBS3")})
	if err == nil {
		t.Fatal("Expected error when exceeding memory limit, got nil")
	}

	writer.Close()
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
