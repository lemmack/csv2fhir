package transform

import (
	"reflect"
	"strings"

	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// ResourceRegistry maps resource names to their reflection types
var ResourceRegistry = map[string]reflect.Type{
	// Administrative
	"Patient":          reflect.TypeOf(fhir.Patient{}),
	"Practitioner":     reflect.TypeOf(fhir.Practitioner{}),
	"Organization":     reflect.TypeOf(fhir.Organization{}),
	"Location":         reflect.TypeOf(fhir.Location{}),
	"Encounter":        reflect.TypeOf(fhir.Encounter{}),
	"Appointment":      reflect.TypeOf(fhir.Appointment{}),
	"Schedule":         reflect.TypeOf(fhir.Schedule{}),
	"Slot":             reflect.TypeOf(fhir.Slot{}),
	"Task":             reflect.TypeOf(fhir.Task{}),
	
	// Clinical
	"Observation":      reflect.TypeOf(fhir.Observation{}),
	"Condition":        reflect.TypeOf(fhir.Condition{}),
	"Procedure":        reflect.TypeOf(fhir.Procedure{}),
	"AllergyIntolerance": reflect.TypeOf(fhir.AllergyIntolerance{}),
	"CarePlan":         reflect.TypeOf(fhir.CarePlan{}),
	"Goal":             reflect.TypeOf(fhir.Goal{}),
	"RiskAssessment":   reflect.TypeOf(fhir.RiskAssessment{}),
	"ServiceRequest":   reflect.TypeOf(fhir.ServiceRequest{}),
	
	// Medications
	"Medication":             reflect.TypeOf(fhir.Medication{}),
	"MedicationRequest":      reflect.TypeOf(fhir.MedicationRequest{}),
	"MedicationStatement":    reflect.TypeOf(fhir.MedicationStatement{}),
	"MedicationDispense":     reflect.TypeOf(fhir.MedicationDispense{}),
	"MedicationAdministration": reflect.TypeOf(fhir.MedicationAdministration{}),
	"Immunization":           reflect.TypeOf(fhir.Immunization{}),

	// Diagnostics
	"DiagnosticReport": reflect.TypeOf(fhir.DiagnosticReport{}),
	"Specimen":         reflect.TypeOf(fhir.Specimen{}),
	"ImagingStudy":     reflect.TypeOf(fhir.ImagingStudy{}),
	"Media":            reflect.TypeOf(fhir.Media{}),

	// Financial
	"Claim":            reflect.TypeOf(fhir.Claim{}),
	"ClaimResponse":    reflect.TypeOf(fhir.ClaimResponse{}),
	"Coverage":         reflect.TypeOf(fhir.Coverage{}),
    "ExplanationOfBenefit": reflect.TypeOf(fhir.ExplanationOfBenefit{}),

	// Foundation
	"StructureDefinition": reflect.TypeOf(fhir.StructureDefinition{}),
	"ValueSet":           reflect.TypeOf(fhir.ValueSet{}),
	"CodeSystem":         reflect.TypeOf(fhir.CodeSystem{}),
}

// GetResourceType looks up a resource type by name (case-insensitive)
func GetResourceType(name string) (reflect.Type, bool) {
	// Try exact match first
	if t, ok := ResourceRegistry[name]; ok {
		return t, true
	}

	// Try case-insensitive match
	target := strings.ToLower(name)
	for k, v := range ResourceRegistry {
		if strings.ToLower(k) == target {
			return v, true
		}
	}
	
	return nil, false
}
