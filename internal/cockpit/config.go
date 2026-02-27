package cockpit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CockpitConfig holds per-client configuration for the cockpit subcommand.
type CockpitConfig struct {
	ClientID   string `json:"client_id"`
	ClientName string `json:"client_name,omitempty"`
	ReportDate string `json:"report_date"`

	// Yahoo Finance ticker overrides (e.g. "MINDTREE" → "LTIM.NS")
	TickerMap map[string]string `json:"ticker_map"`

	// Asset class + hurdle overrides (e.g. "GOLDBEES" → gold_etf, 3%)
	Classifications map[string]Classification `json:"classifications"`

	// Human-readable display names
	DisplayNames map[string]string `json:"display_names"`

	// Symbols where stock TRI is not meaningful — use price-only CAGR
	PriceOnlySymbols []string `json:"price_only_symbols"`

	// Manual lots to inject (e.g. transfer-in holdings)
	ManualLots []ManualLot `json:"manual_lots"`

	// Market regime phases for deep-dive analysis
	MarketPhases []MarketPhase `json:"market_phases"`

	// Hurdle above Nifty CAGR for stocks (default 3%)
	DefaultHurdlePct float64 `json:"default_hurdle_pct"`

	// Expected total income for auto-computing LTCG tax rate.
	// If set (> 0), overrides TaxRate with the computed effective rate.
	ExpectedTotalIncome float64 `json:"expected_total_income,omitempty"`

	// Tax rate for redeployment/terminal value cards.
	// Auto-computed from ExpectedTotalIncome if that field is set.
	TaxRate float64 `json:"tax_rate"`
}

// Classification defines the asset class and hurdle for a symbol.
type Classification struct {
	AssetClass string  `json:"asset_class"` // "stock", "gold_etf", "active_mf", "index_mf"
	HurdlePct  float64 `json:"hurdle_pct"`
}

// ManualLot represents a manually-entered buy lot (e.g. transfer-in).
type ManualLot struct {
	Symbol   string  `json:"symbol"`
	ISIN     string  `json:"isin"`
	BuyDate  string  `json:"buy_date"`
	Quantity int     `json:"quantity"`
	BuyPrice float64 `json:"buy_price"`
}

// MarketPhase defines a market regime period.
type MarketPhase struct {
	Regime string `json:"regime"` // "Bull", "Bear", "Sideways"
	Start  string `json:"start"`
	End    string `json:"end"`
}

// ConfigPath returns the path to the cockpit config file for a client.
func ConfigPath(dataDir, clientID string) string {
	return filepath.Join(dataDir, clientID, fmt.Sprintf("cockpit_%s.json", clientID))
}

// LoadConfig loads cockpit config from disk. If the file doesn't exist,
// creates a skeleton config with sensible defaults and saves it.
func LoadConfig(dataDir, clientID string) (*CockpitConfig, error) {
	path := ConfigPath(dataDir, clientID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultConfig(clientID)
			if saveErr := SaveConfig(dataDir, cfg); saveErr != nil {
				return nil, fmt.Errorf("create skeleton config: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg CockpitConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// SaveConfig writes cockpit config to disk.
func SaveConfig(dataDir string, cfg *CockpitConfig) error {
	path := ConfigPath(dataDir, cfg.ClientID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// ComputeLTCGRate returns the effective LTCG tax rate based on expected total income.
// Formula: 12.5% base × surcharge multiplier × 1.04 cess.
//
//	Income ≤ ₹50L  → 12.5% × 1.00 × 1.04 = 13.00%
//	₹50L – ₹1Cr   → 12.5% × 1.10 × 1.04 = 14.30%
//	≥ ₹1Cr         → 12.5% × 1.15 × 1.04 = 14.95%
func ComputeLTCGRate(income float64) float64 {
	surcharge := 1.0
	if income > 10000000 { // > ₹1 Cr
		surcharge = 1.15
	} else if income > 5000000 { // > ₹50L
		surcharge = 1.10
	}
	return 12.5 * surcharge * 1.04
}

// ResolveTaxRate returns the effective tax rate to use.
// Priority: ExpectedTotalIncome (auto-compute) > explicit TaxRate > default 13%.
func (cfg *CockpitConfig) ResolveTaxRate() float64 {
	if cfg.ExpectedTotalIncome > 0 {
		return ComputeLTCGRate(cfg.ExpectedTotalIncome)
	}
	if cfg.TaxRate > 0 {
		return cfg.TaxRate
	}
	return 13.0 // default: no surcharge
}

// defaultConfig returns a skeleton config with sensible defaults.
func defaultConfig(clientID string) *CockpitConfig {
	return &CockpitConfig{
		ClientID:         clientID,
		ReportDate:       "",
		TickerMap:        map[string]string{},
		Classifications:  map[string]Classification{},
		DisplayNames:     map[string]string{},
		PriceOnlySymbols: []string{},
		ManualLots:       []ManualLot{},
		MarketPhases:     []MarketPhase{},
		DefaultHurdlePct: 3,
		TaxRate:          0,
	}
}

// YahooTicker returns the Yahoo Finance ticker for a symbol.
// Uses the ticker map if available, otherwise defaults to {symbol}.NS.
func (cfg *CockpitConfig) YahooTicker(symbol string) string {
	if t, ok := cfg.TickerMap[symbol]; ok {
		return t
	}
	return symbol + ".NS"
}

// GetClassification returns the asset class and hurdle for a symbol.
// Defaults to "stock" with the configured default hurdle.
func (cfg *CockpitConfig) GetClassification(symbol string) (string, float64) {
	if c, ok := cfg.Classifications[symbol]; ok {
		return c.AssetClass, c.HurdlePct
	}
	return "stock", cfg.DefaultHurdlePct
}

// DisplayName returns the human-readable name for a symbol.
func (cfg *CockpitConfig) DisplayName(symbol string) string {
	if n, ok := cfg.DisplayNames[symbol]; ok {
		return n
	}
	return symbol
}

// IsPriceOnly returns true if the symbol should use price-only CAGR (no stock TRI).
func (cfg *CockpitConfig) IsPriceOnly(symbol string) bool {
	for _, s := range cfg.PriceOnlySymbols {
		if s == symbol {
			return true
		}
	}
	return false
}
