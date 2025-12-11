package validation

import (
	"reflect"
	"regexp"
	"strings"
)

// ReferenceValidator validates FHIR reference formats
type ReferenceValidator struct {
	referenceFields map[string][]string // Resource type -> list of reference field names
}

// NewReferenceValidator creates a new reference validator
func NewReferenceValidator() *ReferenceValidator {
	return &ReferenceValidator{
		referenceFields: map[string][]string{
			"Observation":       {"subject", "performer", "basedOn"},
			"Condition":         {"subject", "asserter"},
			"MedicationRequest": {"subject", "requester"},
			"Procedure":         {"subject", "performer"},
			"Encounter":         {"subject", "participant"},
			"DiagnosticReport":  {"subject", "performer"},
			"Specimen":          {"subject", "collection"},
		},
	}
}

// Validate checks reference fields for valid format
func (v *ReferenceValidator) Validate(resource interface{}) []ValidationError {
	resourceType := getResourceType(resource)
	fields := v.referenceFields[resourceType]
	if fields == nil {
		return nil // No reference fields to validate
	}

	var errors []ValidationError
	for _, field := range fields {
		value, exists := getFieldValue(resource, field)
		if !exists || isFieldEmpty(value) {
			continue // Skip empty fields
		}

		// Extract reference string from the Reference object
		refString := extractReferenceString(value)
		if refString == "" {
			continue
		}

		if !isValidReference(refString) {
			errors = append(errors, CreateError(field+".reference",
				"Invalid reference format (expected 'ResourceType/id', '#id', or full URL)"))
		}
	}

	return errors
}

// extractReferenceString extracts the reference string from a Reference object
func extractReferenceString(value interface{}) string {
	v := reflect.ValueOf(value)

	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// If it's a struct, look for Reference field
	if v.Kind() == reflect.Struct {
		refField := v.FieldByName("Reference")
		if refField.IsValid() {
			return extractStringValue(refField.Interface())
		}
	}

	// If it's a string directly
	if v.Kind() == reflect.String {
		return v.String()
	}

	return ""
}

// isValidReference validates FHIR reference formats
func isValidReference(ref string) bool {
	if ref == "" {
		return false
	}

	// Valid formats:
	// 1. ResourceType/id (e.g., "Patient/123")
	// 2. #id (internal reference)
	// 3. Full URL (http://example.org/fhir/Patient/123)

	// Internal reference (starts with #)
	if strings.HasPrefix(ref, "#") {
		return len(ref) > 1 // Must have an ID after #
	}

	// Full URL
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "urn:") {
		return true // Accept all URLs (more specific validation could be added)
	}

	// Relative reference (ResourceType/id)
	// Must match pattern: CapitalizedWord/something
	// Resource type must start with uppercase letter
	matched, _ := regexp.MatchString(`^[A-Z][a-zA-Z]+/[^/\s]+$`, ref)
	return matched
}
