package formatter

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestNewJSONFormatter(t *testing.T) {
	formatter := NewJSONFormatter()
	if formatter == nil {
		t.Fatal("NewJSONFormatter returned nil")
	}
}

func TestJSONFormatterFormat(t *testing.T) {
	formatter := NewJSONFormatter()
	
	tests := []struct {
		name    string
		data    []GroupedData
		wantErr bool
	}{
		{
			name: "basic_json_output",
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
			wantErr: false,
		},
		{
			name:    "empty_data",
			data:    []GroupedData{},
			wantErr: false,
		},
		{
			name: "multiple_entries",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-3-5-sonnet"},
					InputTokens:   1000,
					OutputTokens:  500,
					TotalTokens:   1500,
					Cost:          0.0225,
					ShowBreakdown: false,
				},
				{
					Date:          "2024-01-16",
					Models:        []string{"claude-3-5-haiku", "claude-opus-4-20250514"},
					InputTokens:   5000,
					OutputTokens:  2500,
					TotalTokens:   7500,
					Cost:          0.120,
					ShowBreakdown: true,
					ModelDetails: []ModelDetail{
						{
							Model:        "claude-3-5-haiku",
							InputTokens:  2000,
							OutputTokens: 1000,
							TotalTokens:  3000,
							Cost:         0.010,
						},
						{
							Model:        "claude-opus-4-20250514",
							InputTokens:  3000,
							OutputTokens: 1500,
							TotalTokens:  4500,
							Cost:         0.110,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			
			err := formatter.Format(tt.data)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				w.Close()
				os.Stdout = old
				return
			}
			
			if err != nil {
				w.Close()
				os.Stdout = old
				return
			}
			
			// Read output
			w.Close()
			buf := new(bytes.Buffer)
			io.Copy(buf, r)
			os.Stdout = old
			
			// Verify valid JSON
			var result []GroupedData
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Errorf("Invalid JSON output: %v\nOutput: %s", err, buf.String())
				return
			}
			
			// Verify data integrity
			if len(result) != len(tt.data) {
				t.Errorf("Expected %d entries, got %d", len(tt.data), len(result))
			}
			
			// For non-empty data, verify fields match
			if len(tt.data) > 0 {
				for i, original := range tt.data {
					if i >= len(result) {
						break
					}
					got := result[i]
					
					if got.Date != original.Date {
						t.Errorf("Entry %d: Date mismatch, got %s, want %s", i, got.Date, original.Date)
					}
					if len(got.Models) != len(original.Models) {
						t.Errorf("Entry %d: Models count mismatch, got %d, want %d", i, len(got.Models), len(original.Models))
					}
					if got.InputTokens != original.InputTokens {
						t.Errorf("Entry %d: InputTokens mismatch, got %d, want %d", i, got.InputTokens, original.InputTokens)
					}
					if got.TotalTokens != original.TotalTokens {
						t.Errorf("Entry %d: TotalTokens mismatch, got %d, want %d", i, got.TotalTokens, original.TotalTokens)
					}
					if got.Cost != original.Cost {
						t.Errorf("Entry %d: Cost mismatch, got %f, want %f", i, got.Cost, original.Cost)
					}
				}
			}
		})
	}
}

func TestJSONFormatterEdgeCases(t *testing.T) {
	formatter := NewJSONFormatter()
	
	t.Run("special_characters", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"test\"model\"with\\quotes", "model\nwith\nnewlines"},
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
		
		// Verify JSON escaping is correct
		var result []GroupedData
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		
		if len(result) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(result))
		}
		
		// Verify special characters are preserved after JSON encoding/decoding
		if len(result[0].Models) != 2 {
			t.Fatalf("Expected 2 models, got %d", len(result[0].Models))
		}
		if result[0].Models[0] != data[0].Models[0] {
			t.Errorf("Model special characters not preserved: got %q, want %q", result[0].Models[0], data[0].Models[0])
		}
		if result[0].Models[1] != data[0].Models[1] {
			t.Errorf("Model newlines not preserved: got %q, want %q", result[0].Models[1], data[0].Models[1])
		}
	})
	
	t.Run("unicode_characters", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"claude-日本語-模型", "🚀 rocket model 🌟"},
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
		
		// Verify Unicode is preserved
		var result []GroupedData
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		
		if result[0].Models[0] != data[0].Models[0] {
			t.Errorf("Unicode model name not preserved: got %q, want %q", result[0].Models[0], data[0].Models[0])
		}
		if result[0].Models[1] != data[0].Models[1] {
			t.Errorf("Emoji in model name not preserved: got %q, want %q", result[0].Models[1], data[0].Models[1])
		}
	})
	
	t.Run("large_numbers", func(t *testing.T) {
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
		var result []GroupedData
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		
		if result[0].InputTokens != data[0].InputTokens {
			t.Errorf("Large input tokens not preserved: got %d, want %d", result[0].InputTokens, data[0].InputTokens)
		}
		if result[0].TotalTokens != data[0].TotalTokens {
			t.Errorf("Large total tokens not preserved: got %d, want %d", result[0].TotalTokens, data[0].TotalTokens)
		}
		if result[0].Cost != data[0].Cost {
			t.Errorf("Large cost not preserved: got %f, want %f", result[0].Cost, data[0].Cost)
		}
	})
}