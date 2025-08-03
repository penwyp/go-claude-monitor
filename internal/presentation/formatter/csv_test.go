package formatter

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewCSVFormatter(t *testing.T) {
	formatter := NewCSVFormatter()
	if formatter == nil {
		t.Fatal("NewCSVFormatter returned nil")
	}
}

func TestCSVFormatterFormat(t *testing.T) {
	formatter := NewCSVFormatter()
	
	tests := []struct {
		name       string
		data       []GroupedData
		wantHeaders []string
		wantRows   int
		checkFields map[int][]string // row index -> expected fields
	}{
		{
			name: "basic_csv_output",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-3-5-sonnet"},
					InputTokens:   1000,
					OutputTokens:  500,
					CacheCreation: 100,
					CacheRead:     50,
					TotalTokens:   1650,
					Cost:          0.0225,
					ShowBreakdown: false,
				},
			},
			wantHeaders: []string{
				"Date",
				"Models",
				"Input",
				"Output",
				"Cache Create",
				"Cache Read",
				"Total Tokens",
				"Cost (USD)",
			},
			wantRows: 1,
			checkFields: map[int][]string{
				0: {"2024-01-15", "claude-3-5-sonnet", "1000", "500", "100", "50", "1650", "0.02"},
			},
		},
		{
			name:        "empty_data",
			data:        []GroupedData{},
			wantHeaders: []string{"Date", "Models"},
			wantRows:    0,
		},
		{
			name: "multiple_entries",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude-3-5-sonnet"},
					InputTokens:  1000,
					OutputTokens: 500,
					TotalTokens:  1500,
					Cost:         0.0225,
				},
				{
					Date:         "2024-01-16",
					Models:       []string{"claude-3-5-haiku", "claude-opus-4-20250514"},
					InputTokens:  2000,
					OutputTokens: 1000,
					TotalTokens:  3000,
					Cost:         0.055,
				},
			},
			wantRows: 2,
			checkFields: map[int][]string{
				0: {"2024-01-15", "claude-3-5-sonnet", "1000", "500", "1500"},
				1: {"2024-01-16", "Opus-4, claude-3-5-haiku", "2000", "1000", "3000"},
			},
		},
		{
			name: "with_model_breakdown",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-3-5-sonnet", "claude-3-5-haiku"},
					InputTokens:   3000,
					OutputTokens:  1500,
					TotalTokens:   4500,
					Cost:          0.055,
					ShowBreakdown: true,
					ModelDetails: []ModelDetail{
						{
							Model:        "claude-3-5-sonnet",
							InputTokens:  2000,
							OutputTokens: 1000,
							TotalTokens:  3000,
							Cost:         0.045,
						},
						{
							Model:        "claude-3-5-haiku",
							InputTokens:  1000,
							OutputTokens: 500,
							TotalTokens:  1500,
							Cost:         0.010,
						},
					},
				},
			},
			wantRows: 3, // 1 main row + 2 model detail rows
			checkFields: map[int][]string{
				0: {"2024-01-15", "claude-3-5-sonnet, claude-3-5-haiku", "3000", "1500", "4500"},
				1: {"└─ 2024-01-15", "claude-3-5-sonnet", "2000", "1000", "3000"},
				2: {"└─ 2024-01-15", "claude-3-5-haiku", "1000", "500", "1500"},
			},
		},
		{
			name: "special_characters_in_fields",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude,with,commas", "model\"with\"quotes"},
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
					Cost:         0.01,
				},
			},
			wantRows: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			
			err := formatter.Format(tt.data)
			if err != nil {
				t.Fatalf("Format returned error: %v", err)
			}
			
			// Read output
			w.Close()
			buf := new(bytes.Buffer)
			io.Copy(buf, r)
			os.Stdout = old
			
			// Parse CSV output
			reader := csv.NewReader(buf)
			records, err := reader.ReadAll()
			if err != nil {
				t.Fatalf("Failed to parse CSV output: %v", err)
			}
			
			// Check headers
			if len(records) > 0 {
				headers := records[0]
				for _, expectedHeader := range tt.wantHeaders {
					found := false
					for _, header := range headers {
						if header == expectedHeader {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected header %q not found in %v", expectedHeader, headers)
					}
				}
			}
			
			// Check number of data rows (excluding header)
			dataRows := len(records) - 1
			if dataRows < 0 {
				dataRows = 0
			}
			if dataRows != tt.wantRows {
				t.Errorf("Expected %d data rows, got %d", tt.wantRows, dataRows)
			}
			
			// Check specific field values
			for rowIdx, expectedFields := range tt.checkFields {
				actualRowIdx := rowIdx + 1 // Skip header
				if actualRowIdx >= len(records) {
					t.Errorf("Row %d not found in output", rowIdx)
					continue
				}
				
				row := records[actualRowIdx]
				for _, expected := range expectedFields {
					found := false
					for _, field := range row {
						if strings.Contains(field, expected) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Row %d: expected field containing %q not found in %v", rowIdx, expected, row)
					}
				}
			}
		})
	}
}

func TestCSVFormatterEdgeCases(t *testing.T) {
	formatter := NewCSVFormatter()
	
	t.Run("newlines_in_fields", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"model\nwith\nnewlines", "model\r\nwith\r\nCRLF"},
				TotalTokens: 100,
				Cost:        0.01,
			},
		}
		
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Fatalf("Format returned error: %v", err)
		}
		
		// Read output
		w.Close()
		buf := new(bytes.Buffer)
		io.Copy(buf, r)
		os.Stdout = old
		
		// CSV should properly handle fields with newlines
		output := buf.String()
		if !strings.Contains(output, "model") || !strings.Contains(output, "newlines") {
			t.Error("Expected model names to be included in CSV output")
		}
	})
	
	t.Run("very_large_numbers", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:          "2024-01-15",
				Models:        []string{"test"},
				InputTokens:   999999999,
				OutputTokens:  888888888,
				CacheCreation: 777777777,
				CacheRead:     666666666,
				TotalTokens:   3333333330,
				Cost:          149999.99,
			},
		}
		
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Fatalf("Format returned error: %v", err)
		}
		
		// Read output
		w.Close()
		buf := new(bytes.Buffer)
		io.Copy(buf, r)
		os.Stdout = old
		
		// Verify large numbers are preserved
		reader := csv.NewReader(buf)
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("Failed to parse CSV: %v", err)
		}
		
		if len(records) < 2 {
			t.Fatal("Expected at least 2 records (header + data)")
		}
		
		// Check that large numbers are in the output
		dataRow := strings.Join(records[1], ",")
		if !strings.Contains(dataRow, "999999999") {
			t.Error("Expected large input tokens to be preserved")
		}
		if !strings.Contains(dataRow, "3333333330") {
			t.Error("Expected large total tokens to be preserved")
		}
	})
	
	t.Run("unicode_and_emoji", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"claude-日本語", "🚀 rocket"},
				TotalTokens: 100,
				Cost:        0.01,
			},
		}
		
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Fatalf("Format returned error: %v", err)
		}
		
		// Read output
		w.Close()
		buf := new(bytes.Buffer)
		io.Copy(buf, r)
		os.Stdout = old
		
		output := buf.String()
		if !strings.Contains(output, "claude-日本語") {
			t.Error("Expected Japanese characters to be preserved")
		}
		if !strings.Contains(output, "🚀 rocket") {
			t.Error("Expected emoji to be preserved")
		}
	})
	
	t.Run("empty_models_list", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{},
				TotalTokens: 100,
				Cost:        0.01,
			},
		}
		
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Fatalf("Format returned error: %v", err)
		}
		
		// Read output
		w.Close()
		buf := new(bytes.Buffer)
		io.Copy(buf, r)
		os.Stdout = old
		
		// Verify it doesn't panic and produces valid CSV
		reader := csv.NewReader(buf)
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatalf("Failed to parse CSV with empty models: %v", err)
		}
		
		if len(records) < 2 {
			t.Error("Expected at least header and one data row")
		}
	})
}