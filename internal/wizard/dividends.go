package wizard

import (
	"fmt"
	"math"
	"sort"
	"time"

	"stock-scorecard/internal/dividend"
)

// DividendResult holds the outcome of dividend reconciliation.
type DividendResult struct {
	Dividends []dividend.FetchedDividend
	Confirmed bool
}

// ReconcileDividends fetches dividends for the given tickers and asks the
// user to confirm the totals at FY level.
func (w *Wizard) ReconcileDividends(tickers []string) *DividendResult {
	w.printf("  Fetching dividends for %d tickers ", len(tickers))

	divs, err := dividend.Fetch(tickers)
	if err != nil {
		w.printf("failed\n")
		w.printf("  ⚠ Could not fetch dividends: %v\n", err)
		w.printf("  You can provide dividends manually via --dividends flag.\n\n")
		return nil
	}

	w.printf("done (%d events)\n\n", len(divs))

	if len(divs) == 0 {
		w.printf("  No dividend data found.\n\n")
		return &DividendResult{Confirmed: true}
	}

	// Group by FY and compute totals
	type fySummary struct {
		fy     string
		total  float64
		stocks int
	}

	fyMap := make(map[string]map[string]float64) // fy → symbol → total
	for _, d := range divs {
		fy := dividendFY(d.ExDate)
		if fyMap[fy] == nil {
			fyMap[fy] = make(map[string]float64)
		}
		fyMap[fy][d.Symbol] += d.Amount
	}

	var summaries []fySummary
	for fy, symbols := range fyMap {
		total := 0.0
		for _, amt := range symbols {
			total += amt
		}
		summaries = append(summaries, fySummary{
			fy:     fy,
			total:  total,
			stocks: len(symbols),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].fy < summaries[j].fy
	})

	for _, s := range summaries {
		w.printf("  %s:  ₹%s  (%d stocks)\n", s.fy, formatLakhs(math.Round(s.total)), s.stocks)
	}

	w.printf("\n  Does this match your records? [Y/n] → ")
	choice := w.readChoice("yn")

	if choice == 'y' {
		w.printf("  ✓ Dividends confirmed.\n\n")
		return &DividendResult{Dividends: divs, Confirmed: true}
	}

	// User said no — show per-stock details
	w.printf("\n  Per-stock breakdown:\n")
	type stockDiv struct {
		symbol string
		total  float64
		count  int
	}
	symbolMap := make(map[string]*stockDiv)
	for _, d := range divs {
		sd, ok := symbolMap[d.Symbol]
		if !ok {
			sd = &stockDiv{symbol: d.Symbol}
			symbolMap[d.Symbol] = sd
		}
		sd.total += d.Amount
		sd.count++
	}
	var stocks []stockDiv
	for _, sd := range symbolMap {
		stocks = append(stocks, *sd)
	}
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].total > stocks[j].total // highest first
	})

	for _, s := range stocks {
		w.printf("    %-20s ₹%.2f  (%d events)\n", s.symbol, s.total, s.count)
	}

	w.printf("\n  Accept these dividends anyway? [Y/n] → ")
	choice2 := w.readChoice("yn")
	if choice2 == 'y' {
		w.printf("  ✓ Dividends accepted.\n\n")
		return &DividendResult{Dividends: divs, Confirmed: true}
	}

	w.printf("  ✓ Dividends skipped. You can provide a corrected CSV via --dividends.\n\n")
	return &DividendResult{Dividends: divs, Confirmed: false}
}

// dividendFY returns the fiscal year string for a date (Apr-Mar).
func dividendFY(d time.Time) string {
	y := d.Year()
	if d.Month() < 4 {
		y--
	}
	return fmt.Sprintf("FY %d-%02d", y, (y+1)%100)
}
