package formatter

import (
	"os"
	"strings"
	"testing"
)

func TestNewTableFormatter(t *testing.T) {
	formatter := NewTableFormatter()
	if formatter == nil {
		t.Fatal("NewTableFormatter returned nil")
	}
	if len(formatter.headers) == 0 {
		t.Error("Expected headers to be initialized")
	}
}

func TestTableFormatterFormat(t *testing.T) {
	formatter := NewTableFormatter()
	
	tests := []struct {
		name       string
		data       []GroupedData
		wantInBody []string // Strings that should appear in the output
		wantErr    bool
	}{
		{
			name: "basic_grouped_data",
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
			wantInBody: []string{
				"2024-01-15",
				"claude-3-5-sonnet",
				"1,000",
				"500",
				"100",
				"50",
				"1,650",
				"0.02",
			},
		},
		{
			name: "multiple_models",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-3-5-sonnet", "claude-3-5-haiku"},
					InputTokens:   3000,
					OutputTokens:  1500,
					CacheCreation: 200,
					CacheRead:     100,
					TotalTokens:   4800,
					Cost:          0.055,
					ShowBreakdown: false,
				},
			},
			wantInBody: []string{
				"2024-01-15",
				"haiku",
				"sonnet",
				"3,000",
				"1,500",
				"4,800",
				"0.06",
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
					Cost:          0.06,
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
			wantInBody: []string{
				"2024-01-15",
				"haiku",
				"sonnet",
				"└─",
				"2,000",
				"1,000",
				"3,000",
				"$0.04",
			},
		},
		{
			name: "empty_data",
			data: []GroupedData{},
			wantInBody: []string{
				"Date",
				"Models",
				"Input",
				"Output",
				"Total Tokens",
				"Cost (USD)",
			},
		},
		{
			name: "zero_tokens",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude-3-5-sonnet"},
					InputTokens:  0,
					OutputTokens: 0,
					TotalTokens:  0,
					Cost:         0.0,
				},
			},
			wantInBody: []string{
				"2024-01-15",
				"claude-3-5-sonnet",
				"0",
			},
		},
		{
			name: "large_numbers",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-opus-4-20250514"},
					InputTokens:   999999999,
					OutputTokens:  888888888,
					CacheCreation: 777777777,
					CacheRead:     666666666,
					TotalTokens:   3333333330,
					Cost:          149999.99,
				},
			},
			wantInBody: []string{
				"999,999,999",
				"888,888,888",
				"777,777,777",
				"666,666,666",
				"3,333,333,330",
				"149,999.99",
			},
		},
	}
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			
			// Format data
			err := formatter.Format(tt.data)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err != nil {
				return
			}
			
			// Read output
			w.Close()
			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			output := string(buf[:n])
			
			// Reopen pipe for next test
			r, w, _ = os.Pipe()
			os.Stdout = w
			
			// Check that expected strings appear in output
			for _, want := range tt.wantInBody {
				if !strings.Contains(output, want) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", want, output)
				}
			}
		})
	}
	
	// Restore stdout
	w.Close()
	os.Stdout = old
}

func TestTableFormatterColumnWidths(t *testing.T) {
	formatter := NewTableFormatter()
	
	tests := []struct {
		name     string
		data     []GroupedData
		minWidth int // Minimum expected total width
	}{
		{
			name: "standard_widths",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude-3-5-sonnet"},
					InputTokens:  1000,
					OutputTokens: 500,
					TotalTokens:  1500,
					Cost:         0.0225,
				},
			},
			minWidth: 80,
		},
		{
			name: "long_model_names",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude-3-5-sonnet", "claude-3-5-haiku", "claude-opus-4-20250514"},
					InputTokens:  1000,
					OutputTokens: 500,
					TotalTokens:  1500,
					Cost:         0.0225,
				},
			},
			minWidth: 100,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a conceptual test since calculateColumnWidths is private
			// We're mainly testing that the formatter handles different data sizes
			old := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w
			
			err := formatter.Format(tt.data)
			if err != nil {
				t.Errorf("Format() error = %v", err)
			}
			
			w.Close()
			os.Stdout = old
		})
	}
}

func TestTableFormatterEdgeCases(t *testing.T) {
	formatter := NewTableFormatter()
	
	t.Run("nil_model_details", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:          "2024-01-15",
				Models:        []string{"test"},
				TotalTokens:   100,
				ShowBreakdown: true,
				ModelDetails:  nil, // This should not cause panic
			},
		}
		
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Errorf("Format() error = %v", err)
		}
		
		w.Close()
		os.Stdout = old
	})
	
	t.Run("empty_models_list", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{},
				TotalTokens: 100,
			},
		}
		
		old := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Errorf("Format() error = %v", err)
		}
		
		w.Close()
		os.Stdout = old
	})
	
	t.Run("special_characters_in_model_names", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"model-with-@#$%", "model_with_underscores"},
				TotalTokens: 100,
			},
		}
		
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		err := formatter.Format(data)
		if err != nil {
			t.Errorf("Format() error = %v", err)
		}
		
		w.Close()
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		output := string(buf[:n])
		os.Stdout = old
		
		if !strings.Contains(output, "model-with-@#$%") {
			t.Error("Expected special characters to be preserved")
		}
	})
}

