package cockpit

import (
	"math"
	"sort"
	"time"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
)

// EnrichedLot is an unrealized lot enriched with valuation metrics.
type EnrichedLot struct {
	BuyDate      string  `json:"buy_date"`
	Quantity     int     `json:"quantity"`
	BuyPrice     float64 `json:"buy_price"`
	Invested     int     `json:"invested"`
	CurrentPrice float64 `json:"current_price"`
	CurrentValue int     `json:"current_value"`
	NiftyBuyTRI  float64 `json:"nifty_buy_tri"`
	NiftyNowTRI  float64 `json:"nifty_report_tri"`
	ShadowNifty  int     `json:"shadow_nifty"`
	NiftyCagr    float64 `json:"nifty_cagr"`
	StockCagr    float64 `json:"stock_cagr"`
	DaysHeld     int     `json:"days_held"`
	YearsHeld    float64 `json:"years_held"`
	TooEarly     bool    `json:"too_early"`
	OptionIncome int     `json:"option_income"`
}

// StockEntry represents a stock's aggregate cockpit data.
type StockEntry struct {
	Name         string        `json:"name"`
	Symbol       string        `json:"symbol"`
	CAGR         float64       `json:"cagr"`
	NiftyCagr    float64       `json:"niftyCagr"`
	Hurdle       float64       `json:"hurdle"`
	HurdlePct    float64       `json:"hurdlePct"`
	Invested     int           `json:"invested"`
	Current      int           `json:"current"`
	Nifty        int           `json:"nifty"`
	OptionIncome int           `json:"option_income"`
	Lots         []EnrichedLot `json:"lots"`
	DeepDive     bool          `json:"deep_dive"`
	AssetClass   string        `json:"asset_class"`
	Surplus      int           `json:"surplus,omitempty"`
	Deficit      int           `json:"deficit,omitempty"`
	Cards        *DeepDiveCards `json:"cards,omitempty"`
}

// TooEarlyEntry represents a stock where all lots are held < 1 year.
type TooEarlyEntry struct {
	Name     string        `json:"name"`
	Symbol   string        `json:"symbol"`
	Invested int           `json:"invested"`
	Current  int           `json:"current"`
	GainLoss int           `json:"gain_loss"`
	Lots     []EnrichedLot `json:"lots"`
}

// NoTestEntry represents a stock that doesn't need a hurdle test (e.g. index MF).
type NoTestEntry struct {
	Name     string        `json:"name"`
	Symbol   string        `json:"symbol"`
	Invested int           `json:"invested"`
	Current  int           `json:"current"`
	Lots     []EnrichedLot `json:"lots"`
}

// EnrichLots enriches open positions with valuation metrics.
func EnrichLots(
	positions []matcher.OpenPosition,
	prices map[string]float64,
	niftyTRI interface{ Lookup(string) (float64, error) },
	stockTRI map[string]map[string]float64,
	reportDate string,
	cfg *CockpitConfig,
) map[string][]EnrichedLot {
	reportDt, _ := time.Parse("2006-01-02", reportDate)
	niftyNow, _ := niftyTRI.Lookup(reportDate)

	lotsBySymbol := make(map[string][]EnrichedLot)

	for _, pos := range positions {
		sym := pos.Symbol
		buyDateStr := pos.BuyDate.Format("2006-01-02")

		currentPrice, hasCurrent := prices[sym]
		if !hasCurrent {
			continue
		}

		quantity := int(pos.Quantity)
		invested := int(math.Round(float64(quantity) * pos.BuyPrice))
		currentValue := int(math.Round(float64(quantity) * currentPrice))

		niftyBuy, err := niftyTRI.Lookup(buyDateStr)
		if err != nil {
			// No Nifty TRI for buy date — still include lot with zero Nifty metrics
			niftyBuy = 0
		}

		daysHeld := int(reportDt.Sub(pos.BuyDate).Hours() / 24)
		yearsHeld := float64(daysHeld) / 365.25

		shadowNifty := 0
		niftyCagr := 0.0
		if niftyBuy > 0 {
			shadowNifty = int(math.Round(float64(invested) * (niftyNow / niftyBuy)))
			if yearsHeld > 0 {
				niftyCagr = (math.Pow(niftyNow/niftyBuy, 1/yearsHeld) - 1) * 100
			}
		}

		// Stock CAGR: prefer TRI (dividend-adjusted), fall back to price-only
		stockCagr := 0.0
		if yearsHeld > 0 && invested > 0 {
			if symTRI, ok := stockTRI[sym]; ok && !cfg.IsPriceOnly(sym) {
				triBuy, hasBuy := StockTRILookup(symTRI, buyDateStr)
				triNow, hasNow := StockTRILookup(symTRI, reportDate)
				if hasBuy && hasNow && triBuy > 0 {
					stockCagr = (math.Pow(triNow/triBuy, 1/yearsHeld) - 1) * 100
				} else {
					stockCagr = (math.Pow(float64(currentValue)/float64(invested), 1/yearsHeld) - 1) * 100
				}
			} else {
				stockCagr = (math.Pow(float64(currentValue)/float64(invested), 1/yearsHeld) - 1) * 100
			}
		}

		lotsBySymbol[sym] = append(lotsBySymbol[sym], EnrichedLot{
			BuyDate:      buyDateStr,
			Quantity:     quantity,
			BuyPrice:     round2(pos.BuyPrice),
			Invested:     invested,
			CurrentPrice: round2(currentPrice),
			CurrentValue: currentValue,
			NiftyBuyTRI:  round4(niftyBuy),
			NiftyNowTRI:  round4(niftyNow),
			ShadowNifty:  shadowNifty,
			NiftyCagr:    round2f(niftyCagr),
			StockCagr:    round2f(stockCagr),
			DaysHeld:     daysHeld,
			YearsHeld:    round2f(yearsHeld),
			TooEarly:     yearsHeld < 1.0,
			OptionIncome: 0,
		})
	}

	return lotsBySymbol
}

// AttributeFnO distributes F&O option income to unrealized lots using the
// pseudo-RealizedTrade trick: convert unrealized lots to fake realized trades
// with SellDate = reportDate, combine with actual realized trades, run
// fno.Attribute(), and extract attribution for unrealized lots only.
func AttributeFnO(
	contracts []fno.ContractPnL,
	lotsBySymbol map[string][]EnrichedLot,
	realizedTrades []matcher.RealizedTrade,
	reportDate string,
	stockTRI map[string]map[string]float64,
	cfg *CockpitConfig,
) (fnoBySymbol map[string]int, totalAttributed int, totalUnattributed int) {
	reportDt, _ := time.Parse("2006-01-02", reportDate)

	// Build flat list of pseudo-RealizedTrades from unrealized lots
	var pseudoTrades []matcher.RealizedTrade
	type lotKey struct {
		symbol string
		index  int
	}
	var keys []lotKey

	symbols := sortedKeys(lotsBySymbol)
	for _, sym := range symbols {
		lots := lotsBySymbol[sym]
		for i, lot := range lots {
			buyDt, _ := time.Parse("2006-01-02", lot.BuyDate)
			pseudoTrades = append(pseudoTrades, matcher.RealizedTrade{
				Symbol:   sym,
				BuyDate:  buyDt,
				SellDate: reportDt,
				Quantity: float64(lot.Quantity),
			})
			keys = append(keys, lotKey{sym, i})
		}
	}

	nUnrealized := len(pseudoTrades)

	// Combine: unrealized pseudo-trades first, then actual realized trades
	combined := make([]matcher.RealizedTrade, 0, nUnrealized+len(realizedTrades))
	combined = append(combined, pseudoTrades...)
	combined = append(combined, realizedTrades...)

	// Run attribution
	attribution, unattributed := fno.Attribute(contracts, combined)

	// Extract attribution for unrealized lots only (indices 0..N-1)
	fnoBySymbol = make(map[string]int)
	totalAttr := 0.0
	for idx, income := range attribution {
		if idx < nUnrealized {
			key := keys[idx]
			lots := lotsBySymbol[key.symbol]
			lot := &lots[key.index]
			lot.OptionIncome = int(math.Round(income))
			fnoBySymbol[key.symbol] += lot.OptionIncome
			totalAttr += income

			// Recompute stock CAGR including option income
			// Use TRI-based return + option income when TRI is available
			if lot.YearsHeld > 0 && lot.Invested > 0 {
				var totalReturn float64
				if symTRI, ok := stockTRI[key.symbol]; ok && !cfg.IsPriceOnly(key.symbol) {
					triBuy, hasBuy := StockTRILookup(symTRI, lot.BuyDate)
					triNow, hasNow := StockTRILookup(symTRI, reportDate)
					if hasBuy && hasNow && triBuy > 0 {
						triReturn := float64(lot.Invested) * (triNow / triBuy)
						totalReturn = triReturn + float64(lot.OptionIncome)
					} else {
						totalReturn = float64(lot.CurrentValue) + float64(lot.OptionIncome)
					}
				} else {
					totalReturn = float64(lot.CurrentValue) + float64(lot.OptionIncome)
				}
				ratio := totalReturn / float64(lot.Invested)
				if ratio > 0 {
					lot.StockCagr = round2f((math.Pow(ratio, 1/lot.YearsHeld) - 1) * 100)
				} else {
					lot.StockCagr = -100.0
				}
			}
			lotsBySymbol[key.symbol] = lots
		}
	}

	totalUnattr := 0.0
	for _, u := range unattributed {
		totalUnattr += u.NetPnL
	}

	return fnoBySymbol, int(math.Round(totalAttr)), int(math.Round(totalUnattr))
}

// ClassifyStocks classifies enriched lots into pass/fail/tooEarly/noTest buckets.
func ClassifyStocks(
	lotsBySymbol map[string][]EnrichedLot,
	cfg *CockpitConfig,
	stockTRI map[string]map[string]float64,
	reportDate string,
) (passList, failList []StockEntry, tooEarlyList []TooEarlyEntry, noTestList []NoTestEntry) {
	symbols := sortedKeys(lotsBySymbol)

	for _, sym := range symbols {
		lots := lotsBySymbol[sym]
		if len(lots) == 0 {
			continue
		}

		assetClass, hurdle := cfg.GetClassification(sym)
		name := cfg.DisplayName(sym)

		symInvested := 0
		symCurrent := 0
		symNifty := 0
		for _, l := range lots {
			symInvested += l.Invested
			symCurrent += l.CurrentValue
			symNifty += l.ShadowNifty
		}

		// No test for index MF
		if assetClass == "index_mf" {
			noTestList = append(noTestList, NoTestEntry{
				Name:     name,
				Symbol:   sym,
				Invested: symInvested,
				Current:  symCurrent,
				Lots:     lots,
			})
			continue
		}

		// Separate LTCG lots from too-early lots
		var ltcgLots, earlyLots []EnrichedLot
		for _, l := range lots {
			if l.TooEarly {
				earlyLots = append(earlyLots, l)
			} else {
				ltcgLots = append(ltcgLots, l)
			}
		}

		// If ALL lots are too early
		if len(ltcgLots) == 0 {
			tooEarlyList = append(tooEarlyList, TooEarlyEntry{
				Name:     name,
				Symbol:   sym,
				Invested: symInvested,
				Current:  symCurrent,
				GainLoss: symCurrent - symInvested,
				Lots:     lots,
			})
			continue
		}

		// Aggregate LTCG lots
		ltcgInvested := 0
		ltcgCurrent := 0
		ltcgNifty := 0
		ltcgOptionIncome := 0
		for _, l := range ltcgLots {
			ltcgInvested += l.Invested
			ltcgCurrent += l.CurrentValue
			ltcgNifty += l.ShadowNifty
			ltcgOptionIncome += l.OptionIncome
		}

		// Weighted CAGR
		totalWeight := 0.0
		weightedStockCagr := 0.0
		weightedNiftyCagr := 0.0
		for _, l := range ltcgLots {
			w := float64(l.Invested)
			totalWeight += w
			weightedStockCagr += l.StockCagr * w
			weightedNiftyCagr += l.NiftyCagr * w
		}
		if totalWeight > 0 {
			weightedStockCagr /= totalWeight
			weightedNiftyCagr /= totalWeight
		}

		// Hurdle test: compare stock return (current + F&O) to nifty + hurdle
		hurdleNifty := 0.0
		for _, l := range ltcgLots {
			hurdleNifty += float64(l.Invested) * math.Pow(1+(l.NiftyCagr+hurdle)/100, l.YearsHeld)
		}
		hurdleSurplus := (ltcgCurrent + ltcgOptionIncome) - int(math.Round(hurdleNifty))

		entry := StockEntry{
			Name:         name,
			Symbol:       sym,
			CAGR:         round1(weightedStockCagr),
			NiftyCagr:    round1(weightedNiftyCagr),
			Hurdle:       round1(weightedNiftyCagr + hurdle),
			HurdlePct:    hurdle,
			Invested:     ltcgInvested,
			Current:      ltcgCurrent,
			Nifty:        ltcgNifty,
			OptionIncome: ltcgOptionIncome,
			Lots:         lots,
			DeepDive:     false,
			AssetClass:   assetClass,
		}

		if hurdleSurplus >= 0 {
			entry.Surplus = hurdleSurplus
			passList = append(passList, entry)
		} else {
			entry.Deficit = -hurdleSurplus
			failList = append(failList, entry)
		}
	}

	// Sort: fail by deficit desc, pass by surplus desc
	sort.Slice(failList, func(i, j int) bool {
		return failList[i].Deficit > failList[j].Deficit
	})
	sort.Slice(passList, func(i, j int) bool {
		return passList[i].Surplus > passList[j].Surplus
	})

	return
}

func sortedKeys(m map[string][]EnrichedLot) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func round2f(v float64) float64 {
	return math.Round(v*100) / 100
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
