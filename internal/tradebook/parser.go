package tradebook

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"stock-scorecard/internal/clientid"
)

// Trade represents a single raw trade row from a Zerodha tradebook CSV.
type Trade struct {
	Symbol    string
	ISIN      string
	TradeDate time.Time
	TradeType string // "buy" or "sell"
	Quantity  float64
	Price     float64
	TradeID   string
	OrderID   string
}

// ConsolidatedTrade represents fills consolidated into a single trade per
// (ISIN, date, trade_type, order_id).
type ConsolidatedTrade struct {
	Symbol    string
	ISIN      string
	Date      time.Time
	TradeType string  // "buy" or "sell"
	Quantity  float64 // rounded to int after consolidation
	AvgPrice  float64 // VWAP = Σ(qty × price) / Σ(qty)
	Value     float64 // Quantity × AvgPrice
	OrderID   string
}

const zerodhaHeader = "symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time"

// requiredEQColumns are the columns we actually use from the tradebook CSV.
// We check these exist (in any position) rather than requiring an exact header match,
// so the parser survives Zerodha format changes (extra columns, reordering, spacing).
var requiredEQColumns = []string{"symbol", "isin", "trade_date", "trade_type", "quantity", "price", "trade_id", "order_id"}

// buildColumnIndex maps column names (lowercased, trimmed) to their positions.
// Returns nil if any required column is missing.
func buildColumnIndex(header []string, required []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	for _, req := range required {
		if _, ok := idx[req]; !ok {
			return nil
		}
	}
	return idx
}

// ParseDirectory reads all *.csv files from dir, deduplicates by trade_id,
// consolidates fills, and returns sorted ConsolidatedTrades plus the detected
// client ID (e.g. "2632" from BT2632_*.csv filenames).
// excludes is a set of symbols to skip (e.g. LIQUIDBEES, GOLDBEES).
func ParseDirectory(dir string, excludes []string, clientFilter ...string) ([]ConsolidatedTrade, string, error) {
	excludeSet := make(map[string]bool, len(excludes))
	for _, s := range excludes {
		excludeSet[strings.ToUpper(strings.TrimSpace(s))] = true
	}

	// Glob all CSVs and rely on header validation to identify Zerodha tradebooks.
	// This supports any file naming convention (BT*, client_id_*, etc.).
	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, "", fmt.Errorf("glob csv files: %w", err)
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no .csv files found in %s", dir)
	}

	// Filter files by client ID prefix if specified
	filterID := ""
	if len(clientFilter) > 0 && clientFilter[0] != "" {
		filterID = strings.ToUpper(clientFilter[0])
		prefix := filterID + "_"
		var filtered []string
		for _, f := range files {
			if strings.HasPrefix(filepath.Base(f), prefix) {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) == 0 {
			return nil, "", fmt.Errorf("no .csv files matching client %s in %s", filterID, dir)
		}
		files = filtered
	}

	// Extract client ID from filenames
	basenames := make([]string, len(files))
	for i, f := range files {
		basenames[i] = filepath.Base(f)
	}
	detectedClientID, _ := clientid.Extract(basenames) // best-effort; empty if not detected

	seenTradeIDs := make(map[string]bool)
	var allTrades []Trade

	for _, f := range files {
		trades, err := parseFile(f, excludeSet, seenTradeIDs)
		if err != nil {
			// Skip non-Zerodha files silently
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	if len(allTrades) == 0 {
		return nil, "", fmt.Errorf("no trades parsed from %s", dir)
	}

	// Fill in blank ISINs from other trades of the same symbol
	isinBySymbol := make(map[string]string)
	for _, t := range allTrades {
		if t.ISIN != "" {
			isinBySymbol[t.Symbol] = t.ISIN
		}
	}
	for i := range allTrades {
		if allTrades[i].ISIN == "" {
			if isin, ok := isinBySymbol[allTrades[i].Symbol]; ok {
				allTrades[i].ISIN = isin
			}
		}
	}

	consolidated := consolidate(allTrades)

	// Sort by (ISIN, date, trade_type)
	sort.Slice(consolidated, func(i, j int) bool {
		if consolidated[i].ISIN != consolidated[j].ISIN {
			return consolidated[i].ISIN < consolidated[j].ISIN
		}
		if !consolidated[i].Date.Equal(consolidated[j].Date) {
			return consolidated[i].Date.Before(consolidated[j].Date)
		}
		return consolidated[i].TradeType < consolidated[j].TradeType
	})

	return consolidated, detectedClientID, nil
}

// parseFile reads a single CSV file, validates it has the required Zerodha
// columns, deduplicates by trade_id using the shared seenTradeIDs map, and
// returns parsed trades. Uses flexible column lookup so it survives Zerodha
// format changes (extra columns, reordering, spacing).
func parseFile(path string, excludes map[string]bool, seenTradeIDs map[string]bool) ([]Trade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Read and validate header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header %s: %w", path, err)
	}
	col := buildColumnIndex(header, requiredEQColumns)
	if col == nil {
		return nil, fmt.Errorf("not a Zerodha tradebook: %s", path)
	}
	// Reject F&O tradebooks — they have an expiry_date column that EQ files don't
	if _, hasFnO := col["expiry_date"]; hasFnO {
		return nil, fmt.Errorf("F&O tradebook (not equity): %s", path)
	}

	var trades []Trade
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row in %s: %w", path, err)
		}

		if len(record) <= col["order_id"] {
			continue
		}

		symbol := strings.TrimSpace(record[col["symbol"]])
		if excludes[strings.ToUpper(symbol)] {
			continue
		}

		isin := strings.TrimSpace(record[col["isin"]])

		tradeID := strings.TrimSpace(record[col["trade_id"]])
		dedupKey := symbol + "|" + tradeID
		if seenTradeIDs[dedupKey] {
			continue // duplicate
		}
		seenTradeIDs[dedupKey] = true

		tradeDate, err := time.Parse("2006-01-02", strings.TrimSpace(record[col["trade_date"]]))
		if err != nil {
			return nil, fmt.Errorf("parse date %q in %s: %w", record[col["trade_date"]], path, err)
		}

		qty, err := strconv.ParseFloat(strings.TrimSpace(record[col["quantity"]]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q in %s: %w", record[col["quantity"]], path, err)
		}

		price, err := strconv.ParseFloat(strings.TrimSpace(record[col["price"]]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse price %q in %s: %w", record[col["price"]], path, err)
		}

		trades = append(trades, Trade{
			Symbol:    symbol,
			ISIN:      isin,
			TradeDate: tradeDate,
			TradeType: strings.ToLower(strings.TrimSpace(record[col["trade_type"]])),
			Quantity:  qty,
			Price:     price,
			TradeID:   tradeID,
			OrderID:   strings.TrimSpace(record[col["order_id"]]),
		})
	}

	return trades, nil
}

// consolidateKey groups fills by (ISIN, date, trade_type, order_id).
type consolidateKey struct {
	ISIN      string
	Date      string
	TradeType string
	OrderID   string
}

// consolidate groups trades by (ISIN, trade_date, trade_type, order_id),
// computes VWAP, and rounds quantity to integer.
func consolidate(trades []Trade) []ConsolidatedTrade {
	type accum struct {
		symbol   string
		isin     string
		date     time.Time
		tt       string
		orderID  string
		totalQty float64
		totalVal float64 // Σ(qty × price)
	}

	groups := make(map[consolidateKey]*accum)
	// Preserve insertion order
	var keys []consolidateKey

	for _, t := range trades {
		k := consolidateKey{
			ISIN:      t.ISIN,
			Date:      t.TradeDate.Format("2006-01-02"),
			TradeType: t.TradeType,
			OrderID:   t.OrderID,
		}
		a, ok := groups[k]
		if !ok {
			a = &accum{
				symbol:  t.Symbol,
				isin:    t.ISIN,
				date:    t.TradeDate,
				tt:      t.TradeType,
				orderID: t.OrderID,
			}
			groups[k] = a
			keys = append(keys, k)
		}
		a.totalQty += t.Quantity
		a.totalVal += t.Quantity * t.Price
		// Keep latest symbol name (for renames)
		a.symbol = t.Symbol
	}

	result := make([]ConsolidatedTrade, 0, len(keys))
	for _, k := range keys {
		a := groups[k]
		if a.totalQty == 0 {
			continue
		}
		qty := math.Round(a.totalQty)
		avgPrice := a.totalVal / a.totalQty
		result = append(result, ConsolidatedTrade{
			Symbol:    a.symbol,
			ISIN:      a.isin,
			Date:      a.date,
			TradeType: a.tt,
			Quantity:  qty,
			AvgPrice:  avgPrice,
			Value:     qty * avgPrice,
			OrderID:   a.orderID,
		})
	}

	return result
}
