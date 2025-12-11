package csv

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestNewReader tests creating a reader for a valid CSV file
func TestNewReader(t *testing.T) {
	content := `name,age,city
John,30,NYC
Jane,25,LA
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	headers := reader.Headers()
	if len(headers) != 3 {
		t.Errorf("Expected 3 headers, got %d", len(headers))
	}
	if headers[0] != "name" || headers[1] != "age" || headers[2] != "city" {
		t.Errorf("Unexpected headers: %v", headers)
	}
}

// TestNewReader_FileNotFound tests error handling for missing files
func TestNewReader_FileNotFound(t *testing.T) {
	_, err := NewReader("/nonexistent/file.csv", ',')
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
}

// TestNewReader_CustomDelimiter tests handling custom delimiters
func TestNewReader_CustomDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		delimiter rune
	}{
		{
			name:      "semicolon",
			content:   "name;age;city\nJohn;30;NYC\n",
			delimiter: ';',
		},
		{
			name:      "pipe",
			content:   "name|age|city\nJohn|30|NYC\n",
			delimiter: '|',
		},
		{
			name:      "tab",
			content:   "name\tage\tcity\nJohn\t30\tNYC\n",
			delimiter: '\t',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempCSVFile(t, tt.content)
			defer os.Remove(tmpFile)

			reader, err := NewReader(tmpFile, tt.delimiter)
			if err != nil {
				t.Fatalf("NewReader failed: %v", err)
			}
			defer reader.Close()

			row, err := reader.Read()
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if row.Data["name"] != "John" {
				t.Errorf("Expected name=John, got %s", row.Data["name"])
			}
		})
	}
}

// TestRead tests reading all rows from a CSV
func TestRead(t *testing.T) {
	content := `name,age,city
John,30,NYC
Jane,25,LA
Bob,35,SF
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	// Read first row
	row1, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row1.Data["name"] != "John" {
		t.Errorf("Expected name=John, got %s", row1.Data["name"])
	}
	if row1.RowNumber != 2 {
		t.Errorf("Expected row number 2, got %d", row1.RowNumber)
	}

	// Read second row
	row2, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row2.Data["name"] != "Jane" {
		t.Errorf("Expected name=Jane, got %s", row2.Data["name"])
	}
	if row2.RowNumber != 3 {
		t.Errorf("Expected row number 3, got %d", row2.RowNumber)
	}

	// Read third row
	row3, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row3.Data["name"] != "Bob" {
		t.Errorf("Expected name=Bob, got %s", row3.Data["name"])
	}

	// Read should return EOF
	_, err = reader.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

// TestRead_QuotedValues tests handling quoted strings with commas
func TestRead_QuotedValues(t *testing.T) {
	content := `name,description,city
"John, Jr.","A person, who lives","NYC"
Jane,"Regular text",LA
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	row1, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row1.Data["name"] != "John, Jr." {
		t.Errorf("Expected 'John, Jr.', got '%s'", row1.Data["name"])
	}
	if row1.Data["description"] != "A person, who lives" {
		t.Errorf("Expected 'A person, who lives', got '%s'", row1.Data["description"])
	}
}

// TestRead_SpecialChars tests handling special characters
func TestRead_SpecialChars(t *testing.T) {
	content := `name,description
"Test with ""quotes""","Text with <>&"
Regular text,Normal
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	row1, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row1.Data["name"] != `Test with "quotes"` {
		t.Errorf("Expected 'Test with \"quotes\"', got '%s'", row1.Data["name"])
	}
	if row1.Data["description"] != "Text with <>&" {
		t.Errorf("Expected 'Text with <>&', got '%s'", row1.Data["description"])
	}
}

// TestRead_MismatchedColumns tests that CSV reader rejects rows with wrong column count
func TestRead_MismatchedColumns(t *testing.T) {
	content := `name,age,city
John,30,NYC
Jane,25
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	// First row should be complete
	row1, err := reader.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if row1.Data["city"] != "NYC" {
		t.Errorf("Expected city=NYC, got %s", row1.Data["city"])
	}

	// Second row has wrong number of fields - should error
	_, err = reader.Read()
	if err == nil {
		t.Error("Expected error for mismatched columns, got nil")
	}
}

// TestRead_EmptyFile tests handling empty CSV files
func TestRead_EmptyFile(t *testing.T) {
	content := ``
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	_, err := NewReader(tmpFile, ',')
	if err == nil {
		t.Fatal("Expected error for empty file, got nil")
	}
}

// TestRead_HeaderOnly tests handling CSV with only headers
func TestRead_HeaderOnly(t *testing.T) {
	content := `name,age,city
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	// First read should return EOF since there's no data
	_, err = reader.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF for header-only file, got %v", err)
	}
}

// TestReadAll tests reading all rows at once
func TestReadAll(t *testing.T) {
	content := `name,age,city
John,30,NYC
Jane,25,LA
Bob,35,SF
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(rows))
	}

	if rows[0].Data["name"] != "John" {
		t.Errorf("Expected first row name=John, got %s", rows[0].Data["name"])
	}
	if rows[2].Data["name"] != "Bob" {
		t.Errorf("Expected third row name=Bob, got %s", rows[2].Data["name"])
	}
}

// TestHeaders tests the Headers method
func TestHeaders(t *testing.T) {
	content := `col1,col2,col3
a,b,c
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	headers := reader.Headers()
	expected := []string{"col1", "col2", "col3"}

	if len(headers) != len(expected) {
		t.Fatalf("Expected %d headers, got %d", len(expected), len(headers))
	}

	for i, h := range headers {
		if h != expected[i] {
			t.Errorf("Header %d: expected %s, got %s", i, expected[i], h)
		}
	}
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	content := `name,age
John,30
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	err = reader.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify file was closed (second close returns error - expected behavior)
	err = reader.Close()
	if err == nil {
		t.Error("Expected error closing already closed file")
	}
}

// TestReuseRecord tests that ReuseRecord doesn't cause data corruption
func TestReuseRecord(t *testing.T) {
	content := `name,value
first,100
second,200
third,300
`
	tmpFile := createTempCSVFile(t, content)
	defer os.Remove(tmpFile)

	reader, err := NewReader(tmpFile, ',')
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer reader.Close()

	// Read all rows and store them
	var rows []*Row
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		rows = append(rows, row)
	}

	// Verify all rows still have correct data
	// This tests that string copying works correctly with ReuseRecord
	if rows[0].Data["name"] != "first" {
		t.Errorf("First row corrupted: expected name=first, got %s", rows[0].Data["name"])
	}
	if rows[1].Data["name"] != "second" {
		t.Errorf("Second row corrupted: expected name=second, got %s", rows[1].Data["name"])
	}
	if rows[2].Data["name"] != "third" {
		t.Errorf("Third row corrupted: expected name=third, got %s", rows[2].Data["name"])
	}
}

// Helper function to create a temporary CSV file
func createTempCSVFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.csv")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}
