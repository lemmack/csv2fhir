package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"csv2fhir/internal/config"
	"csv2fhir/internal/csv"
	"csv2fhir/internal/output"
	"csv2fhir/internal/transform"
	"csv2fhir/internal/validation"
)

func main() {
	// Define CLI flags
	inputFile := flag.String("input", "", "Input CSV file path (required)")
	inputFileShort := flag.String("i", "", "Input CSV file path (short)")
	mappingFile := flag.String("mapping", "", "YAML mapping file path (required)")
	mappingFileShort := flag.String("m", "", "YAML mapping file path (short)")
	outputFile := flag.String("output", "", "Output file path (default: stdout)")
	outputFileShort := flag.String("o", "", "Output file path (short)")
	formatStr := flag.String("format", "bundle", "Output format: bundle or ndjson")
	formatStrShort := flag.String("f", "", "Output format (short)")
	delimiter := flag.String("delimiter", ",", "CSV delimiter")
	delimiterShort := flag.String("d", "", "CSV delimiter (short)")
	maxResources := flag.Int("max-resources", 10000, "Maximum resources in memory for bundle format (default: 10000)")
	validate := flag.Bool("validate", false, "Enable FHIR validation")
	validationLevel := flag.String("validation-level", "error", "Validation level: error (fail on errors) or warn (log warnings)")

	flag.Parse()

	// Handle short flags
	if *inputFileShort != "" {
		inputFile = inputFileShort
	}
	if *mappingFileShort != "" {
		mappingFile = mappingFileShort
	}
	if *outputFileShort != "" {
		outputFile = outputFileShort
	}
	if *formatStrShort != "" {
		formatStr = formatStrShort
	}
	if *delimiterShort != "" {
		delimiter = delimiterShort
	}

	// Validate required flags
	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --input/-i flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *mappingFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --mapping/-m flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Parse format
	format, err := output.ParseFormat(*formatStr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Get delimiter rune
	var delimiterRune rune
	if len(*delimiter) > 0 {
		delimiterRune = rune((*delimiter)[0])
	} else {
		delimiterRune = ','
	}

	// Run the conversion
	if err := run(*inputFile, *mappingFile, *outputFile, format, delimiterRune, *maxResources, *validate, *validationLevel); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(inputPath, mappingPath, outputPath string, format output.Format, delimiter rune, maxResources int, enableValidation bool, validationLevel string) error {
	// Load mapping configuration
	fmt.Fprintf(os.Stderr, "Loading mapping configuration from %s...\n", mappingPath)
	cfg, err := config.LoadMapping(mappingPath)
	if err != nil {
		return fmt.Errorf("failed to load mapping: %w", err)
	}

	// Open CSV file
	fmt.Fprintf(os.Stderr, "Opening CSV file %s...\n", inputPath)
	csvReader, err := csv.NewReader(inputPath, delimiter)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer csvReader.Close()

	// Validate CSV columns against mapping
	cfg.SetCSVColumns(csvReader.Headers())
	if err := cfg.ValidateColumns(); err != nil {
		return fmt.Errorf("mapping validation failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "CSV headers: %v\n", csvReader.Headers())
	fmt.Fprintf(os.Stderr, "Resource type: %s\n", cfg.Resource)
	fmt.Fprintf(os.Stderr, "Output format: %s\n", format)

	// Create transformer with optional validation
	var transformer *transform.Transformer
	if enableValidation {
		fmt.Fprintf(os.Stderr, "FHIR validation enabled (level: %s)\n", validationLevel)
		validator := validation.NewCompositeValidator(
			validation.NewRequiredFieldsValidator(),
			validation.NewDateTimeValidator(),
			validation.NewReferenceValidator(),
		)
		transformer = transform.NewTransformerWithValidator(cfg, validator)
	} else {
		transformer = transform.NewTransformer(cfg)
	}

	// Create output writer with memory limit
	writer, err := output.NewWriterWithLimit(outputPath, format, maxResources)
	if err != nil {
		return fmt.Errorf("failed to create output writer: %w", err)
	}
	defer writer.Close()

	// Process CSV rows
	fmt.Fprintf(os.Stderr, "Processing CSV rows...\n")
	rowCount := 0
	errorCount := 0
	validationErrorCount := 0

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV row: %w", err)
		}

		// Transform row to FHIR resource (with or without validation)
		var resource interface{}
		var validationErrors []validation.ValidationError

		if enableValidation {
			resource, validationErrors, err = transformer.TransformWithValidation(row.Data, row.RowNumber)
		} else {
			resource, err = transformer.Transform(row.Data, row.RowNumber)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			errorCount++
			continue
		}

		// Handle validation errors
		if len(validationErrors) > 0 {
			validationErrorCount++
			formatted := validation.FormatErrors(validationErrors, row.RowNumber)

			if validationLevel == "error" {
				// Fail on validation errors
				fmt.Fprintf(os.Stderr, "%s\n", formatted)
				errorCount++
				continue
			} else {
				// Log as warnings and continue
				fmt.Fprintf(os.Stderr, "%s\n", formatted)
			}
		}

		// Write resource to output
		if err := writer.Write(resource); err != nil {
			return fmt.Errorf("failed to write resource: %w", err)
		}

		rowCount++
		if rowCount%100 == 0 {
			fmt.Fprintf(os.Stderr, "Processed %d rows...\n", rowCount)
		}
	}

	fmt.Fprintf(os.Stderr, "Completed! Processed %d rows (%d errors", rowCount, errorCount)
	if enableValidation {
		fmt.Fprintf(os.Stderr, ", %d validation issues)\n", validationErrorCount)
	} else {
		fmt.Fprintf(os.Stderr, ")\n")
	}

	return nil
}
