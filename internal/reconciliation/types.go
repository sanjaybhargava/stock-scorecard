// Package reconciliation holds per-client corporate action data (splits,
// demergers, manual trades, F&O renames) that was previously hardcoded.
// Data is serialized to/from a JSON file per client.
package reconciliation

import "time"

// ReconciliationData holds all per-client corporate action adjustments.
type ReconciliationData struct {
	ClientID     string            `json:"client_id"`
	Splits       []Split           `json:"splits"`
	Demergers    []Demerger        `json:"demergers"`
	ManualTrades []ManualTrade     `json:"manual_trades"`
	FnORenames   map[string]string `json:"fno_renames"`
}

// Split represents a stock split or bonus issue that changed the ISIN.
// Ratio means: 1 old share became Ratio new shares.
type Split struct {
	OldISIN string  `json:"old_isin"`
	NewISIN string  `json:"new_isin"`
	Ratio   float64 `json:"ratio"`
	Note    string  `json:"note,omitempty"`
}

// Demerger represents a corporate demerger where a parent spins off a child.
type Demerger struct {
	ParentISIN    string  `json:"parent_isin"`
	ChildISIN     string  `json:"child_isin"`
	ChildSymbol   string  `json:"child_symbol"`
	RecordDate    string  `json:"record_date"` // YYYY-MM-DD
	ParentCostPct float64 `json:"parent_cost_pct"`
}

// ManualTrade represents a trade missing from tradebook CSVs.
type ManualTrade struct {
	Symbol    string  `json:"symbol"`
	ISIN      string  `json:"isin"`
	Date      string  `json:"date"`       // YYYY-MM-DD
	TradeType string  `json:"trade_type"` // "buy" or "sell"
	Quantity  float64 `json:"quantity"`
	Price     float64 `json:"price"`
}

// ParseRecordDate parses a Demerger's RecordDate string to time.Time.
func (d Demerger) ParseRecordDate() (time.Time, error) {
	return time.Parse("2006-01-02", d.RecordDate)
}

// ParseDate parses a ManualTrade's Date string to time.Time.
func (m ManualTrade) ParseDate() (time.Time, error) {
	return time.Parse("2006-01-02", m.Date)
}

// SplitsMap converts the Splits slice into a map keyed by OldISIN for
// efficient lookup during FIFO matching.
func (r *ReconciliationData) SplitsMap() map[string]Split {
	m := make(map[string]Split, len(r.Splits))
	for _, s := range r.Splits {
		m[s.OldISIN] = s
	}
	return m
}
