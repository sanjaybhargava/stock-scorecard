package cleandata

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/tradebook"
)

// ReadTrades reads consolidated equity trades from a clean CSV file.
func ReadTrades(path string) ([]tradebook.ConsolidatedTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var trades []tradebook.ConsolidatedTrade
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 8 {
			continue
		}

		date, err := time.Parse("2006-01-02", row[2])
		if err != nil {
			return nil, fmt.Errorf("parse date %q: %w", row[2], err)
		}
		qty, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[4], err)
		}
		avgPrice, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("parse avg_price %q: %w", row[5], err)
		}
		value, err := strconv.ParseFloat(row[6], 64)
		if err != nil {
			return nil, fmt.Errorf("parse value %q: %w", row[6], err)
		}

		trades = append(trades, tradebook.ConsolidatedTrade{
			Symbol:    row[0],
			ISIN:      row[1],
			Date:      date,
			TradeType: row[3],
			Quantity:  qty,
			AvgPrice:  avgPrice,
			Value:     value,
			OrderID:   row[7],
		})
	}

	return trades, nil
}

// ReadFnOTrades reads consolidated F&O trades from a clean CSV file.
func ReadFnOTrades(path string) ([]fno.FnOTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var trades []fno.FnOTrade
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 11 {
			continue
		}

		expiryDate, err := time.Parse("2006-01-02", row[3])
		if err != nil {
			return nil, fmt.Errorf("parse expiry_date %q: %w", row[3], err)
		}
		tradeDate, err := time.Parse("2006-01-02", row[4])
		if err != nil {
			return nil, fmt.Errorf("parse trade_date %q: %w", row[4], err)
		}
		qty, err := strconv.ParseFloat(row[6], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[6], err)
		}
		price, err := strconv.ParseFloat(row[7], 64)
		if err != nil {
			return nil, fmt.Errorf("parse price %q: %w", row[7], err)
		}
		value, err := strconv.ParseFloat(row[8], 64)
		if err != nil {
			return nil, fmt.Errorf("parse value %q: %w", row[8], err)
		}

		trades = append(trades, fno.FnOTrade{
			RawSymbol:  row[0],
			Underlying: row[1],
			OptionType: row[2],
			ExpiryDate: expiryDate,
			TradeDate:  tradeDate,
			TradeType:  row[5],
			Quantity:   qty,
			Price:      price,
			Value:      value,
			TradeID:    row[9],
			OrderID:    row[10],
		})
	}

	return trades, nil
}
