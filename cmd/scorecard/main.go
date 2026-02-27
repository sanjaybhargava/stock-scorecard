package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"stock-scorecard/internal/dividend"
	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/output"
	"stock-scorecard/internal/reconciliation"
	"stock-scorecard/internal/scorer"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
	"stock-scorecard/internal/wizard"
)

//go:embed tri_embedded.csv
var embeddedTRI []byte

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			runImport(os.Args[2:])
			return
		case "score":
			runScore(os.Args[2:])
			return
		case "correct":
			runCorrect(os.Args[2:])
			return
		case "cockpit":
			runCockpit(os.Args[2:])
			return
		case "legacy":
			legacyMain()
			return
		}
	}

	// Default: run import wizard (no subcommand)
	runImport(os.Args[1:])
}

func legacyMain() {
	tradebooksDir := flag.String("tradebooks", "", "Directory containing Zerodha tradebook CSV files (required)")
	triPath := flag.String("tri", "", "Path to NIFTY 500 TRI Indexed CSV file (required)")
	outputPath := flag.String("output", "", "Path for output JSON file (required)")
	dividendsPath := flag.String("dividends", "", "Path to dividends CSV (optional, from pull_dividends.py)")
	fnoDir := flag.String("fno", "", "Directory containing F&O tradebook CSVs (optional)")
	reconPath := flag.String("reconciliation", "", "Path to reconciliation JSON (optional; uses built-in defaults if omitted)")
	wizardMode := flag.Bool("wizard", false, "Run interactive reconciliation wizard after FIFO matching")
	exclude := flag.String("exclude", "LIQUIDBEES", "Comma-separated symbols to skip")
	broker := flag.String("broker", "zerodha", "Broker format for parser selection")
	verbose := flag.Bool("verbose", false, "Print per-symbol FIFO summary to stderr")

	flag.Parse()

	if *tradebooksDir == "" || *triPath == "" || *outputPath == "" {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  stock-scorecard import [--source ~/Downloads] [--output ./data]\n")
		fmt.Fprintf(os.Stderr, "  stock-scorecard score  --data <dir> --client <id> [--output <path>]\n")
		fmt.Fprintf(os.Stderr, "\nLegacy mode (all-in-one):\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *broker != "zerodha" {
		log.Fatalf("unsupported broker: %s (only zerodha is supported)", *broker)
	}

	// Step 1: Parse tradebooks
	excludes := splitCSV(*exclude)
	trades, clientID, err := tradebook.ParseDirectory(*tradebooksDir, excludes)
	if err != nil {
		log.Fatalf("parse tradebooks: %v", err)
	}
	if clientID != "" {
		log.Printf("Detected client ID: %s", clientID)
	}
	log.Printf("Parsed %d consolidated trades", len(trades))

	// Load reconciliation data (splits, demergers, manual trades, F&O renames)
	var recon *reconciliation.ReconciliationData
	if *reconPath != "" {
		recon, err = reconciliation.Load(*reconPath)
		if err != nil {
			log.Fatalf("load reconciliation: %v", err)
		}
		log.Printf("Loaded reconciliation for client %s (%d splits, %d demergers, %d manual trades, %d F&O renames)",
			recon.ClientID, len(recon.Splits), len(recon.Demergers), len(recon.ManualTrades), len(recon.FnORenames))
	} else if clientID != "" {
		// Auto-load reconciliation file if it exists next to output
		autoPath := filepath.Join(filepath.Dir(*outputPath), fmt.Sprintf("reconciliation_%s.json", clientID))
		if _, statErr := os.Stat(autoPath); statErr == nil {
			recon, err = reconciliation.Load(autoPath)
			if err != nil {
				log.Fatalf("auto-load reconciliation %s: %v", autoPath, err)
			}
			log.Printf("Auto-loaded reconciliation from %s", autoPath)
		} else {
			recon = reconciliation.Default()
			log.Printf("Using built-in reconciliation defaults (client %s)", recon.ClientID)
		}
	} else {
		recon = reconciliation.Default()
		log.Printf("Using built-in reconciliation defaults (client %s)", recon.ClientID)
	}

	// Step 2: Load TRI index
	triIdx, err := tri.LoadTRI(*triPath)
	if err != nil {
		log.Fatalf("load TRI: %v", err)
	}
	log.Printf("Loaded TRI index")

	// Step 2b: Load dividends (optional)
	var divIdx *dividend.DividendIndex
	if *dividendsPath != "" {
		divIdx, err = dividend.LoadDividends(*dividendsPath)
		if err != nil {
			log.Fatalf("load dividends: %v", err)
		}
	} else {
		log.Printf("Note: run with --dividends for total return including dividend income")
	}

	// Step 3: FIFO matching
	realized, open, summaries, warnings, err := matcher.Match(trades, triIdx, divIdx, recon)
	if err != nil {
		log.Fatalf("FIFO match: %v", err)
	}
	log.Printf("Matched %d realized trades, %d open positions", len(realized), len(open))

	// Step 3a: Interactive reconciliation wizard (optional)
	if *wizardMode && (len(open) > 0 || len(warnings) > 0) {
		wiz := wizard.New(os.Stdin, os.Stdout)
		wizClientID := clientID
		if wizClientID == "" {
			wizClientID = recon.ClientID
		}
		changed := wiz.ReconcileOpenPositions(open, recon) || wiz.ReconcileUnmatchedSells(warnings, recon)
		if changed {
			// Save updated reconciliation
			savePath := *reconPath
			if savePath == "" {
				savePath = filepath.Join(filepath.Dir(*outputPath), fmt.Sprintf("reconciliation_%s.json", wizClientID))
			}
			recon.ClientID = wizClientID
			if err := reconciliation.Save(savePath, recon); err != nil {
				log.Fatalf("save reconciliation: %v", err)
			}
			log.Printf("Saved reconciliation to %s", savePath)

			// Re-run FIFO with updated data
			log.Printf("Re-running FIFO with reconciliation fixes...")
			realized, open, summaries, warnings, err = matcher.Match(trades, triIdx, divIdx, recon)
			if err != nil {
				log.Fatalf("FIFO re-match: %v", err)
			}
			log.Printf("Re-matched %d realized trades, %d open positions", len(realized), len(open))
		}
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "\n%-20s %10s %10s %10s %10s\n", "Symbol", "Bought", "Sold", "Matched", "Open")
		fmt.Fprintf(os.Stderr, "%s\n", strings.Repeat("-", 62))
		for _, s := range summaries {
			fmt.Fprintf(os.Stderr, "%-20s %10.0f %10.0f %10.0f %10.0f\n",
				s.Symbol, s.SharesBought, s.SharesSold, s.SharesMatched, s.SharesOpen)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Step 3b: F&O option income attribution (optional)
	var unattributedFnO []fno.UnattributedFnO
	if *fnoDir != "" {
		fnoTrades, err := fno.ParseDirectory(*fnoDir, recon.FnORenames)
		if err != nil {
			log.Fatalf("parse F&O tradebooks: %v", err)
		}
		log.Printf("Parsed %d consolidated F&O trades", len(fnoTrades))

		contracts := fno.ComputeContractPnLs(fnoTrades)
		attribution, unattrib := fno.Attribute(contracts, realized)
		unattributedFnO = unattrib

		for idx, amount := range attribution {
			if idx < 0 || idx >= len(realized) {
				log.Printf("WARNING: F&O attribution index %d out of range (0..%d), skipping", idx, len(realized)-1)
				continue
			}
			rounded := math.Round(amount)
			realized[idx].OptionIncome += rounded
			realized[idx].EquityGL += rounded
		}
	} else {
		log.Printf("Note: run with --fno for total return including F&O option income")
	}

	// Step 4: Score
	summary := scorer.Score(realized)

	// Include unattributed F&O in overall alpha — it's real income earned
	totalUnattribFnO := 0.0
	for _, u := range unattributedFnO {
		totalUnattribFnO += u.NetPnL
	}
	summary.TotalMyReturn += math.Round(totalUnattribFnO)
	summary.NetAlpha += math.Round(totalUnattribFnO)

	log.Printf("Win rate: %d%%, Net alpha: ₹%d", summary.WinRate, int(summary.NetAlpha))

	// Step 5: Write JSON
	if err := output.WriteJSON(*outputPath, realized, open, warnings, summary, unattributedFnO); err != nil {
		log.Fatalf("write JSON: %v", err)
	}
	log.Printf("Wrote scorecard to %s", *outputPath)
}

// splitCSV splits a comma-separated string into trimmed tokens.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
