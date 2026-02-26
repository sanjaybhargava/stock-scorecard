package cleandata

import (
	"path/filepath"
	"testing"
	"time"

	"stock-scorecard/internal/fno"
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
