package fno

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"stock-scorecard/internal/matcher"
)

// ContractPnL represents the net P&L for one option contract lifecycle.
type ContractPnL struct {
	Underlying string
	RawSymbol  string
	OptionType string // "CE" or "PE"
	ExpiryDate time.Time
	FirstDate  time.Time // first trade date in the contract
	NetPnL     float64   // Σ(sell_value) - Σ(buy_value)
}

// UnattributedFnO represents F&O income that couldn't be attributed to any
// equity realized trade (e.g., NIFTY index options, naked options).
type UnattributedFnO struct {
	Underlying string
	NetPnL     float64
	Note       string
}

// ComputeContractPnLs groups F&O trades by (underlying, raw_symbol) to compute
// one net P&L per option contract lifecycle.
func ComputeContractPnLs(trades []FnOTrade) []ContractPnL {
	type contractKey struct {
		underlying string
		rawSymbol  string
	}

	type accum struct {
		underlying string
		rawSymbol  string
		optionType string
		expiryDate time.Time
		firstDate  time.Time
		sellValue  float64
		buyValue   float64
	}

	groups := make(map[contractKey]*accum)
	var keyOrder []contractKey

	for _, t := range trades {
		k := contractKey{underlying: t.Underlying, rawSymbol: t.RawSymbol}
		a, ok := groups[k]
		if !ok {
			a = &accum{
				underlying: t.Underlying,
				rawSymbol:  t.RawSymbol,
				optionType: t.OptionType,
				expiryDate: t.ExpiryDate,
				firstDate:  t.TradeDate,
			}
			groups[k] = a
			keyOrder = append(keyOrder, k)
		}
		if t.TradeDate.Before(a.firstDate) {
			a.firstDate = t.TradeDate
		}
		if t.TradeType == "sell" {
			a.sellValue += t.Value
		} else {
			a.buyValue += t.Value
		}
	}

	result := make([]ContractPnL, 0, len(keyOrder))
	for _, k := range keyOrder {
		a := groups[k]
		result = append(result, ContractPnL{
			Underlying: a.underlying,
			RawSymbol:  a.rawSymbol,
			OptionType: a.optionType,
			ExpiryDate: a.expiryDate,
			FirstDate:  a.firstDate,
			NetPnL:     math.Round(a.sellValue - a.buyValue),
		})
	}

	log.Printf("Computed %d F&O contract P&Ls", len(result))
	return result
}

// indexedTrade pairs a realized trade with its index in the slice.
type indexedTrade struct {
	idx   int
	trade matcher.RealizedTrade
}

// attrWeight pairs a realized trade index with its attribution weight.
type attrWeight struct {
	idx int
	w   float64
}

// Attribute distributes each contract's net P&L to overlapping equity realized
// trades using a shares x overlap_days pro-rata weighting.
//
// For put contracts (PE) with no overlap, falls back to "next buy" attribution:
// finds realized trades whose buy_date is the nearest after the put's expiry,
// and distributes pro-rata by quantity.
//
// Returns:
//   - map of realized trade index → total option income attributed
//   - list of unattributed contracts (no overlapping equity trade)
func Attribute(contracts []ContractPnL, realized []matcher.RealizedTrade) (map[int]float64, []UnattributedFnO) {
	// Build a lookup: underlying → list of (index, realized trade)
	byUnderlying := make(map[string][]indexedTrade)
	for i, t := range realized {
		byUnderlying[t.Symbol] = append(byUnderlying[t.Symbol], indexedTrade{idx: i, trade: t})
	}

	attribution := make(map[int]float64)

	// Accumulate unattributed by underlying for cleaner output
	unattribByUnderlying := make(map[string]float64)

	for _, c := range contracts {
		candidates := byUnderlying[c.Underlying]
		if len(candidates) == 0 {
			unattribByUnderlying[c.Underlying] += c.NetPnL
			continue
		}

		// Try 1: overlap-based attribution (covered calls, protective puts)
		var weights []attrWeight
		totalWeight := 0.0

		for _, it := range candidates {
			overlap := overlapDays(c.FirstDate, c.ExpiryDate, it.trade.BuyDate, it.trade.SellDate)
			if overlap <= 0 {
				continue
			}
			w := it.trade.Quantity * float64(overlap)
			weights = append(weights, attrWeight{idx: it.idx, w: w})
			totalWeight += w
		}

		// Try 2: for puts with no overlap, attribute to the next equity buy
		// after the put's expiry (cash-secured put → stock purchase)
		if totalWeight == 0 && c.OptionType == "PE" {
			weights, totalWeight = nextBuyAttribution(c, candidates)
		}

		if totalWeight == 0 {
			unattribByUnderlying[c.Underlying] += c.NetPnL
			continue
		}

		// Distribute pro-rata
		for _, w := range weights {
			share := c.NetPnL * (w.w / totalWeight)
			attribution[w.idx] += share
		}
	}

	// Convert accumulated unattributed to list
	var unattributed []UnattributedFnO
	var underlyings []string
	for u := range unattribByUnderlying {
		underlyings = append(underlyings, u)
	}
	sort.Strings(underlyings)
	for _, u := range underlyings {
		pnl := unattribByUnderlying[u]
		unattributed = append(unattributed, UnattributedFnO{
			Underlying: u,
			NetPnL:     math.Round(pnl),
			Note:       "No equity position to attribute to",
		})
	}

	// Log summary
	totalAttributed := 0.0
	for _, v := range attribution {
		totalAttributed += v
	}
	totalUnattrib := 0.0
	for _, u := range unattributed {
		totalUnattrib += u.NetPnL
	}
	log.Printf("F&O attribution: %d trades received option income, total attributed: %.0f, unattributed: %.0f (%d underlyings)",
		len(attribution), totalAttributed, totalUnattrib, len(unattributed))

	return attribution, unattributed
}

// nextBuyAttribution finds the nearest equity buy date after a put's expiry
// and returns weights for all realized trades from that buy date.
// This handles cash-secured puts that led to stock purchases.
func nextBuyAttribution(c ContractPnL, candidates []indexedTrade) ([]attrWeight, float64) {
	// Find the earliest buy_date that is on or after the put's expiry
	var nearestBuyDate time.Time
	found := false
	for _, it := range candidates {
		buyDate := it.trade.BuyDate
		if !buyDate.Before(c.ExpiryDate) {
			if !found || buyDate.Before(nearestBuyDate) {
				nearestBuyDate = buyDate
				found = true
			}
		}
	}

	if !found {
		return nil, 0
	}

	// Collect all realized trades with that buy_date, weight by quantity
	var weights []attrWeight
	totalWeight := 0.0
	for _, it := range candidates {
		if it.trade.BuyDate.Equal(nearestBuyDate) {
			w := it.trade.Quantity
			weights = append(weights, attrWeight{idx: it.idx, w: w})
			totalWeight += w
		}
	}

	return weights, totalWeight
}

// overlapDays computes the number of overlapping days between two date ranges.
// Contract active period: [contractStart, contractEnd]
// Equity holding period: [buyDate, sellDate]
func overlapDays(contractStart, contractEnd, buyDate, sellDate time.Time) int {
	start := maxTime(contractStart, buyDate)
	end := minTime(contractEnd, sellDate)
	if !start.Before(end) {
		return 0
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 0 {
		return 0
	}
	return days
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// FnOFYSummary holds per-FY F&O income totals.
type FnOFYSummary struct {
	FY           string
	OptionIncome float64
}

// BuildFnOSummary computes total and per-FY F&O option income from realized trades.
func BuildFnOSummary(trades []matcher.RealizedTrade) (float64, []FnOFYSummary) {
	totalOption := 0.0
	byFY := make(map[string]float64)

	for _, t := range trades {
		if t.OptionIncome != 0 {
			totalOption += t.OptionIncome
			fy := fmt.Sprintf("FY %d-%02d", fyYear(t.SellDate), (fyYear(t.SellDate)+1)%100)
			byFY[fy] += t.OptionIncome
		}
	}

	// Sort FYs
	fys := make([]string, 0, len(byFY))
	for fy := range byFY {
		fys = append(fys, fy)
	}
	sort.Strings(fys)

	var result []FnOFYSummary
	for _, fy := range fys {
		result = append(result, FnOFYSummary{FY: fy, OptionIncome: byFY[fy]})
	}

	return totalOption, result
}

func fyYear(d time.Time) int {
	y := d.Year()
	if d.Month() < 4 {
		y--
	}
	return y
}
