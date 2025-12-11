package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

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

	// Initialize counters
	rowCount := 0
	errorCount := 0
	validationErrorCount := 0

	// Create channels for parallel processing
	type job struct {
		data      map[string]string
		rowNumber int
	}

	type result struct {
		resource         interface{}
		validationErrors []validation.ValidationError
		err              error
		rowNumber        int
	}

	// Worker configuration
	numWorkers := 4
	jobs := make(chan job, numWorkers*4)
	results := make(chan result, numWorkers*4)

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var res result
				res.rowNumber = j.rowNumber

				if enableValidation {
					res.resource, res.validationErrors, res.err = transformer.TransformWithValidation(j.data, j.rowNumber)
				} else {
					res.resource, res.err = transformer.Transform(j.data, j.rowNumber)
				}
				results <- res
			}
		}()
	}

	// Closer goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Start writer goroutine (Consumer)
	done := make(chan bool)
	go func() {
		for res := range results {
			// Note: Parallel processing might reorder rows.

			if res.err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", res.err)
				errorCount++
				continue
			}

			// Handle validation errors
			if len(res.validationErrors) > 0 {
				validationErrorCount++
				formatted := validation.FormatErrors(res.validationErrors, res.rowNumber)

				if validationLevel == "error" {
					fmt.Fprintf(os.Stderr, "%s\n", formatted)
					errorCount++
					continue
				} else {
					fmt.Fprintf(os.Stderr, "%s\n", formatted)
				}
			}

			// Write resource to output
			if err := writer.Write(res.resource); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing resource: %v\n", err)
				errorCount++
			}

			rowCount++
			if rowCount%100 == 0 {
				fmt.Fprintf(os.Stderr, "Processed %d rows...\n", rowCount)
			}
		}
		done <- true
	}()

	// Feed the workers (Producer)
	fmt.Fprintf(os.Stderr, "Processing CSV rows (using %d workers)...\n", numWorkers)

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV row: %w", err)
		}

		jobs <- job{data: row.Data, rowNumber: row.RowNumber}
	}
	close(jobs)

	// Wait for writer to finish
	<-done

	fmt.Fprintf(os.Stderr, "Completed! Processed %d rows (%d errors", rowCount, errorCount)
	if enableValidation {
		fmt.Fprintf(os.Stderr, ", %d validation issues)\n", validationErrorCount)
	} else {
		fmt.Fprintf(os.Stderr, ")\n")
	}

	return nil
}
