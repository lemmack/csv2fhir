package validation

import (
	"reflect"
	"regexp"
	"time"
)

// DateTimeValidator validates ISO 8601 datetime formats
type DateTimeValidator struct {
	dateTimeFields map[string][]string // Resource type -> list of datetime field names
}

// NewDateTimeValidator creates a new datetime validator
func NewDateTimeValidator() *DateTimeValidator {
	return &DateTimeValidator{
		dateTimeFields: map[string][]string{
			"Observation":       {"effectiveDateTime", "issued"},
			"Patient":           {"birthDate", "deceasedDateTime"},
			"Condition":         {"onsetDateTime", "abatementDateTime", "recordedDate"},
			"MedicationRequest": {"authoredOn"},
			"Procedure":         {"performedDateTime"},
			"Encounter":         {"period"},
			"DiagnosticReport":  {"effectiveDateTime", "issued"},
			"Specimen":          {"receivedTime"},
		},
	}
}

// Validate checks datetime fields for valid ISO 8601 format
func (v *DateTimeValidator) Validate(resource interface{}) []ValidationError {
	resourceType := getResourceType(resource)
	fields := v.dateTimeFields[resourceType]
	if fields == nil {
		return nil // No datetime fields to validate
	}

	var errors []ValidationError
	for _, field := range fields {
		value, exists := getFieldValue(resource, field)
		if !exists || isFieldEmpty(value) {
			continue // Skip empty fields (required field validation handles this)
		}

		// Extract string value from pointer if needed
		strValue := extractStringValue(value)
		if strValue == "" {
			continue
		}

		if !isValidDateTime(strValue) {
			errors = append(errors, CreateError(field, "Invalid ISO 8601 datetime format"))
		}
	}

	return errors
}

// extractStringValue extracts a string from a value (handling pointers)
func extractStringValue(value interface{}) string {
	v := reflect.ValueOf(value)

	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Get string value
	if v.Kind() == reflect.String {
		return v.String()
	}

	return ""
}

// isValidDateTime validates ISO 8601 datetime formats accepted by FHIR
func isValidDateTime(value string) bool {
	// FHIR accepts various ISO 8601 formats:
	// - YYYY (year)
	// - YYYY-MM (year-month)
	// - YYYY-MM-DD (date)
	// - YYYY-MM-DDThh:mm:ss+zz:zz (dateTime)
	// - YYYY-MM-DDThh:mm:ss.sss+zz:zz (dateTime with milliseconds)
	// - YYYY-MM-DDThh:mm:ssZ (UTC)

	// Regex patterns for each format
	patterns := []string{
		`^\d{4}$`,                                                          // YYYY
		`^\d{4}-\d{2}$`,                                                    // YYYY-MM
		`^\d{4}-\d{2}-\d{2}$`,                                              // YYYY-MM-DD
		`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`,                           // YYYY-MM-DDThh:mm:ssZ
		`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`,             // YYYY-MM-DDThh:mm:ss+zz:zz
		`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,9}Z$`,                  // YYYY-MM-DDThh:mm:ss.sssZ
		`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,9}[+-]\d{2}:\d{2}$`,    // YYYY-MM-DDThh:mm:ss.sss+zz:zz
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, value)
		if matched {
			// Additional validation: try parsing as time
			return tryParseDateTime(value)
		}
	}

	return false
}

// tryParseDateTime attempts to parse the datetime value
func tryParseDateTime(value string) bool {
	// Supported formats for parsing
	formats := []string{
		"2006",               // YYYY
		"2006-01",            // YYYY-MM
		"2006-01-02",         // YYYY-MM-DD
		time.RFC3339,         // YYYY-MM-DDThh:mm:ssZ or +zz:zz
		time.RFC3339Nano,     // YYYY-MM-DDThh:mm:ss.sssZ or +zz:zz
	}

	for _, format := range formats {
		_, err := time.Parse(format, value)
		if err == nil {
			return true
		}
	}

	return false
}
