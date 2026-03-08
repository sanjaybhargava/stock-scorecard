package clientid

import (
	"testing"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name      string
		filenames []string
		want      string
		wantErr   bool
	}{
		{
			name:      "BT prefix client",
			filenames: []string{"BT2632_20200101_20201231.csv"},
			want:      "BT2632",
		},
		{
			name:      "non-BT prefix client",
			filenames: []string{"XY1234_20200101_20201231.csv"},
			want:      "XY1234",
		},
		{
			name:      "multiple files same client",
			filenames: []string{"BT2632_20200101_20201231.csv", "BT2632_FO_20200101_20201231.csv", "BT2632_20210101_20211231.csv"},
			want:      "BT2632",
		},
		{
			name:      "non-BT multiple files same client",
			filenames: []string{"AB5678_20200101_20201231.csv", "AB5678_FO_20200101_20201231.csv"},
			want:      "AB5678",
		},
		{
			name:      "three-letter prefix client",
			filenames: []string{"DUA527_EQ_20240101_20241231.csv", "DUA527_FO_20240101_20241231.csv"},
			want:      "DUA527",
		},
		{
			name:      "mixed with non-matching files",
			filenames: []string{"BT9999_20200101_20201231.csv", "NIFTY500_TRI_Indexed.csv", "dividends.csv"},
			want:      "BT9999",
		},
		{
			name:      "no matching files",
			filenames: []string{"NIFTY500_TRI_Indexed.csv", "dividends.csv"},
			wantErr:   true,
		},
		{
			name:      "multiple client IDs",
			filenames: []string{"BT2632_20200101_20201231.csv", "XY9999_20200101_20201231.csv"},
			wantErr:   true,
		},
		{
			name:      "empty list",
			filenames: []string{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Extract(tt.filenames)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Extract() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Extract() = %q, want %q", got, tt.want)
			}
		})
	}
}
