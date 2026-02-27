package cockpit

import (
	"encoding/json"
	"fmt"
	"os"
)

// CockpitJSON is the top-level output structure.
type CockpitJSON struct {
	ClientName    string         `json:"client_name,omitempty"`
	ReportDate    string         `json:"report_date"`
	TaxRate       float64        `json:"tax_rate"`
	LTCGExemption int            `json:"ltcg_exemption"`
	Portfolio  PortfolioData  `json:"portfolio"`
	Stocks     StocksData     `json:"stocks"`
	Summary    SummaryData    `json:"summary"`
	FnOSummary FnOSummaryData `json:"fno_summary"`
}

// PortfolioData holds portfolio-level aggregates.
type PortfolioData struct {
	TotalInvested int                    `json:"total_invested"`
	TotalCurrent  int                    `json:"total_current"`
	ByClass       map[string]ClassBucket `json:"by_class"`
}

// ClassBucket holds per-asset-class aggregates.
type ClassBucket struct {
	Invested int `json:"invested"`
	Current  int `json:"current"`
	Count    int `json:"count"`
}

// StocksData holds the four classification buckets.
type StocksData struct {
	Pass     []StockEntry   `json:"pass"`
	Fail     []StockEntry   `json:"fail"`
	TooEarly []TooEarlyEntry `json:"tooEarly"`
	NoTest   []NoTestEntry   `json:"noTest"`
}

// SummaryData holds summary counts and totals.
type SummaryData struct {
	PassCount    int `json:"pass_count"`
	FailCount    int `json:"fail_count"`
	TooEarlyCount int `json:"too_early_count"`
	NoTestCount  int `json:"no_test_count"`
	TotalSurplus int `json:"total_surplus"`
	TotalDeficit int `json:"total_deficit"`
}

// FnOSummaryData holds F&O attribution summary.
type FnOSummaryData struct {
	TotalAttributed   int            `json:"total_attributed"`
	TotalUnattributed int            `json:"total_unattributed"`
	BySymbol          map[string]int `json:"by_symbol"`
}

// BuildCockpitJSON assembles the output JSON structure.
func BuildCockpitJSON(
	reportDate string,
	lotsBySymbol map[string][]EnrichedLot,
	passList, failList []StockEntry,
	tooEarlyList []TooEarlyEntry,
	noTestList []NoTestEntry,
	cfg *CockpitConfig,
	fnoBySymbol map[string]int,
	totalAttributed, totalUnattributed int,
) *CockpitJSON {
	// Portfolio aggregates
	byClass := map[string]ClassBucket{
		"stock":     {},
		"gold_etf":  {},
		"active_mf": {},
		"index_mf":  {},
	}
	totalInvested := 0
	totalCurrent := 0

	for sym, lots := range lotsBySymbol {
		assetClass, _ := cfg.GetClassification(sym)
		symInvested := 0
		symCurrent := 0
		for _, l := range lots {
			symInvested += l.Invested
			symCurrent += l.CurrentValue
		}
		totalInvested += symInvested
		totalCurrent += symCurrent

		bucket := byClass[assetClass]
		bucket.Invested += symInvested
		bucket.Current += symCurrent
		bucket.Count++
		byClass[assetClass] = bucket
	}

	// Summary
	totalSurplus := 0
	for _, s := range passList {
		totalSurplus += s.Surplus
	}
	totalDeficit := 0
	for _, s := range failList {
		totalDeficit += s.Deficit
	}

	// Ensure non-nil slices for JSON
	if passList == nil {
		passList = []StockEntry{}
	}
	if failList == nil {
		failList = []StockEntry{}
	}
	if tooEarlyList == nil {
		tooEarlyList = []TooEarlyEntry{}
	}
	if noTestList == nil {
		noTestList = []NoTestEntry{}
	}
	if fnoBySymbol == nil {
		fnoBySymbol = map[string]int{}
	}

	return &CockpitJSON{
		ClientName:    cfg.ClientName,
		ReportDate:    reportDate,
		TaxRate:       cfg.ResolveTaxRate(),
		LTCGExemption: 125000,
		Portfolio: PortfolioData{
			TotalInvested: totalInvested,
			TotalCurrent:  totalCurrent,
			ByClass:       byClass,
		},
		Stocks: StocksData{
			Pass:     passList,
			Fail:     failList,
			TooEarly: tooEarlyList,
			NoTest:   noTestList,
		},
		Summary: SummaryData{
			PassCount:     len(passList),
			FailCount:     len(failList),
			TooEarlyCount: len(tooEarlyList),
			NoTestCount:   len(noTestList),
			TotalSurplus:  totalSurplus,
			TotalDeficit:  totalDeficit,
		},
		FnOSummary: FnOSummaryData{
			TotalAttributed:   totalAttributed,
			TotalUnattributed: totalUnattributed,
			BySymbol:          fnoBySymbol,
		},
	}
}

// WriteCockpitJSON writes the cockpit JSON to the given path.
func WriteCockpitJSON(path string, data *CockpitJSON) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cockpit JSON: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
