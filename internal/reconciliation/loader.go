package reconciliation

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads a ReconciliationData from a JSON file.
func Load(path string) (*ReconciliationData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read reconciliation file: %w", err)
	}
	var r ReconciliationData
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse reconciliation JSON: %w", err)
	}
	return &r, nil
}

// Save writes a ReconciliationData to a JSON file with indentation.
func Save(path string, r *ReconciliationData) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal reconciliation data: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write reconciliation file: %w", err)
	}
	return nil
}

// Default returns the hardcoded reconciliation data for client BT2632.
// This preserves backward compatibility when no --reconciliation flag is given.
func Default() *ReconciliationData {
	return &ReconciliationData{
		ClientID: "BT2632",
		Splits: []Split{
			{OldISIN: "INE00WC01019", NewISIN: "INE00WC01027", Ratio: 5, Note: "AFFLE bonus 4:1 (5x total)"},
			{OldISIN: "INE935N01012", NewISIN: "INE935N01020", Ratio: 5, Note: "DIXON stock split 1:5"},
			{OldISIN: "INE254N01018", NewISIN: "INE254N01026", Ratio: 5, Note: "HNDFDS stock split 1:5"},
			{OldISIN: "INE239A01016", NewISIN: "INE239A01024", Ratio: 10, Note: "NESTLEIND stock split 1:10"},
			{OldISIN: "INE884A01019", NewISIN: "INE884A01027", Ratio: 5, Note: "VAIBHAVGBL stock split 1:5"},
			{OldISIN: "INE001A01036", NewISIN: "INE040A01034", Ratio: 1.68, Note: "HDFC→HDFCBANK merger 42:25"},
		},
		Demergers: []Demerger{
			{
				ParentISIN:    "INE002A01018",
				ChildISIN:     "INE758E01017",
				ChildSymbol:   "JIOFIN",
				RecordDate:    "2023-07-20",
				ParentCostPct: 0.9532,
			},
		},
		ManualTrades: []ManualTrade{
			{Symbol: "MPHASIS", ISIN: "INE356A01018", Date: "2022-01-27", TradeType: "buy", Quantity: 700, Price: 3000.00},
			{Symbol: "SYNGENE", ISIN: "INE398R01022", Date: "2022-06-30", TradeType: "sell", Quantity: 3400, Price: 550.00},
			{Symbol: "DIVISLAB", ISIN: "INE361B01024", Date: "2021-09-30", TradeType: "buy", Quantity: 600, Price: 4800.00},
			{Symbol: "POWERGRID", ISIN: "INE752E01010", Date: "2023-09-12", TradeType: "buy", Quantity: 900, Price: 0},
			{Symbol: "SUNPHARMA", ISIN: "INE044A01036", Date: "2023-06-28", TradeType: "sell", Quantity: 700, Price: 1020.00},
			{Symbol: "BRITANNIA", ISIN: "INE216A01030", Date: "2021-01-28", TradeType: "sell", Quantity: 1000, Price: 3600.00},
			{Symbol: "ULTRACEMCO", ISIN: "INE481G01011", Date: "2023-11-30", TradeType: "sell", Quantity: 100, Price: 9000.00},
			{Symbol: "NAUKRI", ISIN: "INE663F01024", Date: "2021-04-29", TradeType: "sell", Quantity: 750, Price: 5000.00},
		},
		FnORenames: map[string]string{
			"MOTHERSUMI": "MOTHERSON",
			"HDFC":       "HDFCBANK",
		},
	}
}
