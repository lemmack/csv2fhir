package validation

// RequiredFieldsValidator validates that required fields are present for each resource type
type RequiredFieldsValidator struct{}

// NewRequiredFieldsValidator creates a new required fields validator
func NewRequiredFieldsValidator() *RequiredFieldsValidator {
	return &RequiredFieldsValidator{}
}

// Validate checks if required fields are present
func (v *RequiredFieldsValidator) Validate(resource interface{}) []ValidationError {
	resourceType := getResourceType(resource)
	requiredFields := v.getRequiredFields(resourceType)

	var errors []ValidationError
	for _, field := range requiredFields {
		value, exists := getFieldValue(resource, field)
		if !exists || isFieldEmpty(value) {
			errors = append(errors, CreateError(field, "Required field is missing or empty"))
		}
	}

	return errors
}

// getRequiredFields returns the list of required fields for a given resource type
func (v *RequiredFieldsValidator) getRequiredFields(resourceType string) []string {
	switch resourceType {
	case "Observation":
		return []string{"status", "code"}

	case "Patient":
		// Patient has no universally required fields in FHIR R4
		// Typically requires either identifier or name, but not strictly enforced
		return []string{}

	case "Condition":
		return []string{"subject"}

	case "MedicationRequest":
		return []string{"status", "intent", "subject"}
		// Note: medication[x] is also required but checking choice types is complex

	case "Procedure":
		return []string{"status", "subject"}

	case "Encounter":
		return []string{"status", "class"}

	case "DiagnosticReport":
		return []string{"status", "code"}

	case "Specimen":
		// Specimen has no universally required fields in FHIR R4
		return []string{}

	default:
		return []string{}
	}
}
