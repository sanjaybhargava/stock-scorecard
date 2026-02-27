// Package cleandata handles reading and writing the clean per-client data files
// (equity trades, F&O trades) in CSV format. These files live in ./data/BT{id}/
// and serve as the source of truth for scoring.
package cleandata

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/tradebook"
)

// ReviewItem represents a trade that needs human review or was auto-matched.
type ReviewItem struct {
	Tier      int    // 2=intelligent, 3=unresolved
	Status    string // "auto", "unresolved", "corrected", "skip"
	Symbol    string
	ISIN      string
	SellDate  string
	SellPrice float64
	Quantity  float64
	SellValue float64
	BuyDate   string // filled for Tier 2, empty for Tier 3
	BuyPrice  float64
	Reason    string // "transfer_in", "no_buy_found", etc.
}

// CoverageSummary holds statistics about match coverage by sell value.
type CoverageSummary struct {
	TotalSells       int
	MatchedSells     int
	TotalSellValue   float64
	MatchedSellValue float64
	IntelligentCount int
	IntelligentValue float64
	UnresolvedCount  int
	UnresolvedValue  float64
}

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

// WriteRealizedTrades writes matched buy-sell pairs to a CSV file.
func WriteRealizedTrades(path string, trades []matcher.RealizedTrade) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"symbol", "isin", "fy", "type", "buy_date", "sell_date", "hold_days", "quantity", "buy_price", "sell_price", "invested", "sale_value", "equity_gl", "nifty_buy_tri", "nifty_sell_tri", "nifty_return", "alpha", "tier", "tier_reason"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, t := range trades {
		alpha := t.EquityGL - t.NiftyReturn
		tier := int(t.Tier)
		if tier == 0 {
			tier = 1 // default to exact
		}
		row := []string{
			t.Symbol,
			t.ISIN,
			t.FY,
			t.Type,
			t.BuyDate.Format("2006-01-02"),
			t.SellDate.Format("2006-01-02"),
			strconv.Itoa(t.HoldDays),
			strconv.FormatFloat(t.Quantity, 'f', 0, 64),
			strconv.FormatFloat(t.BuyPrice, 'f', 2, 64),
			strconv.FormatFloat(t.SellPrice, 'f', 2, 64),
			strconv.FormatFloat(t.Invested, 'f', 0, 64),
			strconv.FormatFloat(t.SaleValue, 'f', 0, 64),
			strconv.FormatFloat(t.EquityGL, 'f', 0, 64),
			strconv.FormatFloat(t.NiftyBuy, 'f', 2, 64),
			strconv.FormatFloat(t.NiftySell, 'f', 2, 64),
			strconv.FormatFloat(t.NiftyReturn, 'f', 0, 64),
			strconv.FormatFloat(alpha, 'f', 0, 64),
			strconv.Itoa(tier),
			t.TierReason,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// WriteOpenPositions writes unmatched buy lots (open positions) to a CSV file.
func WriteOpenPositions(path string, positions []matcher.OpenPosition) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"symbol", "isin", "buy_date", "quantity", "buy_price", "invested"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, p := range positions {
		row := []string{
			p.Symbol,
			p.ISIN,
			p.BuyDate.Format("2006-01-02"),
			strconv.FormatFloat(p.Quantity, 'f', 0, 64),
			strconv.FormatFloat(p.BuyPrice, 'f', 2, 64),
			strconv.FormatFloat(p.Invested, 'f', 0, 64),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// WriteReviewCSV writes review items to a CSV file with a comment header
// containing coverage statistics.
func WriteReviewCSV(path string, items []ReviewItem, summary CoverageSummary) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	// Write comment header with coverage stats
	pct := 0.0
	if summary.TotalSellValue > 0 {
		pct = summary.MatchedSellValue / summary.TotalSellValue * 100
	}
	fmt.Fprintf(f, "# Coverage: %.1f%% by value (%d of %d sells matched)\n", pct, summary.MatchedSells, summary.TotalSells)
	fmt.Fprintf(f, "# Intelligent: %d items (value %.0f)\n", summary.IntelligentCount, summary.IntelligentValue)
	fmt.Fprintf(f, "# Unresolved: %d items (value %.0f)\n", summary.UnresolvedCount, summary.UnresolvedValue)
	fmt.Fprintf(f, "# Edit status column: auto → corrected (fill buy_date + buy_price), or auto → skip\n")

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"tier", "status", "symbol", "isin", "sell_date", "sell_price", "quantity", "sell_value", "buy_date", "buy_price", "reason"}
	if err := w.Write(header); err != nil {
		return err
	}

	// Sort: Tier 2 first, then Tier 3, within each group by sell_value desc
	sorted := make([]ReviewItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Tier != sorted[j].Tier {
			return sorted[i].Tier < sorted[j].Tier
		}
		return sorted[i].SellValue > sorted[j].SellValue
	})

	for _, item := range sorted {
		row := []string{
			strconv.Itoa(item.Tier),
			item.Status,
			item.Symbol,
			item.ISIN,
			item.SellDate,
			strconv.FormatFloat(item.SellPrice, 'f', 2, 64),
			strconv.FormatFloat(item.Quantity, 'f', 0, 64),
			strconv.FormatFloat(item.SellValue, 'f', 0, 64),
			item.BuyDate,
			fmtPrice(item.BuyPrice),
			item.Reason,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}

func fmtPrice(p float64) string {
	if p == 0 {
		return ""
	}
	return strconv.FormatFloat(p, 'f', 2, 64)
}
