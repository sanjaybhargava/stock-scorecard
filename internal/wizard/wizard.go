// Package wizard provides an interactive reconciliation wizard that helps
// users resolve open positions, unmatched sells, and missing data after
// FIFO matching. It updates a ReconciliationData struct with user decisions.
package wizard

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strings"

	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/reconciliation"
)

// Wizard runs the interactive reconciliation flow.
type Wizard struct {
	in  *bufio.Reader
	out io.Writer
}

// New creates a Wizard that reads from r and writes to w.
func New(r io.Reader, w io.Writer) *Wizard {
	return &Wizard{
		in:  bufio.NewReader(r),
		out: w,
	}
}

// ReconcileOpenPositions asks the user about each open position.
// Returns true if any changes were made (caller should re-run FIFO).
func (w *Wizard) ReconcileOpenPositions(open []matcher.OpenPosition, recon *reconciliation.ReconciliationData) bool {
	changed := false

	for i, pos := range open {
		invested := math.Round(pos.Quantity * pos.BuyPrice)
		w.printf("  %d of %d: %s — %.0f shares bought %s @ ₹%.0f (₹%s)\n",
			i+1, len(open), pos.Symbol, pos.Quantity,
			pos.BuyDate.Format("2006-01-02"), pos.BuyPrice, formatLakhs(invested))

		// Smart recommendation for small positions
		if invested < 10000 {
			w.printf("          This is a small position (under ₹10,000).\n")
		}

		w.printf("          [H]eld  [S]old  s[K]ip → ")

		choice := w.readChoice("hsk")
		switch choice {
		case 'h':
			w.printf("          ✓ Kept as open position.\n\n")
		case 's':
			sellDate := w.readLine("          Sell date (YYYY-MM-DD): ")
			sellPrice := w.readLine("          Sell price per share: ")
			price := parseFloat(sellPrice)
			recon.ManualTrades = append(recon.ManualTrades, reconciliation.ManualTrade{
				Symbol:    pos.Symbol,
				ISIN:      pos.ISIN,
				Date:      sellDate,
				TradeType: "sell",
				Quantity:  pos.Quantity,
				Price:     price,
			})
			w.printf("          ✓ Added sell: %.0f shares @ ₹%.0f on %s\n\n", pos.Quantity, price, sellDate)
			changed = true
		case 'k':
			w.printf("          ✓ Skipped.\n\n")
		}
	}

	return changed
}

// ReconcileUnmatchedSells asks the user about each unmatched sell.
// Returns true if any changes were made (caller should re-run FIFO).
func (w *Wizard) ReconcileUnmatchedSells(warnings []matcher.Warning, recon *reconciliation.ReconciliationData) bool {
	changed := false

	for i, warn := range warnings {
		w.printf("  %d of %d: %s — %.0f shares sold %s, no buy record found\n",
			i+1, len(warnings), warn.Symbol, warn.Unmatched, warn.SellDate)

		// Smart recommendation for small quantities
		if warn.Unmatched < 20 {
			w.printf("          This is a small position. Skipping won't affect your scorecard much.\n")
		}

		w.printf("          [P]rovide buy details  [S]kip → ")

		choice := w.readChoice("ps")
		switch choice {
		case 'p':
			buyDate := w.readLine("          Buy date (YYYY-MM-DD): ")
			buyPrice := w.readLine("          Buy price per share: ")
			isin := w.readLine("          ISIN (or press Enter to skip): ")
			price := parseFloat(buyPrice)
			recon.ManualTrades = append(recon.ManualTrades, reconciliation.ManualTrade{
				Symbol:    warn.Symbol,
				ISIN:      isin,
				Date:      buyDate,
				TradeType: "buy",
				Quantity:  warn.Unmatched,
				Price:     price,
			})
			w.printf("          ✓ Added buy: %.0f shares @ ₹%.0f on %s\n\n", warn.Unmatched, price, buyDate)
			changed = true
		case 's':
			w.printf("          ✓ Skipped.\n\n")
		}
	}

	return changed
}

func (w *Wizard) printf(format string, args ...any) {
	fmt.Fprintf(w.out, format, args...)
}

func (w *Wizard) readLine(prompt string) string {
	w.printf("%s", prompt)
	line, _ := w.in.ReadString('\n')
	return strings.TrimSpace(line)
}

func (w *Wizard) readChoice(valid string) byte {
	for {
		line, _ := w.in.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if len(line) > 0 {
			for _, c := range valid {
				if line[0] == byte(c) {
					return byte(c)
				}
			}
		}
		w.printf("          Please enter one of [%s]: ", strings.ToUpper(valid))
	}
}

// formatLakhs formats a rupee amount in lakhs notation (e.g. 830000 → "8.3L").
func formatLakhs(amount float64) string {
	if math.Abs(amount) >= 100000 {
		return fmt.Sprintf("%.1fL", amount/100000)
	}
	return fmt.Sprintf("%.0f", amount)
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
