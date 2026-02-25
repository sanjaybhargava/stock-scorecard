package scorer

import (
	"math"
	"sort"

	"stock-scorecard/internal/matcher"
)

// FYSummary holds aggregated metrics for a (FY, Type) bucket.
type FYSummary struct {
	FY          string
	Type        string
	NumTrades   int
	Invested    float64
	MyReturn    float64 // total EquityGL
	NiftyReturn float64
	Alpha       float64
}

// Summary holds the overall scorecard summary.
type Summary struct {
	TotalTrades      int
	TotalInvested    float64
	TotalMyReturn    float64
	TotalNiftyReturn float64
	NetAlpha         float64
	WinRate          int // percentage, 0-100
	ByFY             []FYSummary
}

// Score computes the scorecard summary from realized trades.
func Score(trades []matcher.RealizedTrade) Summary {
	// Group by (FY, Type)
	type key struct {
		FY   string
		Type string
	}

	groups := make(map[key]*FYSummary)
	var keyOrder []key

	for _, t := range trades {
		k := key{FY: t.FY, Type: t.Type}
		s, ok := groups[k]
		if !ok {
			s = &FYSummary{FY: t.FY, Type: t.Type}
			groups[k] = s
			keyOrder = append(keyOrder, k)
		}
		s.NumTrades++
		s.Invested += t.Invested
		s.MyReturn += t.EquityGL
		s.NiftyReturn += t.NiftyReturn
	}

	// Compute alpha per bucket
	for _, s := range groups {
		s.Alpha = s.MyReturn - s.NiftyReturn
	}

	// Sort by FY then Type
	sort.Slice(keyOrder, func(i, j int) bool {
		if keyOrder[i].FY != keyOrder[j].FY {
			return keyOrder[i].FY < keyOrder[j].FY
		}
		return keyOrder[i].Type < keyOrder[j].Type
	})

	byFY := make([]FYSummary, len(keyOrder))
	for i, k := range keyOrder {
		s := groups[k]
		byFY[i] = FYSummary{
			FY:          s.FY,
			Type:        s.Type,
			NumTrades:   s.NumTrades,
			Invested:    math.Round(s.Invested),
			MyReturn:    math.Round(s.MyReturn),
			NiftyReturn: math.Round(s.NiftyReturn),
			Alpha:       math.Round(s.Alpha),
		}
	}

	// Win rate: % of unique (ticker, FY, type) combos with aggregate alpha >= 0
	type tickerKey struct {
		Symbol string
		FY     string
		Type   string
	}
	tickerAlpha := make(map[tickerKey]float64)
	for _, t := range trades {
		k := tickerKey{Symbol: t.Symbol, FY: t.FY, Type: t.Type}
		alpha := t.EquityGL - t.NiftyReturn
		tickerAlpha[k] += alpha
	}

	wins := 0
	for _, alpha := range tickerAlpha {
		if alpha >= 0 {
			wins++
		}
	}
	winRate := 0
	if len(tickerAlpha) > 0 {
		winRate = int(math.Round(float64(wins) / float64(len(tickerAlpha)) * 100))
	}

	// Totals
	var totalInvested, totalMyReturn, totalNiftyReturn float64
	for _, s := range byFY {
		totalInvested += s.Invested
		totalMyReturn += s.MyReturn
		totalNiftyReturn += s.NiftyReturn
	}

	return Summary{
		TotalTrades:      len(trades),
		TotalInvested:    math.Round(totalInvested),
		TotalMyReturn:    math.Round(totalMyReturn),
		TotalNiftyReturn: math.Round(totalNiftyReturn),
		NetAlpha:         math.Round(totalMyReturn - totalNiftyReturn),
		WinRate:          winRate,
		ByFY:             byFY,
	}
}
