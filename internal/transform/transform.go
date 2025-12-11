package transform

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"csv2fhir/internal/config"
	"csv2fhir/internal/validation"
)

// Transformer handles CSV to FHIR transformation
type Transformer struct {
	config    *config.MappingConfig
	validator validation.Validator
}

// NewTransformer creates a new transformer with the given mapping config
func NewTransformer(cfg *config.MappingConfig) *Transformer {
	return &Transformer{
		config:    cfg,
		validator: nil, // Validation is optional
	}
}

// NewTransformerWithValidator creates a new transformer with validation enabled
func NewTransformerWithValidator(cfg *config.MappingConfig, validator validation.Validator) *Transformer {
	return &Transformer{
		config:    cfg,
		validator: validator,
	}
}

// Transform converts a CSV row to a FHIR resource
func (t *Transformer) Transform(row map[string]string, rowNumber int) (interface{}, error) {
	// Create the appropriate FHIR resource based on config
	resource, err := t.createResource()
	if err != nil {
		return nil, fmt.Errorf("row %d: failed to create resource: %w", rowNumber, err)
	}

	// Apply defaults first
	for path, value := range t.config.Defaults {
		substituted, err := config.SubstituteVariables(value, row)
		if err != nil {
			// For defaults, log warning but continue (defaults might be literal values)
			// Only warn if the original value contained variables
			if strings.Contains(value, "${") {
				return nil, fmt.Errorf("row %d: failed to substitute variables in default %s: %w", rowNumber, path, err)
			}
		}
		if err := t.setFieldValue(resource, path, substituted); err != nil {
			return nil, fmt.Errorf("row %d: failed to set default %s: %w", rowNumber, path, err)
		}
	}

	// Apply mappings (these override defaults)
	for path, value := range t.config.Mappings {
		substituted, err := config.SubstituteVariables(value, row)
		if err != nil {
			return nil, fmt.Errorf("row %d: failed to substitute variables in mapping %s: %w", rowNumber, path, err)
		}
		// Skip empty values
		if substituted == "" {
			continue
		}
		if err := t.setFieldValue(resource, path, substituted); err != nil {
			return nil, fmt.Errorf("row %d: failed to set mapping %s: %w", rowNumber, path, err)
		}
	}

	// Set resource ID if specified
	if t.config.IDColumn != "" {
		if id, ok := row[t.config.IDColumn]; ok && id != "" {
			if err := t.setResourceID(resource, id); err != nil {
				return nil, fmt.Errorf("row %d: failed to set resource ID: %w", rowNumber, err)
			}
		}
	}

	return resource, nil
}

// TransformWithValidation converts a CSV row to a FHIR resource and validates it
func (t *Transformer) TransformWithValidation(row map[string]string, rowNumber int) (interface{}, []validation.ValidationError, error) {
	// Transform the resource
	resource, err := t.Transform(row, rowNumber)
	if err != nil {
		return nil, nil, err
	}

	// Validate if validator is set
	var validationErrors []validation.ValidationError
	if t.validator != nil {
		validationErrors = t.validator.Validate(resource)
	}

	return resource, validationErrors, nil
}

// createResource creates a new FHIR resource of the configured type
func (t *Transformer) createResource() (interface{}, error) {
	resourceType, ok := GetResourceType(t.config.Resource)
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", t.config.Resource)
	}

	// Create a new instance of the resource type
	return reflect.New(resourceType).Interface(), nil
}

// setResourceID sets the ID field of a FHIR resource
func (t *Transformer) setResourceID(resource interface{}, id string) error {
	v := reflect.ValueOf(resource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	idField := v.FieldByName("Id")
	if !idField.IsValid() {
		return fmt.Errorf("resource has no Id field")
	}

	if idField.Kind() == reflect.Ptr {
		idField.Set(reflect.ValueOf(&id))
	} else {
		idField.SetString(id)
	}

	return nil
}

// setFieldValue sets a value at the given FHIR path
func (t *Transformer) setFieldValue(resource interface{}, path string, value string) error {
	segments, err := config.ParsePath(path)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(resource)
	return t.setNestedFieldValue(v, segments, value)
}

// setNestedFieldValue recursively sets a nested field value using reflect.Value
func (t *Transformer) setNestedFieldValue(v reflect.Value, segments []config.PathSegment, value string) error {
	if len(segments) == 0 {
		return fmt.Errorf("empty path")
	}

	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return fmt.Errorf("nil pointer encountered")
		}
		v = v.Elem()
	}

	segment := segments[0]

	// Validate field name is not empty
	if len(segment.Field) == 0 {
		return fmt.Errorf("empty field name in path")
	}

	// Capitalize first letter for Go struct field
	fieldName := strings.ToUpper(segment.Field[:1]) + segment.Field[1:]
	field := v.FieldByName(fieldName)

	if !field.IsValid() {
		return fmt.Errorf("field %s not found in %s", fieldName, v.Type().Name())
	}

	// Handle array index if present
	if segment.Index != nil {
		if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
			return fmt.Errorf("field %s is not a slice/array", fieldName)
		}

		// Ensure slice is large enough
		index := *segment.Index
		if field.Len() <= index {
			// Grow slice
			newSlice := reflect.MakeSlice(field.Type(), index+1, index+1)
			reflect.Copy(newSlice, field)
			field.Set(newSlice)
		}

		elem := field.Index(index)

		// If element is nil pointer, create new instance
		if elem.Kind() == reflect.Ptr && elem.IsNil() {
			newElem := reflect.New(elem.Type().Elem())
			elem.Set(newElem)
		}

		if len(segments) == 1 {
			return t.setFinalValue(elem, value)
		}
		return t.setNestedFieldValue(elem, segments[1:], value)
	}

	// If this is the last segment, set the value
	if len(segments) == 1 {
		return t.setFinalValue(field, value)
	}

	// Handle pointer fields
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			// Create new instance
			newVal := reflect.New(field.Type().Elem())
			field.Set(newVal)
		}
		return t.setNestedFieldValue(field, segments[1:], value)
	}

	// Handle struct fields
	if field.Kind() == reflect.Struct {
		if !field.CanAddr() {
			return fmt.Errorf("cannot address struct field %s", fieldName)
		}
		return t.setNestedFieldValue(field.Addr(), segments[1:], value)
	}

	return fmt.Errorf("cannot navigate through field %s of type %s", fieldName, field.Kind())
}

// setFinalValue sets the actual value on a field
func (t *Transformer) setFinalValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			newVal := reflect.New(field.Type().Elem())
			field.Set(newVal)
		}
		return t.setFinalValue(field.Elem(), value)
	}

	// Try JSON unmarshaling first for types that implement json.Unmarshaler
	// This handles FHIR enum types and other custom types
	if field.CanAddr() {
		unmarshaler := field.Addr().Interface()
		if _, ok := unmarshaler.(json.Unmarshaler); ok {
			// Use json.Marshal to properly escape the value
			jsonValue, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal value for JSON unmarshaling: %w", err)
			}
			if err := json.Unmarshal(jsonValue, unmarshaler); err == nil {
				return nil
			}
		}
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
		return nil

	case reflect.Int, reflect.Int32, reflect.Int64:
		// Try parsing as int directly first
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot convert %s to int: %w", value, err)
		}
		field.SetInt(intVal)
		return nil

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("cannot convert %s to float: %w", value, err)
		}
		field.SetFloat(floatVal)
		return nil

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("cannot convert %s to bool: %w", value, err)
		}
		field.SetBool(boolVal)
		return nil

	case reflect.Struct:
		// Try to unmarshal as JSON for complex types
		// Use json.Marshal to properly escape special characters
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		if err := json.Unmarshal(jsonValue, field.Addr().Interface()); err != nil {
			return fmt.Errorf("cannot set struct field with value %s: %w", value, err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}
