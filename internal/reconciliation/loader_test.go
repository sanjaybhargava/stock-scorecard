package reconciliation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	orig := Default()

	dir := t.TempDir()
	path := filepath.Join(dir, "reconciliation_test.json")

	if err := Save(path, orig); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Compare client ID
	if loaded.ClientID != orig.ClientID {
		t.Errorf("ClientID: got %q, want %q", loaded.ClientID, orig.ClientID)
	}

	// Compare splits
	if len(loaded.Splits) != len(orig.Splits) {
		t.Fatalf("Splits: got %d, want %d", len(loaded.Splits), len(orig.Splits))
	}
	for i, s := range orig.Splits {
		l := loaded.Splits[i]
		if l.OldISIN != s.OldISIN || l.NewISIN != s.NewISIN || l.Ratio != s.Ratio || l.Note != s.Note {
			t.Errorf("Splits[%d]: got %+v, want %+v", i, l, s)
		}
	}

	// Compare demergers
	if len(loaded.Demergers) != len(orig.Demergers) {
		t.Fatalf("Demergers: got %d, want %d", len(loaded.Demergers), len(orig.Demergers))
	}
	for i, d := range orig.Demergers {
		l := loaded.Demergers[i]
		if l.ParentISIN != d.ParentISIN || l.ChildISIN != d.ChildISIN ||
			l.ChildSymbol != d.ChildSymbol || l.RecordDate != d.RecordDate ||
			l.ParentCostPct != d.ParentCostPct {
			t.Errorf("Demergers[%d]: got %+v, want %+v", i, l, d)
		}
	}

	// Compare manual trades
	if len(loaded.ManualTrades) != len(orig.ManualTrades) {
		t.Fatalf("ManualTrades: got %d, want %d", len(loaded.ManualTrades), len(orig.ManualTrades))
	}
	for i, m := range orig.ManualTrades {
		l := loaded.ManualTrades[i]
		if l.Symbol != m.Symbol || l.ISIN != m.ISIN || l.Date != m.Date ||
			l.TradeType != m.TradeType || l.Quantity != m.Quantity || l.Price != m.Price {
			t.Errorf("ManualTrades[%d]: got %+v, want %+v", i, l, m)
		}
	}

	// Compare F&O renames
	if len(loaded.FnORenames) != len(orig.FnORenames) {
		t.Fatalf("FnORenames: got %d, want %d", len(loaded.FnORenames), len(orig.FnORenames))
	}
	for k, v := range orig.FnORenames {
		if loaded.FnORenames[k] != v {
			t.Errorf("FnORenames[%q]: got %q, want %q", k, loaded.FnORenames[k], v)
		}
	}
}

func TestSplitsMap(t *testing.T) {
	r := Default()
	m := r.SplitsMap()

	if len(m) != len(r.Splits) {
		t.Fatalf("SplitsMap: got %d entries, want %d", len(m), len(r.Splits))
	}

	// Check one specific entry
	s, ok := m["INE935N01012"]
	if !ok {
		t.Fatal("SplitsMap: missing INE935N01012 (DIXON)")
	}
	if s.NewISIN != "INE935N01020" || s.Ratio != 5 {
		t.Errorf("DIXON split: got %+v, want NewISIN=INE935N01020, Ratio=5", s)
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/path.json")
	if err == nil {
		t.Error("Load: expected error for nonexistent file")
	}
}

func TestSaveCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reconciliation.json")

	r := &ReconciliationData{ClientID: "TEST"}
	if err := Save(path, r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Save: file not created")
	}
}
