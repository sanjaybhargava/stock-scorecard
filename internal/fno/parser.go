// Package fno handles parsing and attribution of F&O (Futures & Options) tradebook
// data from Zerodha. It computes per-contract P&L and attributes option income to
// equity realized trades using a two-pass approach:
//
//   1. Overlap-based attribution: for contracts whose active period overlaps with an
//      equity holding period (e.g., covered calls). Income is distributed pro-rata
//      by shares × overlap_days.
//
//   2. Next-buy fallback (PE only): for cash-secured puts with no overlap, income is
//      attributed to the nearest subsequent equity purchase of the same underlying,
//      distributed pro-rata by quantity.
//
// F&O tradebooks use a 14-column CSV format (same as equity + expiry_date column).
// Since F&O trades have no ISIN, underlying matching uses symbol names with explicit
// rename mappings for corporate actions (e.g., MOTHERSUMI→MOTHERSON, HDFC→HDFCBANK).
package fno

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FnOTrade represents a single consolidated F&O trade.
type FnOTrade struct {
	RawSymbol  string
	Underlying string
	OptionType string // "CE" or "PE"
	ExpiryDate time.Time
	TradeDate  time.Time
	TradeType  string  // "buy" or "sell"
	Quantity   float64
	Price      float64
	Value      float64 // Quantity x Price
	TradeID    string
	OrderID    string
}

// symbolRe extracts the underlying ticker from an F&O symbol.
// Examples: BHARTIARTL20DEC520CE → BHARTIARTL, M&M22SEP1200CE → M&M
// Handles decimal strikes like NTPC23JUN182.5CE, POWERGRID23SEP198.75CE
var symbolRe = regexp.MustCompile(`^([A-Z][A-Z&-]*[A-Z])\d{2}[A-Z]{3}\d+(?:\.\d+)?(CE|PE)$`)

// symbolRenames maps old F&O underlying names to current equity display names.
// F&O has no ISIN, so we need explicit symbol mapping for renames and mergers.
var symbolRenames = map[string]string{
	"MOTHERSUMI": "MOTHERSON",
	"HDFC":       "HDFCBANK",
}

const fnoHeader = "symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time,expiry_date"

// ParseDirectory reads all BT*_FO_*.csv files from dir, deduplicates by
// trade_id, consolidates fills, and returns sorted FnOTrades.
func ParseDirectory(dir string) ([]FnOTrade, error) {
	// Glob all CSVs and rely on header validation to identify F&O tradebooks.
	// This supports any file naming convention (BT*_FO_*, client_id_FO_*, etc.).
	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, fmt.Errorf("glob F&O files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .csv files found in %s", dir)
	}

	seenTradeIDs := make(map[string]bool)
	var allRaw []rawFnOTrade

	for _, f := range files {
		trades, err := parseFnOFile(f, seenTradeIDs)
		if err != nil {
			log.Printf("Skipping F&O file %s: %v", f, err)
			continue
		}
		allRaw = append(allRaw, trades...)
	}

	if len(allRaw) == 0 {
		// No F&O trades found is normal — directory may contain only equity tradebooks.
		return nil, nil
	}

	log.Printf("Parsed %d raw F&O trades from %d files", len(allRaw), len(files))

	consolidated := consolidateFnO(allRaw)

	// Sort by underlying, then trade_date
	sort.Slice(consolidated, func(i, j int) bool {
		if consolidated[i].Underlying != consolidated[j].Underlying {
			return consolidated[i].Underlying < consolidated[j].Underlying
		}
		return consolidated[i].TradeDate.Before(consolidated[j].TradeDate)
	})

	return consolidated, nil
}

// rawFnOTrade is the internal representation before consolidation.
type rawFnOTrade struct {
	rawSymbol  string
	underlying string
	optionType string // "CE" or "PE"
	expiryDate time.Time
	tradeDate  time.Time
	tradeType  string
	quantity   float64
	price      float64
	tradeID    string
	orderID    string
}

func parseFnOFile(path string, seenTradeIDs map[string]bool) ([]rawFnOTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header %s: %w", path, err)
	}
	if strings.Join(header, ",") != fnoHeader {
		return nil, fmt.Errorf("not an F&O tradebook: %s (header: %s)", path, strings.Join(header, ","))
	}

	var trades []rawFnOTrade
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row in %s: %w", path, err)
		}
		if len(record) < 14 {
			continue
		}

		symbol := strings.TrimSpace(record[0])
		tradeID := strings.TrimSpace(record[10])
		dedupKey := symbol + "|" + tradeID
		if seenTradeIDs[dedupKey] {
			continue
		}
		seenTradeIDs[dedupKey] = true

		underlying, optionType := extractUnderlying(symbol)
		if underlying == "" {
			log.Printf("Could not extract underlying from F&O symbol %q, skipping", symbol)
			continue
		}

		// Apply symbol renames
		if renamed, ok := symbolRenames[underlying]; ok {
			underlying = renamed
		}

		tradeDate, err := time.Parse("2006-01-02", strings.TrimSpace(record[2]))
		if err != nil {
			return nil, fmt.Errorf("parse trade_date %q in %s: %w", record[2], path, err)
		}

		expiryDate, err := time.Parse("2006-01-02", strings.TrimSpace(record[13]))
		if err != nil {
			return nil, fmt.Errorf("parse expiry_date %q in %s: %w", record[13], path, err)
		}

		qty, err := strconv.ParseFloat(strings.TrimSpace(record[8]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q in %s: %w", record[8], path, err)
		}

		price, err := strconv.ParseFloat(strings.TrimSpace(record[9]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse price %q in %s: %w", record[9], path, err)
		}

		trades = append(trades, rawFnOTrade{
			rawSymbol:  symbol,
			underlying: underlying,
			optionType: optionType,
			expiryDate: expiryDate,
			tradeDate:  tradeDate,
			tradeType:  strings.ToLower(strings.TrimSpace(record[6])),
			quantity:   qty,
			price:      price,
			tradeID:    tradeID,
			orderID:    strings.TrimSpace(record[11]),
		})
	}

	return trades, nil
}

// extractUnderlying parses the underlying ticker and option type from an F&O symbol.
// Returns underlying (e.g. "BHARTIARTL") and optionType ("CE" or "PE").
func extractUnderlying(symbol string) (string, string) {
	m := symbolRe.FindStringSubmatch(symbol)
	if len(m) < 3 {
		return "", ""
	}
	return m[1], m[2]
}

// consolidateKey groups F&O fills by (rawSymbol, trade_date, trade_type, order_id).
type consolidateKey struct {
	rawSymbol string
	date      string
	tradeType string
	orderID   string
}

// consolidateFnO groups raw trades by (rawSymbol, trade_date, trade_type, order_id),
// computes VWAP, and returns consolidated FnOTrades.
func consolidateFnO(trades []rawFnOTrade) []FnOTrade {
	type accum struct {
		rawSymbol  string
		underlying string
		optionType string
		expiryDate time.Time
		tradeDate  time.Time
		tradeType  string
		orderID    string
		totalQty   float64
		totalVal   float64 // Σ(qty × price)
		tradeID    string  // keep first trade_id
	}

	groups := make(map[consolidateKey]*accum)
	var keys []consolidateKey

	for _, t := range trades {
		k := consolidateKey{
			rawSymbol: t.rawSymbol,
			date:      t.tradeDate.Format("2006-01-02"),
			tradeType: t.tradeType,
			orderID:   t.orderID,
		}
		a, ok := groups[k]
		if !ok {
			a = &accum{
				rawSymbol:  t.rawSymbol,
				underlying: t.underlying,
				optionType: t.optionType,
				expiryDate: t.expiryDate,
				tradeDate:  t.tradeDate,
				tradeType:  t.tradeType,
				orderID:    t.orderID,
				tradeID:    t.tradeID,
			}
			groups[k] = a
			keys = append(keys, k)
		}
		a.totalQty += t.quantity
		a.totalVal += t.quantity * t.price
	}

	result := make([]FnOTrade, 0, len(keys))
	for _, k := range keys {
		a := groups[k]
		if a.totalQty == 0 {
			continue
		}
		qty := math.Round(a.totalQty)
		avgPrice := a.totalVal / a.totalQty
		result = append(result, FnOTrade{
			RawSymbol:  a.rawSymbol,
			Underlying: a.underlying,
			OptionType: a.optionType,
			ExpiryDate: a.expiryDate,
			TradeDate:  a.tradeDate,
			TradeType:  a.tradeType,
			Quantity:   qty,
			Price:      avgPrice,
			Value:      qty * avgPrice,
			TradeID:    a.tradeID,
			OrderID:    a.orderID,
		})
	}

	return result
}
