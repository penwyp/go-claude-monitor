package formatter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewSummaryFormatter(t *testing.T) {
	formatter := NewSummaryFormatter()
	if formatter == nil {
		t.Fatal("NewSummaryFormatter returned nil")
	}
}

func TestSummaryFormatterFormat(t *testing.T) {
	formatter := NewSummaryFormatter()
	
	tests := []struct {
		name       string
		data       []GroupedData
		wantInBody []string // Strings that should appear in the summary
		notWant    []string // Strings that should NOT appear
	}{
		{
			name: "basic_summary",
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
				},
			},
			wantInBody: []string{
				"Summary",
				"Total Tokens:",
				"1,650",
				"Total Cost:",
				"$0.02",
				"Date Range:",
				"2024-01-15",
			},
		},
		{
			name: "multiple_models_summary",
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
				{
					Date:         "2024-01-17",
					Models:       []string{"claude-opus-4-20250514"},
					InputTokens:  5000,
					OutputTokens: 2500,
					TotalTokens:  7500,
					Cost:         0.135,
				},
			},
			wantInBody: []string{
				"Total Tokens:",
				"12,000", // 1500 + 3000 + 7500
				"Total Cost:",
				"$0.21", // 0.0225 + 0.055 + 0.135
				"Date Range:",
				"2024-01-15",
				"2024-01-17",
				"Model Usage:",
			},
		},
		{
			name:       "empty_data_summary",
			data:       []GroupedData{},
			wantInBody: []string{
				"No data to summarize",
			},
			notWant: []string{
				"Total Tokens:",
				"Model Usage:",
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
			wantInBody: []string{
				"Total Tokens:",
				"4,500",
				"Total Cost:",
				"$0.06",
				"Model Usage:",
			},
		},
		{
			name: "token_breakdown",
			data: []GroupedData{
				{
					Date:          "2024-01-15",
					Models:        []string{"claude-3-5-sonnet"},
					InputTokens:   1000000,
					OutputTokens:  500000,
					CacheCreation: 100000,
					CacheRead:     50000,
					TotalTokens:   1650000,
					Cost:          22.50,
				},
			},
			wantInBody: []string{
				"Token Breakdown:",
				"Input:",
				"1,000,000",
				"Output:",
				"500,000",
				"Cache Creation:",
				"100,000",
				"Cache Read:",
				"50,000",
			},
		},
		{
			name: "cost_breakdown",
			data: []GroupedData{
				{
					Date:         "2024-01-15",
					Models:       []string{"claude-3-5-sonnet"},
					InputTokens:  1000000,
					OutputTokens: 500000,
					TotalTokens:  1500000,
					Cost:         10.50,
				},
				{
					Date:         "2024-01-16",
					Models:       []string{"claude-opus-4-20250514"},
					InputTokens:  2000000,
					OutputTokens: 1000000,
					TotalTokens:  3000000,
					Cost:         105.00,
				},
			},
			wantInBody: []string{
				"Cost Breakdown:",
				"claude-3-5-sonnet:",
				"$10.50",
				"claude-opus-4-20250514:",
				"$105.00",
				"Total Cost:",
				"$115.50",
			},
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
			
			output := buf.String()
			
			// Check that expected strings appear in output
			for _, want := range tt.wantInBody {
				if !strings.Contains(output, want) {
					t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", want, output)
				}
			}
			
			// Check that unwanted strings do NOT appear
			for _, notWant := range tt.notWant {
				if strings.Contains(output, notWant) {
					t.Errorf("Expected output NOT to contain %q, but it did.\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestSummaryFormatterEdgeCases(t *testing.T) {
	formatter := NewSummaryFormatter()
	
	t.Run("zero_tokens_and_cost", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:         "2024-01-15",
				Models:       []string{"test"},
				InputTokens:  0,
				OutputTokens: 0,
				TotalTokens:  0,
				Cost:         0.0,
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
		if !strings.Contains(output, "Total Tokens:") {
			t.Error("Expected to show total tokens even when zero")
		}
		if !strings.Contains(output, "0") {
			t.Error("Expected to show zero value")
		}
	})
	
	t.Run("very_large_numbers", func(t *testing.T) {
		data := []GroupedData{
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
		// Should format large numbers with commas
		if !strings.Contains(output, "3,333,333,330") {
			t.Error("Expected large token count to be formatted with commas")
		}
		if !strings.Contains(output, "$149,999.99") {
			t.Error("Expected large cost to be formatted with commas")
		}
	})
	
	t.Run("model_deduplication", func(t *testing.T) {
		data := []GroupedData{
			{
				Date:        "2024-01-15",
				Models:      []string{"claude-3-5-sonnet", "claude-3-5-sonnet", "claude-3-5-sonnet"},
				TotalTokens: 1000,
				Cost:        0.015,
			},
			{
				Date:        "2024-01-16",
				Models:      []string{"claude-3-5-sonnet", "claude-3-5-haiku"},
				TotalTokens: 2000,
				Cost:        0.025,
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
		// Should show model usage statistics
		if !strings.Contains(output, "Model Usage:") {
			t.Error("Expected model usage section")
		}
		// Should deduplicate models per day
		if strings.Count(output, "claude-3-5-sonnet") > 2 {
			t.Error("Expected models to be deduplicated in summary")
		}
	})
}