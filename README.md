# csv2fhir

A CLI tool that converts CSV files to FHIR R4 JSON format using user-provided YAML mapping files.

## Features

- Streaming CSV processing for handling arbitrarily large files efficiently
- Flexible YAML-based mapping configuration
- Support for nested FHIR resource structures
- Multiple output formats (Bundle or NDJSON)
- Variable substitution from CSV columns
- Default value support
- FHIR R4 compliant using [golang-fhir-models](https://github.com/samply/golang-fhir-models)

## Installation

```bash
go install
```

Or build from source:

```bash
go build -o csv2fhir
```

## Usage

```bash
csv2fhir --input data.csv --mapping mapping.yaml --output output.json
csv2fhir --input data.csv --mapping mapping.yaml --output output.ndjson --format ndjson
```

### Command Line Flags

- `--input`, `-i`: Input CSV file path (required)
- `--mapping`, `-m`: YAML mapping file path (required)
- `--output`, `-o`: Output file path (default: stdout)
- `--format`, `-f`: Output format, either `bundle` or `ndjson` (default: bundle)
- `--delimiter`, `-d`: CSV delimiter (default: comma)

## YAML Mapping Format

The YAML mapping file defines how CSV columns map to FHIR resource fields.

### Basic Structure

```yaml
resource: Observation          # FHIR resource type (required)
id_column: record_id           # CSV column to use for resource ID (optional)

mappings:                      # Field mappings (required)
  subject.reference: "Patient/${patient_id}"
  code.coding[0].system: "http://loinc.org"
  code.coding[0].code: "${loinc_code}"
  code.coding[0].display: "${test_name}"
  valueQuantity.value: "${result_value}"
  valueQuantity.unit: "${unit}"
  effectiveDateTime: "${observation_date}"

defaults:                      # Default values (optional)
  status: "final"
```

### Mapping Syntax

#### Variable Substitution

Use `${column_name}` to substitute values from CSV columns:

```yaml
mappings:
  subject.reference: "Patient/${patient_id}"  # Static text + variable
  code.coding[0].code: "${loinc_code}"        # Variable only
```

#### Nested Paths

Use dot notation for nested structures:

```yaml
mappings:
  subject.reference: "Patient/${patient_id}"      # subject.reference
  code.text: "${test_name}"                       # code.text
```

#### Array Indices

Use bracket notation for array elements:

```yaml
mappings:
  code.coding[0].system: "http://loinc.org"       # First coding element
  code.coding[0].code: "${loinc_code}"
  code.coding[1].system: "http://example.org"     # Second coding element
  code.coding[1].code: "${local_code}"
```

#### Defaults

Defaults are applied before mappings, so mappings override defaults:

```yaml
defaults:
  status: "final"                # Applied to all resources
  code.coding[0].system: "http://loinc.org"

mappings:
  status: "${status}"            # Overrides default if CSV has status column
```

### Supported Resource Types

Currently supported FHIR R4 resource types:

- Observation
- Patient
- Condition
- MedicationRequest
- Procedure
- Encounter
- DiagnosticReport
- Specimen

## Examples

### Example 1: Laboratory Observations

CSV file ([sample.csv](examples/sample.csv)):

```csv
record_id,patient_id,loinc_code,test_name,result_value,unit,observation_date,status
OBS001,PAT123,2339-0,Glucose,95,mg/dL,2024-01-15T10:30:00Z,final
OBS002,PAT123,2085-9,HDL Cholesterol,55,mg/dL,2024-01-15T10:30:00Z,final
```

Mapping file ([sample-mapping.yaml](examples/sample-mapping.yaml)):

```yaml
resource: Observation
id_column: record_id

mappings:
  subject.reference: "Patient/${patient_id}"
  code.coding[0].system: "http://loinc.org"
  code.coding[0].code: "${loinc_code}"
  code.coding[0].display: "${test_name}"
  valueQuantity.value: "${result_value}"
  valueQuantity.unit: "${unit}"
  effectiveDateTime: "${observation_date}"
  status: "${status}"

defaults:
  status: "final"
```

Run the conversion:

```bash
# Output as Bundle
csv2fhir -i examples/sample.csv -m examples/sample-mapping.yaml -o output.json

# Output as NDJSON
csv2fhir -i examples/sample.csv -m examples/sample-mapping.yaml -o output.ndjson -f ndjson

# Output to stdout
csv2fhir -i examples/sample.csv -m examples/sample-mapping.yaml
```

### Example 2: Patient Demographics

CSV file:

```csv
patient_id,family_name,given_name,birth_date,gender
PAT001,Smith,John,1980-05-15,male
PAT002,Doe,Jane,1992-08-22,female
```

Mapping file:

```yaml
resource: Patient
id_column: patient_id

mappings:
  name[0].family: "${family_name}"
  name[0].given[0]: "${given_name}"
  birthDate: "${birth_date}"
  gender: "${gender}"

defaults:
  active: "true"
```

## Output Formats

### Bundle Format (default)

Creates a FHIR Bundle resource containing all converted resources:

```json
{
  "resourceType": "Bundle",
  "type": "collection",
  "total": 5,
  "entry": [
    {
      "resource": {
        "resourceType": "Observation",
        "id": "OBS001",
        ...
      }
    },
    ...
  ]
}
```

### NDJSON Format

Outputs one FHIR resource per line (newline-delimited JSON):

```json
{"resourceType":"Observation","id":"OBS001",...}
{"resourceType":"Observation","id":"OBS002",...}
{"resourceType":"Observation","id":"OBS003",...}
```

This format is useful for:
- Streaming processing
- Large datasets
- FHIR Bulk Data Export compatibility

## Error Handling

The tool provides helpful error messages with row numbers when issues occur:

- Missing required CSV columns
- Invalid FHIR paths in mapping
- Type conversion errors
- Validation failures

Errors are reported to stderr, and the tool continues processing remaining rows when possible.

## Performance

The tool uses streaming CSV processing, reading one row at a time rather than loading the entire file into memory. This allows it to efficiently handle CSV files of any size.

For large files:
- Consider using NDJSON format for streaming output
- Monitor progress via stderr output (reports every 100 rows)

## Project Structure

```
csv2fhir/
├── main.go                    # CLI entry point
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
├── internal/
│   ├── config/
│   │   └── mapping.go         # YAML parsing and mapping config
│   ├── csv/
│   │   └── reader.go          # Streaming CSV reader
│   ├── transform/
│   │   └── transform.go       # CSV to FHIR transformation logic
│   └── output/
│       └── writer.go          # Bundle and NDJSON output writers
├── examples/
│   ├── sample.csv             # Example CSV data
│   └── sample-mapping.yaml    # Example mapping configuration
└── README.md                  # This file
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License
