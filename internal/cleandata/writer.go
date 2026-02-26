// Package cleandata handles reading and writing the clean per-client data files
// (equity trades, F&O trades) in CSV format. These files live in ./data/BT{id}/
// and serve as the source of truth for scoring.
package cleandata

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/tradebook"
)

// WriteTrades writes consolidated equity trades to a CSV file.
func WriteTrades(path string, trades []tradebook.ConsolidatedTrade) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"symbol", "isin", "date", "trade_type", "quantity", "avg_price", "value", "order_id"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, t := range trades {
		row := []string{
			t.Symbol,
			t.ISIN,
			t.Date.Format("2006-01-02"),
			t.TradeType,
			strconv.FormatFloat(t.Quantity, 'f', 0, 64),
			strconv.FormatFloat(t.AvgPrice, 'f', 4, 64),
			strconv.FormatFloat(t.Value, 'f', 4, 64),
			t.OrderID,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// WriteFnOTrades writes consolidated F&O trades to a CSV file.
func WriteFnOTrades(path string, trades []fno.FnOTrade) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"raw_symbol", "underlying", "option_type", "expiry_date", "trade_date", "trade_type", "quantity", "price", "value", "trade_id", "order_id"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, t := range trades {
		row := []string{
			t.RawSymbol,
			t.Underlying,
			t.OptionType,
			t.ExpiryDate.Format("2006-01-02"),
			t.TradeDate.Format("2006-01-02"),
			t.TradeType,
			strconv.FormatFloat(t.Quantity, 'f', 0, 64),
			strconv.FormatFloat(t.Price, 'f', 4, 64),
			strconv.FormatFloat(t.Value, 'f', 4, 64),
			t.TradeID,
			t.OrderID,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
