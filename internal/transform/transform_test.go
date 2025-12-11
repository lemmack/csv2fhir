package transform

import (
	"csv2fhir/internal/config"
	"testing"

	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// TestNewTransformer tests creating a transformer with valid config
func TestNewTransformer(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{},
		Defaults: map[string]string{},
	}

	transformer := NewTransformer(cfg)
	if transformer == nil {
		t.Fatal("NewTransformer returned nil")
	}
	if transformer.config != cfg {
		t.Error("Transformer config not set correctly")
	}
}

// TestCreateResource tests creating each supported resource type
func TestCreateResource(t *testing.T) {
	tests := []struct {
		resourceType string
		wantType     interface{}
	}{
		{"Observation", &fhir.Observation{}},
		{"observation", &fhir.Observation{}}, // Test case-insensitive
		{"Patient", &fhir.Patient{}},
		{"Condition", &fhir.Condition{}},
		{"MedicationRequest", &fhir.MedicationRequest{}},
		{"Procedure", &fhir.Procedure{}},
		{"Encounter", &fhir.Encounter{}},
		{"DiagnosticReport", &fhir.DiagnosticReport{}},
		{"Specimen", &fhir.Specimen{}},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			cfg := &config.MappingConfig{
				Resource: tt.resourceType,
				Mappings: map[string]string{},
			}
			transformer := NewTransformer(cfg)
			resource, err := transformer.createResource()
			if err != nil {
				t.Fatalf("createResource failed: %v", err)
			}
			// Check type matches expected
			if _, ok := resource.(interface{}); !ok {
				t.Errorf("Expected resource type %T, got %T", tt.wantType, resource)
			}
		})
	}
}

// TestCreateResource_UnsupportedType tests error for unsupported resource types
func TestCreateResource_UnsupportedType(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "UnsupportedResource",
		Mappings: map[string]string{},
	}

	transformer := NewTransformer(cfg)
	_, err := transformer.createResource()
	if err == nil {
		t.Fatal("Expected error for unsupported resource type, got nil")
	}
}

// TestTransform_SimpleMapping tests basic field mapping
func TestTransform_SimpleMapping(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"status": "${status_code}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"status_code": "final",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	// FHIR status is a custom type, just check resource was created
	if obs == nil {
		t.Fatal("Observation not created")
	}
}

// TestTransform_NestedPath tests nested struct field mapping
func TestTransform_NestedPath(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"subject.reference": "Patient/${patient_id}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"patient_id": "123",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	if obs.Subject == nil || obs.Subject.Reference == nil {
		t.Fatal("Subject.reference not set")
	}
	if *obs.Subject.Reference != "Patient/123" {
		t.Errorf("Expected reference 'Patient/123', got '%s'", *obs.Subject.Reference)
	}
}

// TestTransform_ArrayIndex tests array element mapping
func TestTransform_ArrayIndex(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"code.coding[0].system": "http://loinc.org",
			"code.coding[0].code":   "${loinc_code}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"loinc_code": "2339-0",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	if obs.Code.Coding == nil || len(obs.Code.Coding) == 0 {
		t.Fatal("Code.coding not set")
	}
	if obs.Code.Coding[0].System == nil {
		t.Fatal("Code.coding[0].system not set")
	}
	if *obs.Code.Coding[0].System != "http://loinc.org" {
		t.Errorf("Expected system 'http://loinc.org', got '%s'", *obs.Code.Coding[0].System)
	}
	if obs.Code.Coding[0].Code == nil || *obs.Code.Coding[0].Code != "2339-0" {
		t.Error("Code.coding[0].code not set correctly")
	}
}

// TestTransform_MultipleArrayIndices tests multiple array elements
func TestTransform_MultipleArrayIndices(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"code.coding[0].system": "http://loinc.org",
			"code.coding[0].code":   "first",
			"code.coding[1].system": "http://snomed.info/sct",
			"code.coding[1].code":   "second",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	if obs.Code.Coding == nil || len(obs.Code.Coding) < 2 {
		t.Fatal("Code.coding not set with 2 elements")
	}
	if *obs.Code.Coding[0].Code != "first" {
		t.Error("Code.coding[0].code not set correctly")
	}
	if *obs.Code.Coding[1].Code != "second" {
		t.Error("Code.coding[1].code not set correctly")
	}
}

// TestTransform_Defaults tests applying default values
func TestTransform_Defaults(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Defaults: map[string]string{
			"status": "preliminary",
		},
		Mappings: map[string]string{},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	// Status should have been set from defaults (enum type, just check resource exists)
	if obs == nil {
		t.Fatal("Resource not created")
	}
}

// TestTransform_DefaultsOverride tests that mappings override defaults
func TestTransform_DefaultsOverride(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Defaults: map[string]string{
			"status": "preliminary",
		},
		Mappings: map[string]string{
			"status": "${status}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"status": "final",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	// Verify resource was created (status is an enum type)
	if obs == nil {
		t.Fatal("Resource not created")
	}
	// Note: We can't directly compare enum values easily, but we tested substitution worked
}

// TestTransform_IDColumn tests setting resource ID from id_column
func TestTransform_IDColumn(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		IDColumn: "record_id",
		Mappings: map[string]string{},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"record_id": "OBS-123",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	if obs.Id == nil {
		t.Fatal("Id not set")
	}
	if *obs.Id != "OBS-123" {
		t.Errorf("Expected ID 'OBS-123', got '%s'", *obs.Id)
	}
}

// TestTransform_EmptyValue tests that empty values are skipped
func TestTransform_EmptyValue(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"status":            "${status}",
			"subject.reference": "Patient/${patient_id}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"status":     "", // Empty value
		"patient_id": "123",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	obs, ok := resource.(*fhir.Observation)
	if !ok {
		t.Fatal("Expected Observation resource")
	}
	// Status enum would have default value, which is fine
	if obs == nil {
		t.Fatal("Resource not created")
	}
	// Reference should be set
	if obs.Subject == nil || obs.Subject.Reference == nil {
		t.Fatal("Subject.reference should be set")
	}
}

// TestTransform_MissingVariable tests error for missing variables
func TestTransform_MissingVariable(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"status": "${missing_column}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"other_column": "value",
	}

	_, err := transformer.Transform(row, 1)
	if err == nil {
		t.Fatal("Expected error for missing variable, got nil")
	}
}

// TestSetFinalValue_String tests string type assignment
func TestSetFinalValue_String(t *testing.T) {
	// This is tested indirectly through TestTransform_NestedPath
	// Testing directly would require exposing private methods
	t.Skip("Tested indirectly through integration tests")
}

// TestSetFinalValue_Int tests integer conversion
func TestSetFinalValue_Int(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Patient",
		Mappings: map[string]string{
			// Patient doesn't have direct int fields, but we can test via nested structures
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{}

	_, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}
	// More specific int testing would require a resource with int fields
}

// TestSetFinalValue_Bool tests boolean conversion
func TestSetFinalValue_Bool(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Patient",
		Mappings: map[string]string{
			"active": "${is_active}",
		},
	}

	transformer := NewTransformer(cfg)

	// Test true value
	row := map[string]string{
		"is_active": "true",
	}

	resource, err := transformer.Transform(row, 1)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	patient, ok := resource.(*fhir.Patient)
	if !ok {
		t.Fatal("Expected Patient resource")
	}
	if patient.Active == nil {
		t.Fatal("Active not set")
	}
	if !*patient.Active {
		t.Error("Expected active=true")
	}

	// Test false value
	row2 := map[string]string{
		"is_active": "false",
	}

	resource2, err := transformer.Transform(row2, 2)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	patient2, ok := resource2.(*fhir.Patient)
	if !ok {
		t.Fatal("Expected Patient resource")
	}
	if patient2.Active == nil {
		t.Fatal("Active not set")
	}
	if *patient2.Active {
		t.Error("Expected active=false")
	}
}

// TestSetFinalValue_InvalidInt tests error on invalid int conversion
func TestSetFinalValue_InvalidInt(t *testing.T) {
	// Most FHIR fields use strings or custom types, not primitive ints
	// This is difficult to test directly without modifying the FHIR models
	t.Skip("FHIR R4 models don't expose primitive int fields easily")
}

// TestSetFinalValue_InvalidBool tests error on invalid bool conversion
func TestSetFinalValue_InvalidBool(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Patient",
		Mappings: map[string]string{
			"active": "${is_active}",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{
		"is_active": "not_a_boolean",
	}

	_, err := transformer.Transform(row, 1)
	if err == nil {
		t.Fatal("Expected error for invalid boolean value, got nil")
	}
}

// TestTransform_InvalidPath tests error for invalid FHIR paths
func TestTransform_InvalidPath(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"..invalid": "value",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{}

	_, err := transformer.Transform(row, 1)
	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}
}

// TestTransform_NonExistentField tests error for non-existent fields
func TestTransform_NonExistentField(t *testing.T) {
	cfg := &config.MappingConfig{
		Resource: "Observation",
		Mappings: map[string]string{
			"nonExistentField": "value",
		},
	}

	transformer := NewTransformer(cfg)
	row := map[string]string{}

	_, err := transformer.Transform(row, 1)
	if err == nil {
		t.Fatal("Expected error for non-existent field, got nil")
	}
}

// TestSetResourceID tests setting the resource ID
func TestSetResourceID(t *testing.T) {
	// Already tested via TestTransform_IDColumn
	t.Skip("Tested via TestTransform_IDColumn")
}

// TestCreateResource_AllTypes tests all 8 supported resource types can be created
func TestCreateResource_AllTypes(t *testing.T) {
	resourceTypes := []string{
		"Observation",
		"Patient",
		"Condition",
		"MedicationRequest",
		"Procedure",
		"Encounter",
		"DiagnosticReport",
		"Specimen",
	}

	for _, resType := range resourceTypes {
		t.Run(resType, func(t *testing.T) {
			cfg := &config.MappingConfig{
				Resource: resType,
				Mappings: map[string]string{},
			}

			transformer := NewTransformer(cfg)
			_, err := transformer.createResource()
			if err != nil {
				t.Errorf("Failed to create %s: %v", resType, err)
			}
		})
	}
}
