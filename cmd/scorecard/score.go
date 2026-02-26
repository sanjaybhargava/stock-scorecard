package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"stock-scorecard/internal/cleandata"
	"stock-scorecard/internal/dividend"
	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/output"
	"stock-scorecard/internal/reconciliation"
	"stock-scorecard/internal/scorer"
	"stock-scorecard/internal/tri"
)

func runScore(args []string) {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "Data directory containing tri.csv and client subdirectories")
	client := fs.String("client", "", "Client ID (e.g. BT2632) — required")
	outputPath := fs.String("output", "./scorecard.json", "Path for output JSON file")
	verbose := fs.Bool("verbose", false, "Print per-symbol FIFO summary to stderr")

	fs.Parse(args)

	if *client == "" {
		fmt.Fprintf(os.Stderr, "Usage: stock-scorecard score --data <dir> --client <id> --output <path>\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	clientID := strings.ToUpper(*client)

	clientDir := filepath.Join(*dataDir, clientID)

	// Load TRI (shared)
	triPath := filepath.Join(*dataDir, "tri.csv")
	triIdx, err := tri.LoadTRI(triPath)
	if err != nil {
		log.Fatalf("load TRI from %s: %v", triPath, err)
	}
	log.Printf("Loaded TRI from %s", triPath)

	// Load reconciliation
	reconPath := filepath.Join(clientDir, fmt.Sprintf("reconciliation_%s.json", clientID))
	var recon *reconciliation.ReconciliationData
	if _, err := os.Stat(reconPath); err == nil {
		recon, err = reconciliation.Load(reconPath)
		if err != nil {
			log.Fatalf("load reconciliation: %v", err)
		}
		log.Printf("Loaded reconciliation for %s", recon.ClientID)
	} else {
		recon = &reconciliation.ReconciliationData{ClientID: clientID}
		log.Printf("No reconciliation file found, using empty defaults")
	}

	// Load equity trades
	tradesPath := filepath.Join(clientDir, fmt.Sprintf("trades_%s.csv", clientID))
	trades, err := cleandata.ReadTrades(tradesPath)
	if err != nil {
		log.Fatalf("read trades from %s: %v", tradesPath, err)
	}
	log.Printf("Loaded %d equity trades from %s", len(trades), tradesPath)

	// Load dividends (optional)
	var divIdx *dividend.DividendIndex
	divPath := filepath.Join(clientDir, fmt.Sprintf("dividends_%s.csv", clientID))
	if _, err := os.Stat(divPath); err == nil {
		divIdx, err = dividend.LoadDividends(divPath)
		if err != nil {
			log.Fatalf("load dividends: %v", err)
		}
	} else {
		log.Printf("No dividends file found at %s (optional)", divPath)
	}

	// FIFO matching
	realized, open, summaries, warnings, err := matcher.Match(trades, triIdx, divIdx, recon)
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

	// F&O attribution (optional)
	var unattributedFnO []fno.UnattributedFnO
	fnoPath := filepath.Join(clientDir, fmt.Sprintf("fno_%s.csv", clientID))
	if _, err := os.Stat(fnoPath); err == nil {
		fnoTrades, err := cleandata.ReadFnOTrades(fnoPath)
		if err != nil {
			log.Fatalf("read F&O trades: %v", err)
		}
		log.Printf("Loaded %d F&O trades from %s", len(fnoTrades), fnoPath)

		contracts := fno.ComputeContractPnLs(fnoTrades)
		attribution, unattrib := fno.Attribute(contracts, realized)
		unattributedFnO = unattrib

		for idx, amount := range attribution {
			if idx < 0 || idx >= len(realized) {
				continue
			}
			rounded := math.Round(amount)
			realized[idx].OptionIncome += rounded
			realized[idx].EquityGL += rounded
		}
	} else {
		log.Printf("No F&O file found at %s (optional)", fnoPath)
	}

	// Score
	summary := scorer.Score(realized)
	log.Printf("Win rate: %d%%, Net alpha: ₹%d", summary.WinRate, int(summary.NetAlpha))

	// Write JSON
	if err := output.WriteJSON(*outputPath, realized, open, warnings, summary, unattributedFnO); err != nil {
		log.Fatalf("write JSON: %v", err)
	}
	log.Printf("Wrote scorecard to %s", *outputPath)
}
