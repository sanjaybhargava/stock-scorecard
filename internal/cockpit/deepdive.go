package cockpit

import (
	"fmt"
	"log"
	"math"
	"time"
)

// DeepDiveCards holds the deep-dive data for a stock.
type DeepDiveCards struct {
	Phases       []PhaseData       `json:"phases"`
	Drawdowns    []DrawdownData    `json:"drawdowns"`
	Redeployment *RedeploymentCard `json:"redeployment"`
	Terminal     *TerminalCard     `json:"terminal"`
}

// PhaseData holds performance data for one market phase.
type PhaseData struct {
	Regime    string  `json:"regime"`
	Icon      string  `json:"icon"`
	Period    string  `json:"period"`
	StockCagr float64 `json:"stockCagr"`
	NiftyCagr float64 `json:"niftyCagr"`
	Listed    bool    `json:"listed"`
}

// DrawdownData holds max drawdown data for one market phase.
type DrawdownData struct {
	Regime  string  `json:"regime"`
	Icon    string  `json:"icon"`
	Period  string  `json:"period"`
	StockDD float64 `json:"stockDD"`
	NiftyDD float64 `json:"niftyDD"`
	Listed  bool    `json:"listed"`
}

// RedeploymentCard holds data for the redeployment analysis card.
type RedeploymentCard struct {
	Name          string  `json:"name"`
	Shares        int     `json:"shares"`
	Price         float64 `json:"price"`
	CostPerShare  float64 `json:"costPerShare"`
	TaxRate       float64 `json:"taxRate"`
	LTCGExemption int     `json:"ltcgExemption"`
}

// TerminalCard holds data for the terminal value analysis card.
type TerminalCard struct {
	Name          string  `json:"name"`
	Ticker        string  `json:"ticker"`
	TotalShares   int     `json:"totalShares"`
	Price         float64 `json:"price"`
	CostPerShare  float64 `json:"costPerShare"`
	TaxRate       float64 `json:"taxRate"`
	LTCGExemption int     `json:"ltcgExemption"`
	NiftyCagr     float64 `json:"niftyCagr"`
	HcCagr        float64 `json:"hcCagr"`
	NiftyPct      float64 `json:"niftyPct"`
	HcPct         float64 `json:"hcPct"`
	Years         int     `json:"years"`
}

// BuildDeepDiveCards builds deep-dive analysis cards for stocks that failed the hurdle test.
func BuildDeepDiveCards(
	failList []StockEntry,
	lotsBySymbol map[string][]EnrichedLot,
	prices map[string]float64,
	pricer *Pricer,
	niftyTRI interface{ Lookup(string) (float64, error) },
	stockTRI map[string]map[string]float64,
	cfg *CockpitConfig,
) {
	if len(cfg.MarketPhases) == 0 {
		log.Printf("No market phases configured, skipping deep-dive")
		return
	}

	log.Printf("Building deep-dive data for %d failed stocks", len(failList))

	// Determine which symbols need historical data
	var needHistorical []string
	for i := range failList {
		if failList[i].AssetClass != "stock" {
			continue
		}
		needHistorical = append(needHistorical, failList[i].Symbol)
	}

	// Fetch historical prices for all failed stocks
	var historicalPrices map[string]map[string]float64
	if len(needHistorical) > 0 {
		historicalPrices = pricer.FetchAllHistorical(
			needHistorical, "2016-01-01", cfg.ReportDate, cfg,
		)
	}

	for i := range failList {
		stock := &failList[i]
		if stock.AssetClass != "stock" {
			stock.DeepDive = false
			continue
		}

		sym := stock.Symbol
		stockPrices, ok := historicalPrices[sym]
		if !ok || len(stockPrices) < 100 {
			stock.DeepDive = false
			continue
		}

		stock.DeepDive = true

		// Compute phase data
		phases, drawdowns := computePhaseData(stockPrices, niftyTRI, stockTRI, sym, cfg)

		// Compute aggregate for redeployment/terminal cards
		lots := lotsBySymbol[sym]
		totalShares := 0
		totalInvested := 0
		for _, l := range lots {
			totalShares += l.Quantity
			totalInvested += l.Invested
		}
		currentPrice := prices[sym]
		costPerShare := 0.0
		if totalShares > 0 {
			costPerShare = float64(totalInvested) / float64(totalShares)
		}

		taxRate := cfg.ResolveTaxRate()
		stock.Cards = &DeepDiveCards{
			Phases:    phases,
			Drawdowns: drawdowns,
			Redeployment: &RedeploymentCard{
				Name:          stock.Name,
				Shares:        totalShares,
				Price:         round2(currentPrice),
				CostPerShare:  round2(costPerShare),
				TaxRate:       taxRate,
				LTCGExemption: 125000,
			},
			Terminal: &TerminalCard{
				Name:          stock.Name,
				Ticker:        sym,
				TotalShares:   totalShares,
				Price:         round2(currentPrice),
				CostPerShare:  round2(costPerShare),
				TaxRate:       taxRate,
				LTCGExemption: 125000,
				NiftyCagr:    16.2,
				HcCagr:       25,
				NiftyPct:     80,
				HcPct:        20,
				Years:        5,
			},
		}
	}
}

// computePhaseData computes phase performance and drawdown data for a stock.
func computePhaseData(
	stockPrices map[string]float64,
	niftyTRI interface{ Lookup(string) (float64, error) },
	stockTRIData map[string]map[string]float64,
	symbol string,
	cfg *CockpitConfig,
) ([]PhaseData, []DrawdownData) {
	var phases []PhaseData
	var drawdowns []DrawdownData

	for _, phase := range cfg.MarketPhases {
		start := phase.Start
		end := phase.End
		dtStart, _ := time.Parse("2006-01-02", start)
		dtEnd, _ := time.Parse("2006-01-02", end)
		years := float64(dtEnd.Sub(dtStart).Hours()) / (24 * 365.25)

		icon := phaseIcon(phase.Regime)
		period := fmt.Sprintf("%s → %s", fmtDate(start), fmtDate(end))

		// Check if we have price data covering this phase
		phaseDates := SortedDatesInRange(stockPrices, start, end)
		listed := len(phaseDates) >= 20

		if !listed {
			phases = append(phases, PhaseData{
				Regime: phase.Regime, Icon: icon, Period: period,
				StockCagr: 0, NiftyCagr: 0, Listed: false,
			})
			drawdowns = append(drawdowns, DrawdownData{
				Regime: phase.Regime, Icon: icon, Period: period,
				StockDD: 0, NiftyDD: 0, Listed: false,
			})
			continue
		}

		// Stock CAGR — prefer TRI, fall back to price-only
		stockCagr := 0.0
		usedTRI := false
		if symTRI, ok := stockTRIData[symbol]; ok && !cfg.IsPriceOnly(symbol) {
			triStart, hasStart := StockTRILookup(symTRI, start)
			triEnd, hasEnd := StockTRILookup(symTRI, end)
			if hasStart && hasEnd && triStart > 0 {
				stockCagr = computeCAGR(triStart, triEnd, years)
				usedTRI = true
			}
		}
		if !usedTRI {
			spStart, hasStart := NearestPrice(stockPrices, start, "forward")
			spEnd, hasEnd := NearestPrice(stockPrices, end, "back")
			if hasStart && hasEnd && spStart > 0 {
				stockCagr = computeCAGR(spStart, spEnd, years)
			}
		}

		// Nifty CAGR
		niftyCagr := 0.0
		ntStart, errS := niftyTRI.Lookup(start)
		ntEnd, errE := niftyTRI.Lookup(end)
		if errS == nil && errE == nil && ntStart > 0 {
			niftyCagr = computeCAGR(ntStart, ntEnd, years)
		}

		phases = append(phases, PhaseData{
			Regime: phase.Regime, Icon: icon, Period: period,
			StockCagr: round1(stockCagr), NiftyCagr: round1(niftyCagr), Listed: true,
		})

		// Drawdowns
		stockDD := computeMaxDrawdown(stockPrices, start, end)
		niftyDD := computeNiftyDrawdown(niftyTRI, start, end)

		drawdowns = append(drawdowns, DrawdownData{
			Regime: phase.Regime, Icon: icon, Period: period,
			StockDD: stockDD, NiftyDD: niftyDD, Listed: true,
		})
	}

	return phases, drawdowns
}

// computeCAGR computes CAGR as a percentage.
func computeCAGR(startVal, endVal, years float64) float64 {
	if startVal <= 0 || years <= 0 {
		return 0
	}
	return (math.Pow(endVal/startVal, 1/years) - 1) * 100
}

// computeMaxDrawdown computes the max drawdown (%) for prices in [start, end].
func computeMaxDrawdown(prices map[string]float64, start, end string) float64 {
	dates := SortedDatesInRange(prices, start, end)
	if len(dates) < 2 {
		return 0
	}
	peak := 0.0
	maxDD := 0.0
	for _, d := range dates {
		p := prices[d]
		if p > peak {
			peak = p
		}
		if peak > 0 {
			dd := (peak - p) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return round1(maxDD)
}

// computeNiftyDrawdown computes Nifty max drawdown from TRI values in [start, end].
func computeNiftyDrawdown(niftyTRI interface{ Lookup(string) (float64, error) }, start, end string) float64 {
	// Generate daily dates and look up TRI
	dtStart, _ := time.Parse("2006-01-02", start)
	dtEnd, _ := time.Parse("2006-01-02", end)

	peak := 0.0
	maxDD := 0.0
	for d := dtStart; !d.After(dtEnd); d = d.AddDate(0, 0, 1) {
		v, err := niftyTRI.Lookup(d.Format("2006-01-02"))
		if err != nil {
			continue
		}
		if v > peak {
			peak = v
		}
		if peak > 0 {
			dd := (peak - v) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return round1(maxDD)
}

func phaseIcon(regime string) string {
	switch regime {
	case "Bull":
		return "↗"
	case "Bear":
		return "↘"
	case "Sideways":
		return "→"
	default:
		return "?"
	}
}

func fmtDate(dateStr string) string {
	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return dt.Format("Jan 2006")
}
