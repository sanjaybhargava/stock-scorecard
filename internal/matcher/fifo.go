package matcher

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"stock-scorecard/internal/dividend"
	"stock-scorecard/internal/reconciliation"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

// RealizedTrade represents a matched buy-sell pair.
type RealizedTrade struct {
	Symbol         string
	ISIN           string
	BuyDate        time.Time
	SellDate       time.Time
	HoldDays       int
	Quantity       float64
	BuyPrice       float64
	SellPrice      float64
	Invested       float64   // Quantity x BuyPrice
	SaleValue      float64   // Quantity x SellPrice
	EquityGL       float64   // SaleValue - Invested + DividendIncome + OptionIncome
	DividendIncome float64   // dividend income during holding period (also included in EquityGL)
	OptionIncome   float64   // F&O option income attributed to this trade (also included in EquityGL)
	NiftyBuy       float64   // TRI on buy date
	NiftySell      float64   // TRI on sell date
	NiftyReturn    float64   // Invested x (NiftySell/NiftyBuy - 1)
	FY             string    // FY of sell date, e.g. "FY 2024-25"
	Type           string    // "Long" if HoldDays > 365, else "Short"
	Tier           MatchTier // 1=exact, 2=intelligent, 3=skipped
	TierReason     string    // e.g. "transfer_in", "rename:OLD→NEW"
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
	ISIN      string
	SellDate  string
	SellPrice float64
	Unmatched float64
	Total     float64
	Message   string
}

// MatchTier indicates how a realized trade was matched.
type MatchTier int

const (
	TierExact       MatchTier = 1 // Standard FIFO match
	TierIntelligent MatchTier = 2 // Auto-matched (e.g. transfer-in detection)
	TierSkipped     MatchTier = 3 // Unresolved — needs manual input
)

// buyLot is an internal struct for the FIFO queue.
type buyLot struct {
	date    time.Time
	qty     float64
	price   float64
	symbol  string
	isin    string
	orderID string
}

// applySplits adjusts pre-split trades: reassigns old ISIN to new ISIN,
// multiplies quantity by ratio, divides price by ratio.
func applySplits(trades []tradebook.ConsolidatedTrade, splits map[string]reconciliation.Split) []tradebook.ConsolidatedTrade {
	if len(splits) == 0 {
		return trades
	}
	result := make([]tradebook.ConsolidatedTrade, len(trades))
	for i, t := range trades {
		result[i] = t
		if split, ok := splits[t.ISIN]; ok {
			log.Printf("Adjusting %s: ISIN %s -> %s (%.2f:1 split), qty %.0f -> %.0f, price %.2f -> %.2f",
				t.Symbol, t.ISIN, split.NewISIN, split.Ratio,
				t.Quantity, t.Quantity*split.Ratio,
				t.AvgPrice, t.AvgPrice/split.Ratio)
			result[i].ISIN = split.NewISIN
			result[i].Quantity = math.Floor(t.Quantity * split.Ratio)
			result[i].AvgPrice = t.AvgPrice / split.Ratio
			result[i].Value = result[i].Quantity * result[i].AvgPrice
		}
	}
	return result
}

// applyDemergers performs a partial FIFO on each parent ISIN to find lots held
// at the record date. It reduces parent cost and creates synthetic child buy lots.
func applyDemergers(trades []tradebook.ConsolidatedTrade, demergers []reconciliation.Demerger) []tradebook.ConsolidatedTrade {
	for _, d := range demergers {
		recordDate, err := d.ParseRecordDate()
		if err != nil {
			log.Printf("WARNING: skipping demerger %s: invalid record_date %q: %v", d.ChildSymbol, d.RecordDate, err)
			continue
		}
		trades = applyOneDemerger(trades, d.ParentISIN, d.ChildISIN, d.ChildSymbol, recordDate, d.ParentCostPct)
	}
	return trades
}

func applyOneDemerger(trades []tradebook.ConsolidatedTrade, parentISIN, childISIN, childSymbol string, recordDate time.Time, parentCostPct float64) []tradebook.ConsolidatedTrade {
	// Collect parent trades sorted by (date, tradeType)
	var parentTrades []tradebook.ConsolidatedTrade
	for _, t := range trades {
		if t.ISIN == parentISIN {
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
		if t.Date.After(recordDate) {
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
		childSymbol, totalHeld, recordDate.Format("2006-01-02"), totalHeld, childSymbol)

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
		if t.ISIN == parentISIN && t.TradeType == "buy" {
			k := lotKey{date: t.Date, price: t.AvgPrice}
			if heldLots[k] {
				// Adjust parent cost
				adjusted := t
				adjusted.AvgPrice = t.AvgPrice * parentCostPct
				adjusted.Value = adjusted.Quantity * adjusted.AvgPrice
				log.Printf("  %s buy %s: cost %.2f -> %.2f (%.2f%% retained)",
					t.Symbol, t.Date.Format("2006-01-02"), t.AvgPrice, adjusted.AvgPrice, parentCostPct*100)
				result = append(result, adjusted)
				continue
			}
		}
		result = append(result, t)
	}

	// Create synthetic child buy lots from held parent lots
	for _, l := range queue {
		childPrice := l.price * (1 - parentCostPct)
		child := tradebook.ConsolidatedTrade{
			Symbol:    childSymbol,
			ISIN:      childISIN,
			Date:      l.date,
			TradeType: "buy",
			Quantity:  l.qty,
			AvgPrice:  childPrice,
			Value:     l.qty * childPrice,
			OrderID:   "demerger",
		}
		log.Printf("  Created %s buy %s: %.0f shares @ %.2f (%.2f%% of parent %.2f)",
			childSymbol, l.date.Format("2006-01-02"), l.qty, childPrice, (1-parentCostPct)*100, l.price)
		result = append(result, child)
	}

	return result
}

// injectManualTrades adds known missing trades to the trade list.
func injectManualTrades(trades []tradebook.ConsolidatedTrade, manuals []reconciliation.ManualTrade) []tradebook.ConsolidatedTrade {
	for _, m := range manuals {
		d, err := m.ParseDate()
		if err != nil {
			log.Printf("WARNING: skipping manual trade %s: invalid date %q: %v", m.Symbol, m.Date, err)
			continue
		}
		log.Printf("Injecting manual %s: %s %s %.0f shares @ %.2f", m.TradeType, m.Symbol, d.Format("2006-01-02"), m.Quantity, m.Price)
		trades = append(trades, tradebook.ConsolidatedTrade{
			Symbol:    m.Symbol,
			ISIN:      m.ISIN,
			Date:      d,
			TradeType: m.TradeType,
			Quantity:  m.Quantity,
			AvgPrice:  m.Price,
			Value:     m.Quantity * m.Price,
			OrderID:   "manual",
		})
	}
	return trades
}

// Match performs FIFO matching on consolidated trades, enriches with TRI data,
// and returns realized trades, open positions, and per-symbol summaries.
// If recon is nil, no corporate action adjustments or manual trades are applied.
func Match(trades []tradebook.ConsolidatedTrade, triIdx *tri.TRIIndex, divIdx *dividend.DividendIndex, recon *reconciliation.ReconciliationData) ([]RealizedTrade, []OpenPosition, []SymbolSummary, []Warning, error) {
	// Apply corporate action adjustments and manual entries before grouping
	if recon != nil {
		trades = applySplits(trades, recon.SplitsMap())
		trades = applyDemergers(trades, recon.Demergers)
		trades = injectManualTrades(trades, recon.ManualTrades)
	}

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
						matchQty, triIdx, divIdx,
						TierExact, "",
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
					warnings = append(warnings, Warning{
						Symbol:    displaySymbol,
						ISIN:      isin,
						SellDate:  t.Date.Format("2006-01-02"),
						SellPrice: t.AvgPrice,
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

func buildRealizedTrade(symbol, isin string, buyDate time.Time, buyPrice float64, sellDate time.Time, sellPrice float64, qty float64, triIdx *tri.TRIIndex, divIdx *dividend.DividendIndex, tier MatchTier, tierReason string) (RealizedTrade, error) {
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

	var dividendIncome float64
	if divIdx != nil {
		divPerShare := divIdx.Lookup(symbol, buyDate, sellDate)
		dividendIncome = qty * divPerShare
		equityGL += dividendIncome
		if dividendIncome > 0 {
			log.Printf("  %s: +%.0f dividend (%.0f shares x %.2f/share)", symbol, dividendIncome, qty, divPerShare)
		}
	}

	tradeType := "Short"
	if holdDays > 365 {
		tradeType = "Long"
	}

	return RealizedTrade{
		Symbol:         symbol,
		ISIN:           isin,
		BuyDate:        buyDate,
		SellDate:       sellDate,
		HoldDays:       holdDays,
		Quantity:       qty,
		BuyPrice:       buyPrice,
		SellPrice:      sellPrice,
		Invested:       math.Round(invested),
		SaleValue:      math.Round(saleValue),
		EquityGL:       math.Round(equityGL),
		DividendIncome: math.Round(dividendIncome),
		NiftyBuy:       niftyBuy,
		NiftySell:      niftySell,
		NiftyReturn:    math.Round(niftyReturn),
		FY:             fiscalYear(sellDate),
		Type:           tradeType,
		Tier:           tier,
		TierReason:     tierReason,
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
