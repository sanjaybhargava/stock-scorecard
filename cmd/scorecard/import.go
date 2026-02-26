package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"stock-scorecard/internal/cleandata"
	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/reconciliation"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
	"stock-scorecard/internal/wizard"
)

func runImport(args []string) {
	home, _ := os.UserHomeDir()
	defaultSource := filepath.Join(home, "Downloads")

	fs := flag.NewFlagSet("import", flag.ExitOnError)
	sourceDir := fs.String("source", defaultSource, "Directory containing raw Zerodha tradebook CSVs")
	outputDir := fs.String("output", "./data", "Output directory for clean data files")
	exclude := fs.String("exclude", "LIQUIDBEES", "Comma-separated symbols to skip")

	fs.Parse(args)

	// Welcome
	fmt.Println()
	fmt.Println("  ╭──────────────────────────────────────────────╮")
	fmt.Println("  │  Stock Scorecard                              │")
	fmt.Println("  ╰──────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("  Welcome! This tool creates a scorecard of your realised trades on Zerodha.")
	fmt.Println("  The first step is to get clean, matched data from your tradebooks.")
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()

	// ── Step 1: Import tradebooks ──────────────────────────────
	fmt.Println("  Step 1 of 3: Importing tradebooks")
	fmt.Println()

	excludes := splitCSV(*exclude)
	trades, clientID, err := tradebook.ParseDirectory(*sourceDir, excludes)
	if err != nil {
		log.Fatalf("  ✗ parse tradebooks: %v", err)
	}
	if clientID == "" {
		log.Fatalf("  ✗ Could not detect client ID from filenames (expected {clientID}_*.csv, e.g. BT2632_20200101_20201231.csv)")
	}

	// Parse F&O (optional, same source directory)
	fnoTrades, _ := fno.ParseDirectory(*sourceDir, nil)

	eqCount := len(trades)
	fnoCount := len(fnoTrades)
	fmt.Printf("  ✓ %d equity trades", eqCount)
	if fnoCount > 0 {
		fmt.Printf(", %d F&O trades", fnoCount)
	}
	fmt.Println(" (after removing duplicates)")
	fmt.Printf("  ✓ Client: %s\n", clientID)
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()

	// ── Prepare TRI + reconciliation ───────────────────────────
	sharedTRI := filepath.Join(*outputDir, "tri.csv")
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("  ✗ create output dir: %v", err)
	}
	if _, err := os.Stat(sharedTRI); os.IsNotExist(err) {
		if err := os.WriteFile(sharedTRI, embeddedTRI, 0644); err != nil {
			log.Fatalf("  ✗ write TRI: %v", err)
		}
	}
	triIdx, err := tri.LoadTRI(sharedTRI)
	if err != nil {
		log.Fatalf("  ✗ load TRI: %v", err)
	}

	clientDir := filepath.Join(*outputDir, clientID)
	if err := os.MkdirAll(clientDir, 0755); err != nil {
		log.Fatalf("  ✗ create client dir: %v", err)
	}

	reconPath := filepath.Join(clientDir, fmt.Sprintf("reconciliation_%s.json", clientID))
	var recon *reconciliation.ReconciliationData
	if _, err := os.Stat(reconPath); err == nil {
		recon, err = reconciliation.Load(reconPath)
		if err != nil {
			log.Fatalf("  ✗ load reconciliation: %v", err)
		}
	} else {
		recon = &reconciliation.ReconciliationData{ClientID: clientID}
	}

	// ── Step 2: Match trades + confirm open positions ──────────
	fmt.Println("  Step 2 of 3: Matching trades and confirming open positions")
	fmt.Println()
	fmt.Println("  Matching your buy and sell trades using FIFO (first-in, first-out)...")

	realized, open, _, warnings, err := matcher.Match(trades, triIdx, nil, recon)
	if err != nil {
		log.Fatalf("  ✗ FIFO match: %v", err)
	}
	fmt.Printf("  ✓ %d trades matched\n", len(realized))

	if len(open) > 0 {
		// Sort open positions by invested amount (largest first)
		sort.Slice(open, func(i, j int) bool {
			return open[i].Invested > open[j].Invested
		})

		fmt.Println()
		fmt.Printf("  We think you have %d open positions (bought but not yet sold).\n", len(open))
		fmt.Println("  Please confirm each one — are you still holding, or did you sell elsewhere?")
		fmt.Println()

		wiz := wizard.New(os.Stdin, os.Stdout)
		if wiz.ReconcileOpenPositions(open, recon) {
			if err := reconciliation.Save(reconPath, recon); err != nil {
				log.Fatalf("  ✗ save reconciliation: %v", err)
			}
			realized, open, _, warnings, err = matcher.Match(trades, triIdx, nil, recon)
			if err != nil {
				log.Fatalf("  ✗ FIFO re-match: %v", err)
			}
		}

		fmt.Printf("  ✓ Open positions confirmed! %d realised trades, %d still open.\n", len(realized), len(open))
	} else {
		fmt.Println("  ✓ No open positions — all trades fully matched!")
	}

	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()

	// ── Step 3: Resolve unmatched sells ────────────────────────
	fmt.Println("  Step 3 of 3: Resolving unmatched sells")
	fmt.Println()

	if len(warnings) > 0 {
		fmt.Printf("  We found %d sells where the original buy is missing from your tradebooks.\n", len(warnings))
		fmt.Println("  This usually means you bought these before your Zerodha tradebook history starts.")
		fmt.Println("  We'll go through them one by one — you can provide the buy details or skip.")
		fmt.Println()

		wiz := wizard.New(os.Stdin, os.Stdout)
		if wiz.ReconcileUnmatchedSells(warnings, recon) {
			if err := reconciliation.Save(reconPath, recon); err != nil {
				log.Fatalf("  ✗ save reconciliation: %v", err)
			}
			realized, open, _, _, err = matcher.Match(trades, triIdx, nil, recon)
			if err != nil {
				log.Fatalf("  ✗ FIFO re-match: %v", err)
			}
		}

		fmt.Println("  ✓ Unmatched sells resolved!")
	} else {
		fmt.Println("  ✓ No unmatched sells — all buy records present!")
	}

	// ── Write clean data files ─────────────────────────────────
	tradesPath := filepath.Join(clientDir, fmt.Sprintf("trades_%s.csv", clientID))
	if err := cleandata.WriteTrades(tradesPath, trades); err != nil {
		log.Fatalf("  ✗ write trades: %v", err)
	}
	if fnoCount > 0 {
		fnoPath := filepath.Join(clientDir, fmt.Sprintf("fno_%s.csv", clientID))
		if err := cleandata.WriteFnOTrades(fnoPath, fnoTrades); err != nil {
			log.Fatalf("  ✗ write F&O trades: %v", err)
		}
	}
	if err := reconciliation.Save(reconPath, recon); err != nil {
		log.Fatalf("  ✗ save reconciliation: %v", err)
	}

	// ── Done ───────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  ✓ We now have clean, matched data on your realised trades!")
	fmt.Println()
	fmt.Printf("  Client %s: %d realised trades, %d open positions\n", clientID, len(realized), len(open))
	fmt.Printf("  Data saved to %s/\n", clientDir)
	fmt.Println()
	fmt.Printf("  Next: Run \"stock-scorecard score --client %s\" to generate your scorecard.\n", clientID)
	fmt.Println()
}
