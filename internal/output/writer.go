package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// Format represents the output format type
type Format string

const (
	FormatBundle Format = "bundle"
	FormatNDJSON Format = "ndjson"
)

// Writer handles writing FHIR resources to output
type Writer struct {
	writer       io.Writer
	format       Format
	file         *os.File
	resources    []interface{}
	firstWrite   bool
	maxResources int
	closed       bool
	warnedLimit  bool
}

// NewWriter creates a new output writer
func NewWriter(outputPath string, format Format) (*Writer, error) {
	return NewWriterWithLimit(outputPath, format, 10000) // Default 10k resources
}

// NewWriterWithLimit creates a new output writer with a configurable resource limit
func NewWriterWithLimit(outputPath string, format Format, maxResources int) (*Writer, error) {
	var writer io.Writer
	var file *os.File
	var err error

	if outputPath == "" || outputPath == "-" {
		writer = os.Stdout
	} else {
		file, err = os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create output file: %w", err)
		}
		writer = file
	}

	// Validate max resources
	if maxResources <= 0 {
		maxResources = 10000 // Sensible default
	}

	return &Writer{
		writer:       writer,
		format:       format,
		file:         file,
		resources:    []interface{}{},
		firstWrite:   true,
		maxResources: maxResources,
		closed:       false,
		warnedLimit:  false,
	}, nil
}

// Write writes a FHIR resource to the output
func (w *Writer) Write(resource interface{}) error {
	if w.format == FormatNDJSON {
		// Write immediately as newline-delimited JSON
		data, err := json.Marshal(resource)
		if err != nil {
			return fmt.Errorf("failed to marshal resource: %w", err)
		}
		if _, err := w.writer.Write(data); err != nil {
			return fmt.Errorf("failed to write resource: %w", err)
		}
		if _, err := w.writer.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	} else {
		// Check memory limit before collecting resources for bundle
		currentCount := len(w.resources)

		// Warn when approaching limit (at 90%)
		if !w.warnedLimit && currentCount >= int(float64(w.maxResources)*0.9) {
			fmt.Fprintf(os.Stderr, "Warning: Approaching memory limit (%d/%d resources). Consider using NDJSON format for large files.\n",
				currentCount, w.maxResources)
			w.warnedLimit = true
		}

		// Auto-switch to streaming if limit exceeded
		if currentCount >= w.maxResources {
			return fmt.Errorf("resource limit exceeded (%d resources). Use --format ndjson for large files or increase --max-resources", w.maxResources)
		}

		// Collect resources for bundle
		w.resources = append(w.resources, resource)
	}

	return nil
}

// Close finalizes the output (creates bundle if needed) and closes the file
func (w *Writer) Close() error {
	// Prevent double-close
	if w.closed {
		return nil
	}
	w.closed = true

	// Ensure file is closed even if bundle writing fails
	var bundleErr error
	if w.format == FormatBundle && len(w.resources) > 0 {
		bundleErr = w.writeBundle()
	}

	// Always attempt to close the file
	var closeErr error
	if w.file != nil {
		closeErr = w.file.Close()
		w.file = nil // Prevent future close attempts
	}

	// Return the first error encountered
	if bundleErr != nil {
		return fmt.Errorf("failed to write bundle: %w", bundleErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file: %w", closeErr)
	}

	return nil
}

// writeBundle creates and writes a FHIR Bundle containing all resources
func (w *Writer) writeBundle() error {
	bundle := &fhir.Bundle{
		Type: fhir.BundleTypeCollection,
	}

	// Create bundle entries
	entries := make([]fhir.BundleEntry, 0, len(w.resources))
	for _, resource := range w.resources {
		// Marshal resource to JSON for BundleEntry
		resourceJSON, err := json.Marshal(resource)
		if err != nil {
			return fmt.Errorf("failed to marshal resource: %w", err)
		}

		entry := fhir.BundleEntry{
			Resource: resourceJSON,
		}
		entries = append(entries, entry)
	}

	bundle.Entry = entries

	// Set bundle metadata
	total := len(entries)
	bundle.Total = &total

	// Marshal and write bundle
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bundle: %w", err)
	}

	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write bundle: %w", err)
	}

	return nil
}

// ParseFormat parses a format string into a Format type
func ParseFormat(s string) (Format, error) {
	switch s {
	case "bundle", "":
		return FormatBundle, nil
	case "ndjson":
		return FormatNDJSON, nil
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: bundle, ndjson)", s)
	}
}
