package fno

import (
	"testing"
)

func TestExtractUnderlying(t *testing.T) {
	tests := []struct {
		symbol     string
		wantUnderlying string
		wantType   string
	}{
		// Standard symbols
		{"BHARTIARTL20DEC520CE", "BHARTIARTL", "CE"},
		{"RELIANCE21JAN2000PE", "RELIANCE", "PE"},
		{"TCS22FEB3500CE", "TCS", "CE"},

		// Symbols with & (M&M)
		{"M&M22SEP1200CE", "M&M", "CE"},

		// Symbols with hyphen (MCDOWELL-N)
		{"MCDOWELL-N21OCT800PE", "MCDOWELL-N", "PE"},

		// Decimal strikes
		{"NTPC23JUN182.5CE", "NTPC", "CE"},
		{"POWERGRID23SEP198.75CE", "POWERGRID", "CE"},
		{"ITC24JAN450.50PE", "ITC", "PE"},

		// Should NOT match — no CE/PE suffix
		{"NIFTY23JUN18000", "", ""},

		// Should NOT match — starts with digit
		{"123FAKE21JAN100CE", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			gotUnderlying, gotType := extractUnderlying(tt.symbol)
			if gotUnderlying != tt.wantUnderlying {
				t.Errorf("extractUnderlying(%q) underlying = %q, want %q", tt.symbol, gotUnderlying, tt.wantUnderlying)
			}
			if gotType != tt.wantType {
				t.Errorf("extractUnderlying(%q) optionType = %q, want %q", tt.symbol, gotType, tt.wantType)
			}
		})
	}
}
