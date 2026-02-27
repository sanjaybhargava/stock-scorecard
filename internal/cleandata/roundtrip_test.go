package cleandata

import (
	"path/filepath"
	"testing"
	"time"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/tradebook"
)

func TestTradesRoundtrip(t *testing.T) {
	trades := []tradebook.ConsolidatedTrade{
		{Symbol: "BHARTIARTL", ISIN: "INE397D01024", Date: time.Date(2023, 8, 30, 0, 0, 0, 0, time.UTC), TradeType: "buy", Quantity: 950, AvgPrice: 859.70, Value: 816715, OrderID: "123456"},
		{Symbol: "BHARTIARTL", ISIN: "INE397D01024", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), TradeType: "sell", Quantity: 950, AvgPrice: 1600, Value: 1520000, OrderID: "789012"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "trades.csv")

	if err := WriteTrades(path, trades); err != nil {
		t.Fatalf("WriteTrades: %v", err)
	}

	loaded, err := ReadTrades(path)
	if err != nil {
		t.Fatalf("ReadTrades: %v", err)
	}

	if len(loaded) != len(trades) {
		t.Fatalf("got %d trades, want %d", len(loaded), len(trades))
	}

	for i, want := range trades {
		got := loaded[i]
		if got.Symbol != want.Symbol || got.ISIN != want.ISIN || !got.Date.Equal(want.Date) ||
			got.TradeType != want.TradeType || got.Quantity != want.Quantity || got.OrderID != want.OrderID {
			t.Errorf("trade[%d]: got %+v, want %+v", i, got, want)
		}
		// AvgPrice may have rounding differences due to CSV formatting
		if diff := got.AvgPrice - want.AvgPrice; diff > 0.01 || diff < -0.01 {
			t.Errorf("trade[%d] AvgPrice: got %f, want %f", i, got.AvgPrice, want.AvgPrice)
		}
	}
}

func TestFnOTradesRoundtrip(t *testing.T) {
	trades := []fno.FnOTrade{
		{RawSymbol: "TCS24JAN4000CE", Underlying: "TCS", OptionType: "CE", ExpiryDate: time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC), TradeDate: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), TradeType: "sell", Quantity: 150, Price: 50, Value: 7500, TradeID: "T1", OrderID: "O1"},
		{RawSymbol: "TCS24JAN4000CE", Underlying: "TCS", OptionType: "CE", ExpiryDate: time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC), TradeDate: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), TradeType: "buy", Quantity: 150, Price: 10, Value: 1500, TradeID: "T2", OrderID: "O2"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "fno.csv")

	if err := WriteFnOTrades(path, trades); err != nil {
		t.Fatalf("WriteFnOTrades: %v", err)
	}

	loaded, err := ReadFnOTrades(path)
	if err != nil {
		t.Fatalf("ReadFnOTrades: %v", err)
	}

	if len(loaded) != len(trades) {
		t.Fatalf("got %d trades, want %d", len(loaded), len(trades))
	}

	for i, want := range trades {
		got := loaded[i]
		if got.RawSymbol != want.RawSymbol || got.Underlying != want.Underlying ||
			got.OptionType != want.OptionType || !got.ExpiryDate.Equal(want.ExpiryDate) ||
			!got.TradeDate.Equal(want.TradeDate) || got.TradeType != want.TradeType ||
			got.Quantity != want.Quantity || got.TradeID != want.TradeID || got.OrderID != want.OrderID {
			t.Errorf("trade[%d]: got %+v, want %+v", i, got, want)
		}
	}
}

func TestRealizedTradesRoundtrip(t *testing.T) {
	trades := []matcher.RealizedTrade{
		{Symbol: "BHARTIARTL", ISIN: "INE397D01024", FY: "FY 2024-25", Type: "Long", BuyDate: time.Date(2023, 8, 30, 0, 0, 0, 0, time.UTC), SellDate: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), HoldDays: 494, Quantity: 950, BuyPrice: 859.70, SellPrice: 1600, Invested: 816715, SaleValue: 1520000, EquityGL: 703285, NiftyBuy: 280.50, NiftySell: 410.20, NiftyReturn: 377500, Tier: matcher.TierExact},
		{Symbol: "TCS", ISIN: "INE467B01029", FY: "FY 2023-24", Type: "Short", BuyDate: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC), SellDate: time.Date(2023, 12, 20, 0, 0, 0, 0, time.UTC), HoldDays: 188, Quantity: 100, BuyPrice: 3400, SellPrice: 3800, Invested: 340000, SaleValue: 380000, EquityGL: 40000, NiftyBuy: 250.10, NiftySell: 290.30, NiftyReturn: 54658, Tier: matcher.TierIntelligent, TierReason: "transfer_in"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "realized.csv")

	if err := WriteRealizedTrades(path, trades); err != nil {
		t.Fatalf("WriteRealizedTrades: %v", err)
	}

	loaded, err := ReadRealizedTrades(path)
	if err != nil {
		t.Fatalf("ReadRealizedTrades: %v", err)
	}

	if len(loaded) != len(trades) {
		t.Fatalf("got %d trades, want %d", len(loaded), len(trades))
	}

	for i, want := range trades {
		got := loaded[i]
		if got.Symbol != want.Symbol || got.ISIN != want.ISIN || got.FY != want.FY || got.Type != want.Type ||
			!got.BuyDate.Equal(want.BuyDate) || !got.SellDate.Equal(want.SellDate) ||
			got.HoldDays != want.HoldDays || got.Quantity != want.Quantity ||
			got.Invested != want.Invested || got.SaleValue != want.SaleValue ||
			got.EquityGL != want.EquityGL || got.NiftyReturn != want.NiftyReturn {
			t.Errorf("trade[%d]: got %+v, want %+v", i, got, want)
		}
		if got.Tier != want.Tier {
			t.Errorf("trade[%d] Tier: got %d, want %d", i, got.Tier, want.Tier)
		}
		if got.TierReason != want.TierReason {
			t.Errorf("trade[%d] TierReason: got %q, want %q", i, got.TierReason, want.TierReason)
		}
		// Prices and TRI values may have rounding from CSV formatting
		if diff := got.BuyPrice - want.BuyPrice; diff > 0.01 || diff < -0.01 {
			t.Errorf("trade[%d] BuyPrice: got %f, want %f", i, got.BuyPrice, want.BuyPrice)
		}
		if diff := got.SellPrice - want.SellPrice; diff > 0.01 || diff < -0.01 {
			t.Errorf("trade[%d] SellPrice: got %f, want %f", i, got.SellPrice, want.SellPrice)
		}
		if diff := got.NiftyBuy - want.NiftyBuy; diff > 0.01 || diff < -0.01 {
			t.Errorf("trade[%d] NiftyBuy: got %f, want %f", i, got.NiftyBuy, want.NiftyBuy)
		}
		if diff := got.NiftySell - want.NiftySell; diff > 0.01 || diff < -0.01 {
			t.Errorf("trade[%d] NiftySell: got %f, want %f", i, got.NiftySell, want.NiftySell)
		}
	}
}

func TestReviewCSVRoundtrip(t *testing.T) {
	items := []ReviewItem{
		{Tier: 2, Status: "auto", Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Quantity: 200, SellValue: 500000, BuyDate: "2016-02-24", BuyPrice: 1035, Reason: "transfer_in"},
		{Tier: 3, Status: "unresolved", Symbol: "OBSCURE", ISIN: "INE999Z01099", SellDate: "2020-06-15", SellPrice: 300, Quantity: 50, SellValue: 15000, Reason: "no_buy_found"},
	}
	summary := CoverageSummary{
		TotalSells: 100, MatchedSells: 95, TotalSellValue: 5000000, MatchedSellValue: 4800000,
		IntelligentCount: 1, IntelligentValue: 500000, UnresolvedCount: 1, UnresolvedValue: 15000,
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "review.csv")

	if err := WriteReviewCSV(path, items, summary); err != nil {
		t.Fatalf("WriteReviewCSV: %v", err)
	}

	loaded, err := ReadReviewCSV(path)
	if err != nil {
		t.Fatalf("ReadReviewCSV: %v", err)
	}

	if len(loaded) != len(items) {
		t.Fatalf("got %d items, want %d", len(loaded), len(items))
	}

	// Check first item (Tier 2 — sorted first by tier, then by sell_value desc)
	got := loaded[0]
	if got.Tier != 2 || got.Status != "auto" || got.Symbol != "RELIANCE" || got.Reason != "transfer_in" {
		t.Errorf("item[0]: got %+v", got)
	}
	if got.BuyDate != "2016-02-24" || got.BuyPrice != 1035 {
		t.Errorf("item[0] buy: got date=%q price=%f", got.BuyDate, got.BuyPrice)
	}

	// Check second item (Tier 3)
	got2 := loaded[1]
	if got2.Tier != 3 || got2.Status != "unresolved" || got2.Symbol != "OBSCURE" {
		t.Errorf("item[1]: got %+v", got2)
	}
	if got2.BuyDate != "" || got2.BuyPrice != 0 {
		t.Errorf("item[1] should have empty buy: got date=%q price=%f", got2.BuyDate, got2.BuyPrice)
	}
}

func TestOpenPositionsRoundtrip(t *testing.T) {
	positions := []matcher.OpenPosition{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", BuyDate: time.Date(2022, 3, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 2450.50, Invested: 1225250},
		{Symbol: "INFY", ISIN: "INE009A01021", BuyDate: time.Date(2023, 7, 10, 0, 0, 0, 0, time.UTC), Quantity: 200, BuyPrice: 1380.75, Invested: 276150},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "unrealized.csv")

	if err := WriteOpenPositions(path, positions); err != nil {
		t.Fatalf("WriteOpenPositions: %v", err)
	}

	loaded, err := ReadOpenPositions(path)
	if err != nil {
		t.Fatalf("ReadOpenPositions: %v", err)
	}

	if len(loaded) != len(positions) {
		t.Fatalf("got %d positions, want %d", len(loaded), len(positions))
	}

	for i, want := range positions {
		got := loaded[i]
		if got.Symbol != want.Symbol || got.ISIN != want.ISIN ||
			!got.BuyDate.Equal(want.BuyDate) || got.Quantity != want.Quantity ||
			got.Invested != want.Invested {
			t.Errorf("position[%d]: got %+v, want %+v", i, got, want)
		}
		if diff := got.BuyPrice - want.BuyPrice; diff > 0.01 || diff < -0.01 {
			t.Errorf("position[%d] BuyPrice: got %f, want %f", i, got.BuyPrice, want.BuyPrice)
		}
	}
}
