package wizard

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/reconciliation"
)

func TestOpenPosition_Held(t *testing.T) {
	input := "h\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
	}

	changed := w.ReconcileOpenPositions(open, recon)
	if changed {
		t.Error("expected no changes for held position")
	}
	if len(recon.ManualTrades) != 0 {
		t.Errorf("expected 0 manual trades, got %d", len(recon.ManualTrades))
	}
	if !strings.Contains(out.String(), "Kept as open position") {
		t.Error("expected 'Kept as open position' in output")
	}
}

func TestOpenPosition_Sold(t *testing.T) {
	input := "s\n2024-08-15\n1800\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
	}

	changed := w.ReconcileOpenPositions(open, recon)
	if !changed {
		t.Error("expected changes for sold position")
	}
	if len(recon.ManualTrades) != 1 {
		t.Fatalf("expected 1 manual trade, got %d", len(recon.ManualTrades))
	}

	m := recon.ManualTrades[0]
	if m.Symbol != "HDFCBANK" || m.TradeType != "sell" || m.Quantity != 500 || m.Price != 1800 || m.Date != "2024-08-15" {
		t.Errorf("unexpected manual trade: %+v", m)
	}
}

func TestOpenPosition_Skip(t *testing.T) {
	input := "k\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "INFY", ISIN: "INE009A01021", BuyDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Quantity: 1000, BuyPrice: 1500},
	}

	changed := w.ReconcileOpenPositions(open, recon)
	if changed {
		t.Error("expected no changes for skipped position")
	}
	if !strings.Contains(out.String(), "Skipped") {
		t.Error("expected 'Skipped' in output")
	}
}

func TestUnmatchedSell_ProvideBuy(t *testing.T) {
	input := "p\n2022-06-15\n2500\nINE002A01018\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	warnings := []matcher.Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Unmatched: 200, Total: 200},
	}

	changed := w.ReconcileUnmatchedSells(warnings, recon)
	if !changed {
		t.Error("expected changes for provided buy")
	}
	if len(recon.ManualTrades) != 1 {
		t.Fatalf("expected 1 manual trade, got %d", len(recon.ManualTrades))
	}

	m := recon.ManualTrades[0]
	if m.Symbol != "RELIANCE" || m.TradeType != "buy" || m.Quantity != 200 || m.Price != 2500 || m.Date != "2022-06-15" || m.ISIN != "INE002A01018" {
		t.Errorf("unexpected manual trade: %+v", m)
	}
}

func TestUnmatchedSell_Skip(t *testing.T) {
	input := "s\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	warnings := []matcher.Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Unmatched: 200, Total: 200},
	}

	changed := w.ReconcileUnmatchedSells(warnings, recon)
	if changed {
		t.Error("expected no changes for skipped sell")
	}
}

func TestFormatLakhs(t *testing.T) {
	tests := []struct {
		amount float64
		want   string
	}{
		{830000, "8.3L"},
		{1500000, "15.0L"},
		{42500, "42500"},
		{100000, "1.0L"},
		{99999, "99999"},
	}
	for _, tt := range tests {
		got := FormatLakhs(tt.amount)
		if got != tt.want {
			t.Errorf("FormatLakhs(%.0f) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}

func TestCancelledInput(t *testing.T) {
	// EOF immediately — simulates Ctrl-C / pipe close
	input := ""
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
		{Symbol: "INFY", ISIN: "INE009A01021", BuyDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Quantity: 1000, BuyPrice: 1500},
	}

	changed := w.ReconcileOpenPositions(open, recon)
	if changed {
		t.Error("expected no changes on cancelled input")
	}
	if !strings.Contains(out.String(), "(cancelled)") {
		t.Error("expected '(cancelled)' in output")
	}
}

func TestInvalidSellDate(t *testing.T) {
	input := "s\nbaddate\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
	}

	w.ReconcileOpenPositions(open, recon)
	if len(recon.ManualTrades) != 0 {
		t.Errorf("expected 0 manual trades for invalid date, got %d", len(recon.ManualTrades))
	}
	if !strings.Contains(out.String(), "Invalid date") {
		t.Error("expected 'Invalid date' in output")
	}
}

func TestInvalidSellPrice(t *testing.T) {
	input := "s\n2024-08-15\n0\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
	}

	w.ReconcileOpenPositions(open, recon)
	if len(recon.ManualTrades) != 0 {
		t.Errorf("expected 0 manual trades for invalid price, got %d", len(recon.ManualTrades))
	}
	if !strings.Contains(out.String(), "Invalid price") {
		t.Error("expected 'Invalid price' in output")
	}
}

func TestInvalidBuyDate(t *testing.T) {
	input := "p\nnot-a-date\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	warnings := []matcher.Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Unmatched: 200, Total: 200},
	}

	w.ReconcileUnmatchedSells(warnings, recon)
	if len(recon.ManualTrades) != 0 {
		t.Errorf("expected 0 manual trades for invalid buy date, got %d", len(recon.ManualTrades))
	}
	if !strings.Contains(out.String(), "Invalid date") {
		t.Error("expected 'Invalid date' in output")
	}
}

func TestInvalidBuyPrice(t *testing.T) {
	input := "p\n2022-06-15\n0\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	warnings := []matcher.Warning{
		{Symbol: "RELIANCE", ISIN: "INE002A01018", SellDate: "2023-12-01", SellPrice: 2500, Unmatched: 200, Total: 200},
	}

	w.ReconcileUnmatchedSells(warnings, recon)
	if len(recon.ManualTrades) != 0 {
		t.Errorf("expected 0 manual trades for invalid buy price, got %d", len(recon.ManualTrades))
	}
	if !strings.Contains(out.String(), "Invalid price") {
		t.Error("expected 'Invalid price' in output")
	}
}

func TestMultipleOpenPositions(t *testing.T) {
	// Test with 2 open positions: first held, second sold
	input := "h\ns\n2024-08-15\n1800\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "HDFCBANK", ISIN: "INE040A01034", BuyDate: time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC), Quantity: 500, BuyPrice: 1650},
		{Symbol: "INFY", ISIN: "INE009A01021", BuyDate: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), Quantity: 1000, BuyPrice: 1500},
	}

	changed := w.ReconcileOpenPositions(open, recon)
	if !changed {
		t.Error("expected changes")
	}
	if len(recon.ManualTrades) != 1 {
		t.Fatalf("expected 1 manual trade, got %d", len(recon.ManualTrades))
	}
	if recon.ManualTrades[0].Symbol != "INFY" {
		t.Errorf("expected INFY trade, got %s", recon.ManualTrades[0].Symbol)
	}

	// Verify progress indicators appear
	output := out.String()
	if !strings.Contains(output, "1 of 2") || !strings.Contains(output, "2 of 2") {
		t.Error("expected progress indicators '1 of 2' and '2 of 2'")
	}
}

func TestSmallPositionRecommendation(t *testing.T) {
	input := "k\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	open := []matcher.OpenPosition{
		{Symbol: "N100", ISIN: "INE123A01010", BuyDate: time.Date(2020, 8, 28, 0, 0, 0, 0, time.UTC), Quantity: 5, BuyPrice: 872},
	}

	w.ReconcileOpenPositions(open, recon)
	output := out.String()
	if !strings.Contains(output, "small position") {
		t.Error("expected 'small position' recommendation for invested < ₹10,000")
	}
}

func TestSmallUnmatchedSellRecommendation(t *testing.T) {
	input := "s\n"
	var out bytes.Buffer
	w := New(strings.NewReader(input), &out)

	recon := &reconciliation.ReconciliationData{}
	warnings := []matcher.Warning{
		{Symbol: "MON100", ISIN: "INE123B01010", SellDate: "2021-07-01", SellPrice: 100, Unmatched: 7, Total: 7},
	}

	w.ReconcileUnmatchedSells(warnings, recon)
	output := out.String()
	if !strings.Contains(output, "small position") {
		t.Error("expected 'small position' recommendation for quantity < 20")
	}
}
