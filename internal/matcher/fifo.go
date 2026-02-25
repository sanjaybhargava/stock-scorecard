package matcher

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

// RealizedTrade represents a matched buy-sell pair.
type RealizedTrade struct {
	Symbol      string
	ISIN        string
	BuyDate     time.Time
	SellDate    time.Time
	HoldDays    int
	Quantity    float64
	BuyPrice    float64
	SellPrice   float64
	Invested    float64 // Quantity × BuyPrice
	SaleValue   float64 // Quantity × SellPrice
	EquityGL    float64 // SaleValue - Invested
	NiftyBuy    float64 // TRI on buy date
	NiftySell   float64 // TRI on sell date
	NiftyReturn float64 // Invested × (NiftySell/NiftyBuy - 1)
	FY          string  // FY of sell date, e.g. "FY 2024-25"
	Type        string  // "Long" if HoldDays > 365, else "Short"
}

// OpenPosition represents unmatched buy lots still held.
type OpenPosition struct {
	Symbol   string
	ISIN     string
	BuyDate  time.Time
	Quantity float64
	BuyPrice float64
	Invested float64
}

// SymbolSummary provides per-symbol FIFO diagnostics for verbose output.
type SymbolSummary struct {
	Symbol        string
	SharesBought  float64
	SharesSold    float64
	SharesMatched float64
	SharesOpen    float64
}

// buyLot is an internal struct for the FIFO queue.
type buyLot struct {
	date     time.Time
	qty      float64
	price    float64
	symbol   string
	isin     string
	orderID  string
}

// Match performs FIFO matching on consolidated trades, enriches with TRI data,
// and returns realized trades, open positions, and per-symbol summaries.
func Match(trades []tradebook.ConsolidatedTrade, triIdx *tri.TRIIndex) ([]RealizedTrade, []OpenPosition, []SymbolSummary, error) {
	// Group trades by ISIN
	byISIN := make(map[string][]tradebook.ConsolidatedTrade)
	var isinOrder []string
	for _, t := range trades {
		if _, seen := byISIN[t.ISIN]; !seen {
			isinOrder = append(isinOrder, t.ISIN)
		}
		byISIN[t.ISIN] = append(byISIN[t.ISIN], t)
	}

	// Track latest symbol per ISIN for display
	latestSymbol := make(map[string]string)
	for _, t := range trades {
		existing, ok := latestSymbol[t.ISIN]
		if !ok || t.Date.After(trades[0].Date) {
			_ = existing
			latestSymbol[t.ISIN] = t.Symbol
		}
	}
	// More accurate: iterate all and keep the one with the latest date
	for isin, group := range byISIN {
		latest := group[0]
		for _, t := range group[1:] {
			if t.Date.After(latest.Date) || (t.Date.Equal(latest.Date) && t.Symbol != latest.Symbol) {
				latest = t
			}
		}
		latestSymbol[isin] = latest.Symbol
	}

	var allRealized []RealizedTrade
	var allOpen []OpenPosition
	var summaries []SymbolSummary

	for _, isin := range isinOrder {
		group := byISIN[isin]
		displaySymbol := latestSymbol[isin]

		// Sort by (date ASC, tradeType ASC — "buy" < "sell")
		sort.Slice(group, func(i, j int) bool {
			if !group[i].Date.Equal(group[j].Date) {
				return group[i].Date.Before(group[j].Date)
			}
			return group[i].TradeType < group[j].TradeType
		})

		var queue []buyLot
		var totalBought, totalSold, totalMatched float64

		for _, t := range group {
			if t.TradeType == "buy" {
				queue = append(queue, buyLot{
					date:    t.Date,
					qty:     t.Quantity,
					price:   t.AvgPrice,
					symbol:  t.Symbol,
					isin:    t.ISIN,
					orderID: t.OrderID,
				})
				totalBought += t.Quantity
			} else { // sell
				totalSold += t.Quantity
				remaining := t.Quantity
				for remaining > 0 && len(queue) > 0 {
					lot := &queue[0]
					matchQty := math.Min(lot.qty, remaining)

					realized, err := buildRealizedTrade(
						displaySymbol, isin,
						lot.date, lot.price,
						t.Date, t.AvgPrice,
						matchQty, triIdx,
					)
					if err != nil {
						return nil, nil, nil, fmt.Errorf("enrich %s: %w", displaySymbol, err)
					}
					allRealized = append(allRealized, realized)
					totalMatched += matchQty

					lot.qty -= matchQty
					remaining -= matchQty
					if lot.qty <= 0 {
						queue = queue[1:]
					}
				}
				if remaining > 0 {
					// Stock splits/bonus issues can change the ISIN, so pre-split buys
					// are under the old ISIN while post-split sells use the new one.
					// Without corporate action data we can't match — warn and skip.
					log.Printf("WARNING: %s (ISIN %s): sell of %.0f on %s has %.0f unmatched (likely stock split/bonus)",
						displaySymbol, isin, t.Quantity, t.Date.Format("2006-01-02"), remaining)
				}
			}
		}

		// Remaining queue → open positions
		for _, lot := range queue {
			allOpen = append(allOpen, OpenPosition{
				Symbol:   displaySymbol,
				ISIN:     isin,
				BuyDate:  lot.date,
				Quantity: lot.qty,
				BuyPrice: lot.price,
				Invested: math.Round(lot.qty * lot.price),
			})
		}

		totalOpen := 0.0
		for _, lot := range queue {
			totalOpen += lot.qty
		}

		summaries = append(summaries, SymbolSummary{
			Symbol:        displaySymbol,
			SharesBought:  totalBought,
			SharesSold:    totalSold,
			SharesMatched: totalMatched,
			SharesOpen:    totalOpen,
		})
	}

	return allRealized, allOpen, summaries, nil
}

func buildRealizedTrade(symbol, isin string, buyDate time.Time, buyPrice float64, sellDate time.Time, sellPrice float64, qty float64, triIdx *tri.TRIIndex) (RealizedTrade, error) {
	niftyBuy, err := triIdx.Lookup(buyDate.Format("2006-01-02"))
	if err != nil {
		return RealizedTrade{}, fmt.Errorf("TRI lookup buy %s: %w", buyDate.Format("2006-01-02"), err)
	}
	niftySell, err := triIdx.Lookup(sellDate.Format("2006-01-02"))
	if err != nil {
		return RealizedTrade{}, fmt.Errorf("TRI lookup sell %s: %w", sellDate.Format("2006-01-02"), err)
	}

	invested := qty * buyPrice
	saleValue := qty * sellPrice
	equityGL := saleValue - invested
	niftyReturn := invested * (niftySell/niftyBuy - 1)
	holdDays := int(sellDate.Sub(buyDate).Hours() / 24)

	tradeType := "Short"
	if holdDays > 365 {
		tradeType = "Long"
	}

	return RealizedTrade{
		Symbol:      symbol,
		ISIN:        isin,
		BuyDate:     buyDate,
		SellDate:    sellDate,
		HoldDays:    holdDays,
		Quantity:    qty,
		BuyPrice:    buyPrice,
		SellPrice:   sellPrice,
		Invested:    math.Round(invested),
		SaleValue:   math.Round(saleValue),
		EquityGL:    math.Round(equityGL),
		NiftyBuy:    niftyBuy,
		NiftySell:   niftySell,
		NiftyReturn: math.Round(niftyReturn),
		FY:          fiscalYear(sellDate),
		Type:        tradeType,
	}, nil
}

// fiscalYear returns "FY YYYY-YY" based on the date.
// Apr 1 to Mar 31 → e.g. sell on 2025-01-15 → "FY 2024-25"
func fiscalYear(d time.Time) string {
	y := d.Year()
	if d.Month() < 4 { // Jan-Mar belongs to previous FY
		y--
	}
	return fmt.Sprintf("FY %d-%02d", y, (y+1)%100)
}
