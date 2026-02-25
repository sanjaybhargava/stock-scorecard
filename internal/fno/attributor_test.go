package fno

import (
	"testing"
	"time"

	"stock-scorecard/internal/matcher"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func TestOverlapDays(t *testing.T) {
	tests := []struct {
		name          string
		contractStart time.Time
		contractEnd   time.Time
		buyDate       time.Time
		sellDate      time.Time
		want          int
	}{
		{
			name:          "full overlap",
			contractStart: date(2024, 1, 1),
			contractEnd:   date(2024, 1, 31),
			buyDate:       date(2023, 12, 1),
			sellDate:      date(2024, 2, 28),
			want:          31,
		},
		{
			name:          "no overlap — contract before holding",
			contractStart: date(2024, 1, 1),
			contractEnd:   date(2024, 1, 31),
			buyDate:       date(2024, 3, 1),
			sellDate:      date(2024, 4, 1),
			want:          0,
		},
		{
			name:          "no overlap — contract after holding",
			contractStart: date(2024, 6, 1),
			contractEnd:   date(2024, 6, 30),
			buyDate:       date(2024, 1, 1),
			sellDate:      date(2024, 3, 1),
			want:          0,
		},
		{
			name:          "partial overlap — contract starts before buy",
			contractStart: date(2024, 1, 1),
			contractEnd:   date(2024, 1, 31),
			buyDate:       date(2024, 1, 15),
			sellDate:      date(2024, 2, 28),
			want:          17, // Jan 15 to Jan 31 inclusive
		},
		{
			name:          "partial overlap — contract ends after sell",
			contractStart: date(2024, 1, 15),
			contractEnd:   date(2024, 2, 28),
			buyDate:       date(2024, 1, 1),
			sellDate:      date(2024, 1, 31),
			want:          17, // Jan 15 to Jan 31 inclusive
		},
		{
			name:          "same day = 0 (strict Before comparison)",
			contractStart: date(2024, 1, 15),
			contractEnd:   date(2024, 1, 15),
			buyDate:       date(2024, 1, 15),
			sellDate:      date(2024, 1, 15),
			want:          0, // start == end, so !start.Before(end) → 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlapDays(tt.contractStart, tt.contractEnd, tt.buyDate, tt.sellDate)
			if got != tt.want {
				t.Errorf("overlapDays() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestComputeContractPnLs(t *testing.T) {
	trades := []FnOTrade{
		{RawSymbol: "TCS24JAN4000CE", Underlying: "TCS", OptionType: "CE", ExpiryDate: date(2024, 1, 25), TradeDate: date(2024, 1, 2), TradeType: "sell", Quantity: 150, Price: 50, Value: 7500},
		{RawSymbol: "TCS24JAN4000CE", Underlying: "TCS", OptionType: "CE", ExpiryDate: date(2024, 1, 25), TradeDate: date(2024, 1, 20), TradeType: "buy", Quantity: 150, Price: 10, Value: 1500},
	}

	contracts := ComputeContractPnLs(trades)
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}

	c := contracts[0]
	if c.Underlying != "TCS" {
		t.Errorf("underlying = %q, want TCS", c.Underlying)
	}
	if c.OptionType != "CE" {
		t.Errorf("optionType = %q, want CE", c.OptionType)
	}
	// net_pnl = sell_value - buy_value = 7500 - 1500 = 6000
	if c.NetPnL != 6000 {
		t.Errorf("NetPnL = %f, want 6000", c.NetPnL)
	}
	// FirstDate should be the earliest trade date
	if !c.FirstDate.Equal(date(2024, 1, 2)) {
		t.Errorf("FirstDate = %v, want 2024-01-02", c.FirstDate)
	}
}

func TestAttribute_OverlapBased(t *testing.T) {
	contracts := []ContractPnL{
		{
			Underlying: "TCS",
			RawSymbol:  "TCS24JAN4000CE",
			OptionType: "CE",
			ExpiryDate: date(2024, 1, 25),
			FirstDate:  date(2024, 1, 2),
			NetPnL:     6000,
		},
	}

	realized := []matcher.RealizedTrade{
		{
			Symbol:   "TCS",
			BuyDate:  date(2023, 12, 1),
			SellDate: date(2024, 2, 28),
			Quantity: 150,
		},
	}

	attribution, unattributed := Attribute(contracts, realized)
	if len(unattributed) != 0 {
		t.Errorf("expected 0 unattributed, got %d", len(unattributed))
	}
	if got, ok := attribution[0]; !ok {
		t.Error("expected attribution to trade 0")
	} else if got != 6000 {
		t.Errorf("attribution[0] = %f, want 6000", got)
	}
}

func TestAttribute_NextBuyFallback(t *testing.T) {
	// Cash-secured put expires, then stock is bought → next-buy attribution
	contracts := []ContractPnL{
		{
			Underlying: "INFY",
			RawSymbol:  "INFY24JAN1500PE",
			OptionType: "PE",
			ExpiryDate: date(2024, 1, 25),
			FirstDate:  date(2024, 1, 2),
			NetPnL:     3000,
		},
	}

	realized := []matcher.RealizedTrade{
		{
			// Equity bought AFTER put expiry — should match via next-buy
			Symbol:   "INFY",
			BuyDate:  date(2024, 1, 26),
			SellDate: date(2024, 6, 15),
			Quantity: 200,
		},
	}

	attribution, unattributed := Attribute(contracts, realized)
	if len(unattributed) != 0 {
		t.Errorf("expected 0 unattributed, got %d", len(unattributed))
	}
	if got, ok := attribution[0]; !ok {
		t.Error("expected attribution to trade 0")
	} else if got != 3000 {
		t.Errorf("attribution[0] = %f, want 3000", got)
	}
}

func TestAttribute_CENoNextBuy(t *testing.T) {
	// CE contract with no overlap should NOT use next-buy fallback
	contracts := []ContractPnL{
		{
			Underlying: "RELIANCE",
			RawSymbol:  "RELIANCE24JAN2500CE",
			OptionType: "CE",
			ExpiryDate: date(2024, 1, 25),
			FirstDate:  date(2024, 1, 2),
			NetPnL:     5000,
		},
	}

	realized := []matcher.RealizedTrade{
		{
			Symbol:   "RELIANCE",
			BuyDate:  date(2024, 2, 1), // after contract expiry
			SellDate: date(2024, 6, 15),
			Quantity: 100,
		},
	}

	attribution, unattributed := Attribute(contracts, realized)
	if len(attribution) != 0 {
		t.Errorf("CE should not use next-buy fallback, got %d attributions", len(attribution))
	}
	if len(unattributed) != 1 {
		t.Fatalf("expected 1 unattributed, got %d", len(unattributed))
	}
	if unattributed[0].NetPnL != 5000 {
		t.Errorf("unattributed NetPnL = %f, want 5000", unattributed[0].NetPnL)
	}
}

func TestAttribute_NoEquityTrades(t *testing.T) {
	// Index option (NIFTY) — no equity trades at all
	contracts := []ContractPnL{
		{
			Underlying: "NIFTY",
			RawSymbol:  "NIFTY24JAN22000CE",
			OptionType: "CE",
			ExpiryDate: date(2024, 1, 25),
			FirstDate:  date(2024, 1, 2),
			NetPnL:     -2000,
		},
	}

	attribution, unattributed := Attribute(contracts, []matcher.RealizedTrade{})
	if len(attribution) != 0 {
		t.Errorf("expected 0 attributions, got %d", len(attribution))
	}
	if len(unattributed) != 1 {
		t.Fatalf("expected 1 unattributed, got %d", len(unattributed))
	}
	if unattributed[0].Underlying != "NIFTY" {
		t.Errorf("underlying = %q, want NIFTY", unattributed[0].Underlying)
	}
}

func TestFyYear(t *testing.T) {
	tests := []struct {
		date time.Time
		want int
	}{
		{date(2024, 4, 1), 2024},  // Apr = start of FY 2024-25
		{date(2024, 3, 31), 2023}, // Mar = end of FY 2023-24
		{date(2024, 1, 15), 2023}, // Jan = FY 2023-24
		{date(2024, 12, 1), 2024}, // Dec = FY 2024-25
	}
	for _, tt := range tests {
		got := fyYear(tt.date)
		if got != tt.want {
			t.Errorf("fyYear(%v) = %d, want %d", tt.date, got, tt.want)
		}
	}
}
