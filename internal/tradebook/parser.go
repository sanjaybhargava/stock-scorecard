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

// ParseDirectory reads all *.csv files from dir, deduplicates by trade_id,
// consolidates fills, and returns sorted ConsolidatedTrades.
// excludes is a set of symbols to skip (e.g. LIQUIDBEES, GOLDBEES).
func ParseDirectory(dir string, excludes []string) ([]ConsolidatedTrade, error) {
	excludeSet := make(map[string]bool, len(excludes))
	for _, s := range excludes {
		excludeSet[strings.ToUpper(strings.TrimSpace(s))] = true
	}

	// Match Zerodha tradebook naming: BT{client_id}_{start}_{end}.csv
	files, err := filepath.Glob(filepath.Join(dir, "BT*.csv"))
	if err != nil {
		return nil, fmt.Errorf("glob csv files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no BT*.csv tradebook files found in %s", dir)
	}

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
		return nil, fmt.Errorf("no trades parsed from %s", dir)
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

	return consolidated, nil
}

// parseFile reads a single CSV file, validates the Zerodha header, deduplicates
// by trade_id using the shared seenTradeIDs map, and returns parsed trades.
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
	if strings.Join(header, ",") != zerodhaHeader {
		return nil, fmt.Errorf("not a Zerodha tradebook: %s", path)
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

		// Columns: symbol(0), isin(1), trade_date(2), exchange(3), segment(4),
		//          series(5), trade_type(6), auction(7), quantity(8), price(9),
		//          trade_id(10), order_id(11), order_execution_time(12)
		if len(record) < 13 {
			continue
		}

		symbol := strings.TrimSpace(record[0])
		if excludes[strings.ToUpper(symbol)] {
			continue
		}

		isin := strings.TrimSpace(record[1])
		// Skip ETFs and mutual funds (ISIN prefix INF vs INE for equities)
		if strings.HasPrefix(isin, "INF") {
			continue
		}

		tradeID := strings.TrimSpace(record[10])
		if seenTradeIDs[tradeID] {
			continue // duplicate
		}
		seenTradeIDs[tradeID] = true

		tradeDate, err := time.Parse("2006-01-02", strings.TrimSpace(record[2]))
		if err != nil {
			return nil, fmt.Errorf("parse date %q in %s: %w", record[2], path, err)
		}

		qty, err := strconv.ParseFloat(strings.TrimSpace(record[8]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q in %s: %w", record[8], path, err)
		}

		price, err := strconv.ParseFloat(strings.TrimSpace(record[9]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse price %q in %s: %w", record[9], path, err)
		}

		trades = append(trades, Trade{
			Symbol:    symbol,
			ISIN:      isin,
			TradeDate: tradeDate,
			TradeType: strings.ToLower(strings.TrimSpace(record[6])),
			Quantity:  qty,
			Price:     price,
			TradeID:   tradeID,
			OrderID:   strings.TrimSpace(record[11]),
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
