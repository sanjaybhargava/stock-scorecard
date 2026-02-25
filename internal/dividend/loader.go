package dividend

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

// DividendEvent represents a single split-adjusted dividend payment.
type DividendEvent struct {
	ExDate time.Time
	Amount float64 // split-adjusted per-share amount
}

// DividendIndex holds dividend events keyed by symbol, sorted by date.
type DividendIndex struct {
	data map[string][]DividendEvent
}

// LoadDividends parses a CSV with columns: symbol, ex_date, amount.
// Returns a DividendIndex with events sorted by date per symbol.
func LoadDividends(path string) (*DividendIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Read and validate header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if len(header) < 3 || header[0] != "symbol" || header[1] != "ex_date" || header[2] != "amount" {
		return nil, fmt.Errorf("unexpected header: %v (expected symbol,ex_date,amount)", header)
	}

	idx := &DividendIndex{data: make(map[string][]DividendEvent)}
	lineNum := 1
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read line %d: %w", lineNum+1, err)
		}
		lineNum++

		symbol := row[0]
		exDate, err := time.Parse("2006-01-02", row[1])
		if err != nil {
			return nil, fmt.Errorf("parse date line %d: %w", lineNum, err)
		}
		amount, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse amount line %d: %w", lineNum, err)
		}

		idx.data[symbol] = append(idx.data[symbol], DividendEvent{
			ExDate: exDate,
			Amount: amount,
		})
	}

	// Sort events by date per symbol
	for sym := range idx.data {
		sort.Slice(idx.data[sym], func(i, j int) bool {
			return idx.data[sym][i].ExDate.Before(idx.data[sym][j].ExDate)
		})
	}

	totalEvents := 0
	for _, events := range idx.data {
		totalEvents += len(events)
	}
	log.Printf("Loaded %d dividend events for %d symbols", totalEvents, len(idx.data))

	return idx, nil
}

// Lookup returns the total dividend per share for a symbol between
// from (inclusive) and to (exclusive): from <= ex_date < to.
func (d *DividendIndex) Lookup(symbol string, from, to time.Time) float64 {
	events, ok := d.data[symbol]
	if !ok {
		return 0
	}

	total := 0.0
	for _, e := range events {
		if !e.ExDate.Before(from) && e.ExDate.Before(to) {
			total += e.Amount
		}
	}
	return total
}
