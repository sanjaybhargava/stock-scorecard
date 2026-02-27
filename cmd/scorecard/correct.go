package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stock-scorecard/internal/cleandata"
	"stock-scorecard/internal/reconciliation"
)

func runCorrect(args []string) {
	fs := flag.NewFlagSet("correct", flag.ExitOnError)
	inputPath := fs.String("input", "", "Path to review CSV file (required)")
	dataDir := fs.String("data", "./data", "Data directory containing reconciliation files")

	fs.Parse(args)

	if *inputPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: stock-scorecard correct --input <review_CSV>\n")
		fmt.Fprintf(os.Stderr, "\nReads a review CSV, validates corrections, and saves to reconciliation.\n")
		os.Exit(1)
	}

	// Extract client ID from filename: review_{clientID}.csv
	base := filepath.Base(*inputPath)
	clientID := ""
	if strings.HasPrefix(base, "review_") && strings.HasSuffix(base, ".csv") {
		clientID = strings.TrimSuffix(strings.TrimPrefix(base, "review_"), ".csv")
	}
	if clientID == "" {
		log.Fatalf("  ✗ Could not extract client ID from filename %q (expected review_{clientID}.csv)", base)
	}

	// Read review CSV
	items, err := cleandata.ReadReviewCSV(*inputPath)
	if err != nil {
		log.Fatalf("  ✗ read review CSV: %v", err)
	}

	// Load reconciliation
	reconPath := filepath.Join(*dataDir, clientID, fmt.Sprintf("reconciliation_%s.json", clientID))
	var recon *reconciliation.ReconciliationData
	if _, err := os.Stat(reconPath); err == nil {
		recon, err = reconciliation.Load(reconPath)
		if err != nil {
			log.Fatalf("  ✗ load reconciliation: %v", err)
		}
	} else {
		recon = &reconciliation.ReconciliationData{ClientID: clientID}
	}

	// Process corrections
	corrected := 0
	skipped := 0
	errors := 0

	for _, item := range items {
		switch item.Status {
		case "corrected":
			// Validate buy_date
			buyDate, err := time.Parse("2006-01-02", item.BuyDate)
			if err != nil {
				fmt.Printf("  ✗ %s: invalid buy_date %q (expected YYYY-MM-DD)\n", item.Symbol, item.BuyDate)
				errors++
				continue
			}

			// Validate buy_price
			if item.BuyPrice <= 0 {
				fmt.Printf("  ✗ %s: buy_price must be > 0 (got %.2f)\n", item.Symbol, item.BuyPrice)
				errors++
				continue
			}

			// Validate buy_date < sell_date
			sellDate, _ := time.Parse("2006-01-02", item.SellDate)
			if !buyDate.Before(sellDate) {
				fmt.Printf("  ✗ %s: buy_date %s must be before sell_date %s\n", item.Symbol, item.BuyDate, item.SellDate)
				errors++
				continue
			}

			// Add manual buy trade to reconciliation
			recon.ManualTrades = append(recon.ManualTrades, reconciliation.ManualTrade{
				Symbol:    item.Symbol,
				ISIN:      item.ISIN,
				Date:      item.BuyDate,
				TradeType: "buy",
				Quantity:  item.Quantity,
				Price:     item.BuyPrice,
			})
			corrected++
			fmt.Printf("  ✓ %s: added buy %.0f shares @ ₹%.0f on %s\n", item.Symbol, item.Quantity, item.BuyPrice, item.BuyDate)

		case "skip":
			skipped++

		case "auto", "unresolved":
			// Unchanged — ignore
		}
	}

	if errors > 0 {
		fmt.Printf("\n  %d errors — fix and re-run.\n", errors)
		os.Exit(1)
	}

	if corrected == 0 {
		fmt.Println("\n  No corrections found. Edit the review CSV and set status to \"corrected\".")
		return
	}

	// Save updated reconciliation
	if err := os.MkdirAll(filepath.Dir(reconPath), 0755); err != nil {
		log.Fatalf("  ✗ create dir: %v", err)
	}
	if err := reconciliation.Save(reconPath, recon); err != nil {
		log.Fatalf("  ✗ save reconciliation: %v", err)
	}

	fmt.Printf("\n  ✓ %d corrections saved to reconciliation\n", corrected)
	if skipped > 0 {
		fmt.Printf("  %d items skipped\n", skipped)
	}
	fmt.Printf("\n  Re-run import to regenerate CSVs:\n")
	fmt.Printf("    stock-scorecard import\n\n")
}
