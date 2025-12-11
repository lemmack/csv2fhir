package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// Reader wraps csv.Reader and provides streaming row-by-row access
type Reader struct {
	file      *os.File
	csvReader *csv.Reader
	headers   []string
	rowNumber int
}

// Row represents a CSV row as a map of column name to value
type Row struct {
	Data      map[string]string
	RowNumber int
}

// NewReader creates a new CSV reader
func NewReader(path string, delimiter rune) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}

	csvReader := csv.NewReader(file)
	csvReader.Comma = delimiter
	csvReader.TrimLeadingSpace = true
	csvReader.ReuseRecord = true // Memory optimization for large files

	// Read header row
	headers, err := csvReader.Read()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Create a copy of headers since ReuseRecord is enabled
	headersCopy := make([]string, len(headers))
	copy(headersCopy, headers)

	return &Reader{
		file:      file,
		csvReader: csvReader,
		headers:   headersCopy,
		rowNumber: 1, // Row 1 is the header, data starts at row 2
	}, nil
}

// Headers returns the CSV column headers
func (r *Reader) Headers() []string {
	return r.headers
}

// Read reads the next row from the CSV file
func (r *Reader) Read() (*Row, error) {
	record, err := r.csvReader.Read()
	if err != nil {
		return nil, err
	}

	r.rowNumber++

	// Convert to map
	// CRITICAL: Copy values since ReuseRecord=true means record slice is reused
	rowData := make(map[string]string, len(r.headers))
	for i, header := range r.headers {
		if i < len(record) {
			// Create a copy of the string value to avoid shared references
			rowData[header] = string([]byte(record[i]))
		} else {
			rowData[header] = "" // Handle missing columns
		}
	}

	return &Row{
		Data:      rowData,
		RowNumber: r.rowNumber,
	}, nil
}

// ReadAll reads all rows from the CSV file (use with caution on large files)
func (r *Reader) ReadAll() ([]*Row, error) {
	rows := []*Row{}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// Close closes the underlying file
func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
