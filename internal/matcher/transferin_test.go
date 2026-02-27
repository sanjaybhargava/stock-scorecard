package matcher

import (
	"os"
	"testing"
	"time"

	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

func loadTestTRI(t *testing.T) *tri.TRIIndex {
	t.Helper()
	// Use embedded TRI from cmd/scorecard — find it relative to project root
	// For testing, create a minimal TRI in a temp file
	dir := t.TempDir()
	path := dir + "/tri.csv"
	data := "Date,TRI_Indexed\n2016-02-24,100.00\n2017-01-15,130.00\n2018-06-20,155.00\n2020-03-01,120.00\n2023-12-01,300.00\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	idx, err := tri.LoadTRI(path)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestDetectTransferIns_SellOnlyISIN(t *testing.T) {
	triIdx := loadTestTRI(t)

	// Warning for a symbol with no buy in trades — and it exists in nifty500Prices
	warnings := []Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Unmatched: 200, Total: 200},
	}
	trades := []tradebook.ConsolidatedTrade{
		// Different ISIN — no buy for RELIANCE
		{Symbol: "TCS", ISIN: "INE467B01029", Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), TradeType: "buy", Quantity: 100, AvgPrice: 2000},
	}

	realized, remaining := DetectTransferIns(warnings, trades, triIdx, nil)

	if len(realized) != 1 {
		t.Fatalf("expected 1 realized trade, got %d", len(realized))
	}
	if realized[0].Tier != TierIntelligent {
		t.Errorf("expected TierIntelligent, got %d", realized[0].Tier)
	}
	if realized[0].TierReason != "transfer_in" {
		t.Errorf("expected reason 'transfer_in', got %q", realized[0].TierReason)
	}
	if realized[0].BuyDate != firstTRIDate {
		t.Errorf("expected buy date %v, got %v", firstTRIDate, realized[0].BuyDate)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining warnings, got %d", len(remaining))
	}
}

func TestDetectTransferIns_ISINWithPriorBuy(t *testing.T) {
	triIdx := loadTestTRI(t)

	// Warning for ISIN that has a buy before the sell — unmatched portion is
	// still a pre-history holding, should be auto-matched if price exists
	warnings := []Warning{
		{Symbol: "TCS", ISIN: "INE467B01029", SellDate: "2023-12-01", SellPrice: 3800, Unmatched: 50, Total: 100},
	}
	trades := []tradebook.ConsolidatedTrade{
		{Symbol: "TCS", ISIN: "INE467B01029", Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), TradeType: "buy", Quantity: 100, AvgPrice: 2000},
	}

	realized, remaining := DetectTransferIns(warnings, trades, triIdx, nil)

	// TCS is in the static price map, so it should be matched
	if len(realized) != 1 {
		t.Errorf("expected 1 realized trade (pre-history holding), got %d", len(realized))
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining warnings, got %d", len(remaining))
	}
}

func TestDetectTransferIns_MissingPrice(t *testing.T) {
	triIdx := loadTestTRI(t)

	// Warning for a symbol NOT in nifty500Prices
	warnings := []Warning{
		{Symbol: "OBSCURETICKER", ISIN: "INE999Z01099", SellDate: "2023-12-01", SellPrice: 500, Unmatched: 100, Total: 100},
	}

	realized, remaining := DetectTransferIns(warnings, nil, triIdx, nil)

	if len(realized) != 0 {
		t.Errorf("expected 0 realized (no price), got %d", len(realized))
	}
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining warning, got %d", len(remaining))
	}
}

func TestDetectTransferIns_BuyAfterSell(t *testing.T) {
	triIdx := loadTestTRI(t)

	// Warning: earliest buy is AFTER the sell date — this IS a transfer-in
	warnings := []Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2018-06-20", SellPrice: 1100, Unmatched: 100, Total: 100},
	}
	trades := []tradebook.ConsolidatedTrade{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), TradeType: "buy", Quantity: 200, AvgPrice: 1500},
	}

	realized, remaining := DetectTransferIns(warnings, trades, triIdx, nil)

	if len(realized) != 1 {
		t.Fatalf("expected 1 realized (buy after sell = transfer-in), got %d", len(realized))
	}
	if realized[0].TierReason != "transfer_in" {
		t.Errorf("expected reason 'transfer_in', got %q", realized[0].TierReason)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining, got %d", len(remaining))
	}
}
