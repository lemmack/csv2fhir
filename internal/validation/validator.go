package validation

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidationError represents a single validation error or warning
type ValidationError struct {
	Field    string // FHIR path (e.g., "status", "code.coding[0].system")
	Message  string // Error description
	Severity string // "error" or "warning"
}

// Validator validates FHIR resources
type Validator interface {
	Validate(resource interface{}) []ValidationError
}

// CompositeValidator combines multiple validators
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a new composite validator
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

// Validate runs all validators and collects errors
func (c *CompositeValidator) Validate(resource interface{}) []ValidationError {
	var errors []ValidationError
	for _, validator := range c.validators {
		errors = append(errors, validator.Validate(resource)...)
	}
	return errors
}

// getResourceType extracts the resource type from a FHIR resource
func getResourceType(resource interface{}) string {
	t := reflect.TypeOf(resource)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// getFieldValue gets a field value from a resource using reflection
func getFieldValue(resource interface{}, fieldName string) (interface{}, bool) {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Capitalize first letter for Go struct field
	fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
	field := v.FieldByName(fieldName)

	if !field.IsValid() {
		return nil, false
	}

	return field.Interface(), true
}

// isFieldEmpty checks if a field is empty (nil pointer or zero value)
func isFieldEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)

	switch v.Kind() {
	case reflect.Ptr:
		return v.IsNil()
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Array:
		return v.Len() == 0
	case reflect.Struct:
		// For structs, check if all fields are zero values
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).IsZero() {
				return false
			}
		}
		return true
	default:
		return v.IsZero()
	}
}

// CreateError creates a validation error
func CreateError(field, message string) ValidationError {
	return ValidationError{
		Field:    field,
		Message:  message,
		Severity: "error",
	}
}

// CreateWarning creates a validation warning
func CreateWarning(field, message string) ValidationError {
	return ValidationError{
		Field:    field,
		Message:  message,
		Severity: "warning",
	}
}

// FormatErrors formats validation errors for display
func FormatErrors(errors []ValidationError, rowNumber int) string {
	if len(errors) == 0 {
		return ""
	}

	var lines []string
	for _, err := range errors {
		lines = append(lines, fmt.Sprintf("Row %d: Validation %s in field '%s': %s",
			rowNumber, err.Severity, err.Field, err.Message))
	}
	return strings.Join(lines, "\n")
}
