package tri

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// TRIIndex holds the NIFTY 500 TRI indexed values keyed by date string.
type TRIIndex struct {
	values map[string]float64
	dates  []string // sorted ascending
}

// LoadTRI reads a TRI CSV (Date,TRI_Indexed) and returns a TRIIndex.
func LoadTRI(path string) (*TRIIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open TRI file: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Read and validate header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read TRI header: %w", err)
	}
	if len(header) < 2 || strings.TrimSpace(header[0]) != "Date" || strings.TrimSpace(header[1]) != "TRI_Indexed" {
		return nil, fmt.Errorf("unexpected TRI header: %v", header)
	}

	values := make(map[string]float64)
	var dates []string

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read TRI row: %w", err)
		}
		if len(record) < 2 {
			continue
		}

		date := strings.TrimSpace(record[0])
		valStr := strings.TrimSpace(record[1])
		if valStr == "" {
			continue // skip rows with missing values
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, fmt.Errorf("parse TRI value %q for %s: %w", record[1], date, err)
		}

		values[date] = val
		dates = append(dates, date)
	}

	sort.Strings(dates)

	if len(dates) == 0 {
		return nil, fmt.Errorf("no TRI data in %s", path)
	}

	return &TRIIndex{values: values, dates: dates}, nil
}

// Lookup returns the TRI value for the given date (YYYY-MM-DD).
// If the exact date is not found (weekend/holiday), it falls back to the
// most recent prior trading day.
func (idx *TRIIndex) Lookup(date string) (float64, error) {
	if v, ok := idx.values[date]; ok {
		return v, nil
	}

	// Binary search for the most recent prior date
	i := sort.SearchStrings(idx.dates, date)
	if i == 0 {
		return 0, fmt.Errorf("no TRI data on or before %s", date)
	}
	return idx.values[idx.dates[i-1]], nil
}
