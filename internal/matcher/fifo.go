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
	Invested    float64 // Quantity x BuyPrice
	SaleValue   float64 // Quantity x SellPrice
	EquityGL    float64 // SaleValue - Invested
	NiftyBuy    float64 // TRI on buy date
	NiftySell   float64 // TRI on sell date
	NiftyReturn float64 // Invested x (NiftySell/NiftyBuy - 1)
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

// Warning represents an unmatched sell (pre-account holding or missing buy data).
type Warning struct {
	Symbol    string
	SellDate  string
	Unmatched float64
	Total     float64
	Message   string
}

// buyLot is an internal struct for the FIFO queue.
type buyLot struct {
	date    time.Time
	qty     float64
	price   float64
	symbol  string
	isin    string
	orderID string
}

// isinSplit maps old ISIN to new ISIN with the split/bonus ratio.
// Ratio means: 1 old share became N new shares.
type isinSplit struct {
	newISIN string
	ratio   float64
}

// Known corporate actions (stock splits / bonus issues / mergers) that changed the ISIN.
var knownSplits = map[string]isinSplit{
	"INE00WC01019": {newISIN: "INE00WC01027", ratio: 5},    // AFFLE bonus 4:1 (5x total)
	"INE935N01012": {newISIN: "INE935N01020", ratio: 5},    // DIXON stock split 1:5
	"INE254N01018": {newISIN: "INE254N01026", ratio: 5},    // HNDFDS stock split 1:5
	"INE239A01016": {newISIN: "INE239A01024", ratio: 10},   // NESTLEIND stock split 1:10
	"INE884A01019": {newISIN: "INE884A01027", ratio: 5},    // VAIBHAVGBL stock split 1:5
	"INE001A01036": {newISIN: "INE040A01034", ratio: 1.68}, // HDFC→HDFCBANK merger 42:25
}

// applySplits adjusts pre-split trades: reassigns old ISIN to new ISIN,
// multiplies quantity by ratio, divides price by ratio.
func applySplits(trades []tradebook.ConsolidatedTrade) []tradebook.ConsolidatedTrade {
	result := make([]tradebook.ConsolidatedTrade, len(trades))
	for i, t := range trades {
		result[i] = t
		if split, ok := knownSplits[t.ISIN]; ok {
			log.Printf("Adjusting %s: ISIN %s -> %s (%.0f:1 split), qty %.0f -> %.0f, price %.2f -> %.2f",
				t.Symbol, t.ISIN, split.newISIN, split.ratio,
				t.Quantity, t.Quantity*split.ratio,
				t.AvgPrice, t.AvgPrice/split.ratio)
			result[i].ISIN = split.newISIN
			result[i].Quantity = math.Floor(t.Quantity * split.ratio)
			result[i].AvgPrice = t.AvgPrice / split.ratio
			result[i].Value = result[i].Quantity * result[i].AvgPrice
		}
	}
	return result
}

// demerger describes a corporate demerger where a parent company spins off a child.
// Parent shares stay under the parent ISIN but cost is reduced.
// New child shares are created with cost = (1 - parentCostPct) of original parent cost.
type demerger struct {
	parentISIN    string
	childISIN     string
	childSymbol   string
	recordDate    time.Time
	parentCostPct float64 // e.g. 0.9532 means parent retains 95.32% of cost
}

// Known demergers.
var knownDemergers = []demerger{
	{
		parentISIN:    "INE002A01018", // RELIANCE
		childISIN:     "INE758E01017", // JIOFIN
		childSymbol:   "JIOFIN",
		recordDate:    time.Date(2023, 7, 20, 0, 0, 0, 0, time.UTC),
		parentCostPct: 0.9532,
	},
}

// applyDemergers performs a partial FIFO on each parent ISIN to find lots held
// at the record date. It reduces parent cost and creates synthetic child buy lots.
func applyDemergers(trades []tradebook.ConsolidatedTrade) []tradebook.ConsolidatedTrade {
	for _, d := range knownDemergers {
		trades = applyOneDemerger(trades, d)
	}
	return trades
}

func applyOneDemerger(trades []tradebook.ConsolidatedTrade, d demerger) []tradebook.ConsolidatedTrade {
	// Collect parent trades sorted by (date, tradeType)
	var parentTrades []tradebook.ConsolidatedTrade
	for _, t := range trades {
		if t.ISIN == d.parentISIN {
			parentTrades = append(parentTrades, t)
		}
	}
	if len(parentTrades) == 0 {
		return trades
	}

	sort.Slice(parentTrades, func(i, j int) bool {
		if !parentTrades[i].Date.Equal(parentTrades[j].Date) {
			return parentTrades[i].Date.Before(parentTrades[j].Date)
		}
		return parentTrades[i].TradeType < parentTrades[j].TradeType
	})

	// Partial FIFO up to the record date to find open lots
	type lot struct {
		date  time.Time
		qty   float64
		price float64
	}
	var queue []lot

	for _, t := range parentTrades {
		if t.Date.After(d.recordDate) {
			break
		}
		if t.TradeType == "buy" {
			queue = append(queue, lot{date: t.Date, qty: t.Quantity, price: t.AvgPrice})
		} else {
			remaining := t.Quantity
			for remaining > 0 && len(queue) > 0 {
				match := math.Min(queue[0].qty, remaining)
				queue[0].qty -= match
				remaining -= match
				if queue[0].qty <= 0 {
					queue = queue[1:]
				}
			}
		}
	}

	if len(queue) == 0 {
		return trades
	}

	// Log what we found
	totalHeld := 0.0
	for _, l := range queue {
		totalHeld += l.qty
	}
	log.Printf("Demerger %s: %.0f parent shares held at record date %s, creating %.0f %s lots",
		d.childSymbol, totalHeld, d.recordDate.Format("2006-01-02"), totalHeld, d.childSymbol)

	// Track which parent buy dates/prices need cost adjustment
	type lotKey struct {
		date  time.Time
		price float64
	}
	heldLots := make(map[lotKey]bool)
	for _, l := range queue {
		heldLots[lotKey{date: l.date, price: l.price}] = true
	}

	// Adjust parent cost for ALL buys (simpler: adjust all parent buys, not just held ones,
	// since the cost split applies to the share itself)
	// Actually, only lots held at record date should be adjusted.
	// Build result with adjusted parent trades and new child trades.
	var result []tradebook.ConsolidatedTrade
	for _, t := range trades {
		if t.ISIN == d.parentISIN && t.TradeType == "buy" {
			k := lotKey{date: t.Date, price: t.AvgPrice}
			if heldLots[k] {
				// Adjust parent cost
				adjusted := t
				adjusted.AvgPrice = t.AvgPrice * d.parentCostPct
				adjusted.Value = adjusted.Quantity * adjusted.AvgPrice
				log.Printf("  %s buy %s: cost %.2f -> %.2f (%.2f%% retained)",
					t.Symbol, t.Date.Format("2006-01-02"), t.AvgPrice, adjusted.AvgPrice, d.parentCostPct*100)
				result = append(result, adjusted)
				continue
			}
		}
		result = append(result, t)
	}

	// Create synthetic child buy lots from held parent lots
	for _, l := range queue {
		childPrice := l.price * (1 - d.parentCostPct)
		child := tradebook.ConsolidatedTrade{
			Symbol:    d.childSymbol,
			ISIN:      d.childISIN,
			Date:      l.date,
			TradeType: "buy",
			Quantity:  l.qty,
			AvgPrice:  childPrice,
			Value:     l.qty * childPrice,
			OrderID:   "demerger",
		}
		log.Printf("  Created %s buy %s: %.0f shares @ %.2f (%.2f%% of parent %.2f)",
			d.childSymbol, l.date.Format("2006-01-02"), l.qty, childPrice, (1-d.parentCostPct)*100, l.price)
		result = append(result, child)
	}

	return result
}

// manualTrade represents a trade missing from the tradebooks (e.g. downloaded
// before the trade appeared, or pre-account holding with known details).
type manualTrade struct {
	symbol    string
	isin      string
	date      time.Time
	tradeType string // "buy" or "sell"
	quantity  float64
	price     float64
}

// Known missing trades not present in tradebook CSVs.
var knownManualTrades = []manualTrade{
	{symbol: "MPHASIS", isin: "INE356A01018", date: time.Date(2022, 1, 27, 0, 0, 0, 0, time.UTC), tradeType: "buy", quantity: 700, price: 3000.00},
	{symbol: "SYNGENE", isin: "INE398R01022", date: time.Date(2022, 6, 30, 0, 0, 0, 0, time.UTC), tradeType: "sell", quantity: 3400, price: 550.00},
	{symbol: "DIVISLAB", isin: "INE361B01024", date: time.Date(2021, 9, 30, 0, 0, 0, 0, time.UTC), tradeType: "buy", quantity: 600, price: 4800.00},
	{symbol: "POWERGRID", isin: "INE752E01010", date: time.Date(2023, 9, 12, 0, 0, 0, 0, time.UTC), tradeType: "buy", quantity: 900, price: 0},
	{symbol: "SUNPHARMA", isin: "INE044A01036", date: time.Date(2023, 6, 28, 0, 0, 0, 0, time.UTC), tradeType: "sell", quantity: 700, price: 1020.00},
	{symbol: "BRITANNIA", isin: "INE216A01030", date: time.Date(2021, 1, 28, 0, 0, 0, 0, time.UTC), tradeType: "sell", quantity: 1000, price: 3600.00},
	{symbol: "ULTRACEMCO", isin: "INE481G01011", date: time.Date(2023, 11, 30, 0, 0, 0, 0, time.UTC), tradeType: "sell", quantity: 100, price: 9000.00},
	{symbol: "NAUKRI", isin: "INE663F01024", date: time.Date(2021, 4, 29, 0, 0, 0, 0, time.UTC), tradeType: "sell", quantity: 750, price: 5000.00},
}

// injectManualTrades adds known missing trades to the trade list.
func injectManualTrades(trades []tradebook.ConsolidatedTrade) []tradebook.ConsolidatedTrade {
	for _, m := range knownManualTrades {
		log.Printf("Injecting manual %s: %s %s %.0f shares @ %.2f", m.tradeType, m.symbol, m.date.Format("2006-01-02"), m.quantity, m.price)
		trades = append(trades, tradebook.ConsolidatedTrade{
			Symbol:    m.symbol,
			ISIN:      m.isin,
			Date:      m.date,
			TradeType: m.tradeType,
			Quantity:  m.quantity,
			AvgPrice:  m.price,
			Value:     m.quantity * m.price,
			OrderID:   "manual",
		})
	}
	return trades
}

// Match performs FIFO matching on consolidated trades, enriches with TRI data,
// and returns realized trades, open positions, and per-symbol summaries.
func Match(trades []tradebook.ConsolidatedTrade, triIdx *tri.TRIIndex) ([]RealizedTrade, []OpenPosition, []SymbolSummary, []Warning, error) {
	// Apply corporate action adjustments and manual entries before grouping
	trades = applySplits(trades)
	trades = applyDemergers(trades)
	trades = injectManualTrades(trades)

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
	for isin, group := range byISIN {
		if len(group) == 0 {
			continue
		}
		latest := group[0]
		for _, t := range group[1:] {
			if t.Date.After(latest.Date) {
				latest = t
			}
		}
		latestSymbol[isin] = latest.Symbol
	}

	var allRealized []RealizedTrade
	var allOpen []OpenPosition
	var summaries []SymbolSummary
	var warnings []Warning

	for _, isin := range isinOrder {
		group := byISIN[isin]
		displaySymbol := latestSymbol[isin]

		// Sort by (date ASC, tradeType ASC -- "buy" < "sell")
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
						return nil, nil, nil, nil, fmt.Errorf("enrich %s: %w", displaySymbol, err)
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
					msg := fmt.Sprintf("%.0f of %.0f shares sold on %s unmatched (pre-account holding or missing buy data)",
						remaining, t.Quantity, t.Date.Format("2006-01-02"))
					log.Printf("WARNING: %s (ISIN %s): %s", displaySymbol, isin, msg)
					warnings = append(warnings, Warning{
						Symbol:    displaySymbol,
						SellDate:  t.Date.Format("2006-01-02"),
						Unmatched: remaining,
						Total:     t.Quantity,
						Message:   msg,
					})
				}
			}
		}

		// Remaining queue -> open positions
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

	return allRealized, allOpen, summaries, warnings, nil
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
// Apr 1 to Mar 31: sell on 2025-01-15 -> "FY 2024-25"
func fiscalYear(d time.Time) string {
	y := d.Year()
	if d.Month() < 4 { // Jan-Mar belongs to previous FY
		y--
	}
	return fmt.Sprintf("FY %d-%02d", y, (y+1)%100)
}
