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
	writer     io.Writer
	format     Format
	file       *os.File
	resources  []interface{}
	firstWrite bool
}

// NewWriter creates a new output writer
func NewWriter(outputPath string, format Format) (*Writer, error) {
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

	return &Writer{
		writer:     writer,
		format:     format,
		file:       file,
		resources:  []interface{}{},
		firstWrite: true,
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
		// Collect resources for bundle
		w.resources = append(w.resources, resource)
	}

	return nil
}

// Close finalizes the output (creates bundle if needed) and closes the file
func (w *Writer) Close() error {
	if w.format == FormatBundle && len(w.resources) > 0 {
		if err := w.writeBundle(); err != nil {
			if w.file != nil {
				w.file.Close()
			}
			return err
		}
	}

	if w.file != nil {
		return w.file.Close()
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
