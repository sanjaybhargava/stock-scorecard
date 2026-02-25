package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/output"
	"stock-scorecard/internal/scorer"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

func main() {
	tradebooksDir := flag.String("tradebooks", "", "Directory containing Zerodha tradebook CSV files (required)")
	triPath := flag.String("tri", "", "Path to NIFTY 500 TRI Indexed CSV file (required)")
	outputPath := flag.String("output", "", "Path for output JSON file (required)")
	exclude := flag.String("exclude", "LIQUIDBEES,GOLDBEES", "Comma-separated symbols to skip")
	broker := flag.String("broker", "zerodha", "Broker format for parser selection")
	verbose := flag.Bool("verbose", false, "Print per-symbol FIFO summary to stderr")

	flag.Parse()

	if *tradebooksDir == "" || *triPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *broker != "zerodha" {
		log.Fatalf("unsupported broker: %s (only zerodha is supported)", *broker)
	}

	// Step 1: Parse tradebooks
	excludes := strings.Split(*exclude, ",")
	trades, err := tradebook.ParseDirectory(*tradebooksDir, excludes)
	if err != nil {
		log.Fatalf("parse tradebooks: %v", err)
	}
	log.Printf("Parsed %d consolidated trades", len(trades))

	// Step 2: Load TRI index
	triIdx, err := tri.LoadTRI(*triPath)
	if err != nil {
		log.Fatalf("load TRI: %v", err)
	}
	log.Printf("Loaded TRI index")

	// Step 3: FIFO matching
	realized, open, summaries, err := matcher.Match(trades, triIdx)
	if err != nil {
		log.Fatalf("FIFO match: %v", err)
	}
	log.Printf("Matched %d realized trades, %d open positions", len(realized), len(open))

	if *verbose {
		fmt.Fprintf(os.Stderr, "\n%-20s %10s %10s %10s %10s\n", "Symbol", "Bought", "Sold", "Matched", "Open")
		fmt.Fprintf(os.Stderr, "%s\n", strings.Repeat("-", 62))
		for _, s := range summaries {
			fmt.Fprintf(os.Stderr, "%-20s %10.0f %10.0f %10.0f %10.0f\n",
				s.Symbol, s.SharesBought, s.SharesSold, s.SharesMatched, s.SharesOpen)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Step 4: Score
	summary := scorer.Score(realized)
	log.Printf("Win rate: %d%%, Net alpha: ₹%d", summary.WinRate, int(summary.NetAlpha))

	// Step 5: Write JSON
	if err := output.WriteJSON(*outputPath, realized, open, summary); err != nil {
		log.Fatalf("write JSON: %v", err)
	}
	log.Printf("Wrote scorecard to %s", *outputPath)
}
