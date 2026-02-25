package output

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/scorer"
)

// Scorecard is the top-level JSON output structure.
type Scorecard struct {
	GeneratedAt     string              `json:"generated_at"`
	Trades          []TradeJSON         `json:"trades"`
	OpenPositions   []OpenJSON          `json:"open_positions"`
	Warnings        []WarningJSON       `json:"warnings"`
	Summary         SummaryJSON         `json:"summary"`
	DividendSummary *DividendSummJSON   `json:"dividend_summary,omitempty"`
}

// DividendSummJSON is the JSON representation of the dividend summary.
type DividendSummJSON struct {
	TotalDividendIncome int              `json:"total_dividend_income"`
	ByFY                []DividendFYJSON `json:"by_fy"`
}

// DividendFYJSON is a per-FY dividend income total.
type DividendFYJSON struct {
	FY             string `json:"fy"`
	DividendIncome int    `json:"dividend_income"`
}

// WarningJSON represents an unmatched sell in the output.
type WarningJSON struct {
	Ticker    string `json:"ticker"`
	SellDate  string `json:"sell_date"`
	Unmatched int    `json:"unmatched_shares"`
	Total     int    `json:"total_shares"`
	Message   string `json:"message"`
}

// TradeJSON is the JSON representation of a realized trade.
type TradeJSON struct {
	FY           string  `json:"fy"`
	Type         string  `json:"type"`
	Ticker       string  `json:"ticker"`
	BuyDate      string  `json:"buy_date"`
	SellDate     string  `json:"sell_date"`
	HoldDays     int     `json:"hold_days"`
	Quantity     int     `json:"quantity"`
	BuyPrice     float64 `json:"buy_price"`
	SellPrice    float64 `json:"sell_price"`
	Invested     int     `json:"invested"`
	SaleValue    int     `json:"sale_value"`
	EquityGL     int     `json:"equity_gl"`
	NiftyBuyTRI  float64 `json:"nifty_buy_tri"`
	NiftySellTRI float64 `json:"nifty_sell_tri"`
	NiftyReturn  int     `json:"nifty_return"`
	Alpha        int     `json:"alpha"`
}

// OpenJSON is the JSON representation of an open position.
type OpenJSON struct {
	Ticker   string  `json:"ticker"`
	BuyDate  string  `json:"buy_date"`
	Quantity int     `json:"quantity"`
	BuyPrice float64 `json:"buy_price"`
	Invested int     `json:"invested"`
	Note     string  `json:"note"`
}

// SummaryJSON is the JSON representation of the scorecard summary.
type SummaryJSON struct {
	TotalTrades      int          `json:"total_trades"`
	TotalInvested    int          `json:"total_invested"`
	TotalMyReturn    int          `json:"total_my_return"`
	TotalNiftyReturn int          `json:"total_nifty_return"`
	NetAlpha         int          `json:"net_alpha"`
	WinRate          int          `json:"win_rate"`
	ByFY             []FYSummJSON `json:"by_fy"`
}

// FYSummJSON is the JSON representation of a FY summary bucket.
type FYSummJSON struct {
	FY          string `json:"fy"`
	Type        string `json:"type"`
	NumTrades   int    `json:"num_trades"`
	Invested    int    `json:"invested"`
	MyReturn    int    `json:"my_return"`
	NiftyReturn int    `json:"nifty_return"`
	Alpha       int    `json:"alpha"`
}

// WriteJSON serializes the scorecard to a JSON file.
func WriteJSON(path string, trades []matcher.RealizedTrade, open []matcher.OpenPosition, warnings []matcher.Warning, summary scorer.Summary) error {
	sc := Scorecard{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Convert trades
	sc.Trades = make([]TradeJSON, len(trades))
	for i, t := range trades {
		alpha := int(t.EquityGL - t.NiftyReturn)
		sc.Trades[i] = TradeJSON{
			FY:           t.FY,
			Type:         t.Type,
			Ticker:       t.Symbol,
			BuyDate:      t.BuyDate.Format("2006-01-02"),
			SellDate:     t.SellDate.Format("2006-01-02"),
			HoldDays:     t.HoldDays,
			Quantity:     int(t.Quantity),
			BuyPrice:     roundTo2(t.BuyPrice),
			SellPrice:    roundTo2(t.SellPrice),
			Invested:     int(t.Invested),
			SaleValue:    int(t.SaleValue),
			EquityGL:     int(t.EquityGL),
			NiftyBuyTRI:  roundTo2(t.NiftyBuy),
			NiftySellTRI: roundTo2(t.NiftySell),
			NiftyReturn:  int(t.NiftyReturn),
			Alpha:        alpha,
		}
	}

	// Convert open positions
	sc.OpenPositions = make([]OpenJSON, len(open))
	for i, o := range open {
		sc.OpenPositions[i] = OpenJSON{
			Ticker:   o.Symbol,
			BuyDate:  o.BuyDate.Format("2006-01-02"),
			Quantity: int(o.Quantity),
			BuyPrice: roundTo2(o.BuyPrice),
			Invested: int(o.Invested),
			Note:     "No matching sell — still held",
		}
	}

	// Convert warnings
	sc.Warnings = make([]WarningJSON, len(warnings))
	for i, w := range warnings {
		sc.Warnings[i] = WarningJSON{
			Ticker:    w.Symbol,
			SellDate:  w.SellDate,
			Unmatched: int(w.Unmatched),
			Total:     int(w.Total),
			Message:   w.Message,
		}
	}

	// Convert summary
	sc.Summary = SummaryJSON{
		TotalTrades:      summary.TotalTrades,
		TotalInvested:    int(summary.TotalInvested),
		TotalMyReturn:    int(summary.TotalMyReturn),
		TotalNiftyReturn: int(summary.TotalNiftyReturn),
		NetAlpha:         int(summary.NetAlpha),
		WinRate:          summary.WinRate,
	}
	sc.Summary.ByFY = make([]FYSummJSON, len(summary.ByFY))
	for i, fy := range summary.ByFY {
		sc.Summary.ByFY[i] = FYSummJSON{
			FY:          fy.FY,
			Type:        fy.Type,
			NumTrades:   fy.NumTrades,
			Invested:    int(fy.Invested),
			MyReturn:    int(fy.MyReturn),
			NiftyReturn: int(fy.NiftyReturn),
			Alpha:       int(fy.Alpha),
		}
	}

	// Build dividend summary if any trade has dividend income
	sc.DividendSummary = buildDividendSummary(trades)

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// buildDividendSummary aggregates dividend income by FY (based on sell date).
// Returns nil if no dividend income exists.
func buildDividendSummary(trades []matcher.RealizedTrade) *DividendSummJSON {
	totalDiv := 0.0
	byFY := make(map[string]float64)

	for _, t := range trades {
		if t.DividendIncome > 0 {
			totalDiv += t.DividendIncome
			fy := dividendFY(t.SellDate)
			byFY[fy] += t.DividendIncome
		}
	}

	if totalDiv == 0 {
		return nil
	}

	// Sort FYs
	fys := make([]string, 0, len(byFY))
	for fy := range byFY {
		fys = append(fys, fy)
	}
	sort.Strings(fys)

	result := &DividendSummJSON{
		TotalDividendIncome: int(math.Round(totalDiv)),
	}
	for _, fy := range fys {
		result.ByFY = append(result.ByFY, DividendFYJSON{
			FY:             fy,
			DividendIncome: int(math.Round(byFY[fy])),
		})
	}

	return result
}

// dividendFY returns "FY YYYY-YY" based on date (Apr-Mar fiscal year).
func dividendFY(d time.Time) string {
	y := d.Year()
	if d.Month() < 4 {
		y--
	}
	return fmt.Sprintf("FY %d-%02d", y, (y+1)%100)
}

func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
