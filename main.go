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
	if err := run(*inputFile, *mappingFile, *outputFile, format, delimiterRune); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(inputPath, mappingPath, outputPath string, format output.Format, delimiter rune) error {
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

	// Create transformer
	transformer := transform.NewTransformer(cfg)

	// Create output writer
	writer, err := output.NewWriter(outputPath, format)
	if err != nil {
		return fmt.Errorf("failed to create output writer: %w", err)
	}
	defer writer.Close()

	// Process CSV rows
	fmt.Fprintf(os.Stderr, "Processing CSV rows...\n")
	rowCount := 0
	errorCount := 0

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV row: %w", err)
		}

		// Transform row to FHIR resource
		resource, err := transformer.Transform(row.Data, row.RowNumber)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			errorCount++
			continue
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

	fmt.Fprintf(os.Stderr, "Completed! Processed %d rows (%d errors)\n", rowCount, errorCount)

	return nil
}
