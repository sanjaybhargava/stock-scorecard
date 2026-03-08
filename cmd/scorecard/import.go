package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"stock-scorecard/internal/cleandata"
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

func runImport(args []string) {
	home, _ := os.UserHomeDir()
	defaultSource := filepath.Join(home, "Downloads")

	fs := flag.NewFlagSet("import", flag.ExitOnError)
	sourceDir := fs.String("source", defaultSource, "Directory containing raw Zerodha tradebook CSVs")
	outputDir := fs.String("output", "./data", "Output directory for clean data files")
	exclude := fs.String("exclude", "LIQUIDBEES", "Comma-separated symbols to skip")
	clientFlag := fs.String("client", "", "Client ID to filter files (e.g. ZY7393). Required when multiple clients' files are in source dir")
	wizardMode := fs.Bool("wizard", false, "Run interactive wizard for open positions and unmatched sells")

	fs.Parse(args)

	// Welcome
	fmt.Println()
	fmt.Println("  ╭──────────────────────────────────────────────╮")
	fmt.Println("  │  Stock Scorecard                              │")
	fmt.Println("  ╰──────────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("  Welcome! This tool creates a scorecard of your realised trades on Zerodha.")
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()

	// ── Step 1: Import tradebooks ──────────────────────────────
	if *wizardMode {
		fmt.Println("  Step 1 of 3: Importing tradebooks")
	} else {
		fmt.Println("  Step 1 of 2: Importing tradebooks")
	}
	fmt.Println()

	excludes := splitCSV(*exclude)
	trades, clientID, err := tradebook.ParseDirectory(*sourceDir, excludes, *clientFlag)
	if err != nil {
		log.Fatalf("  ✗ parse tradebooks: %v", err)
	}
	// Prefer --client flag over auto-detected ID from filenames
	if *clientFlag != "" {
		clientID = strings.ToUpper(*clientFlag)
	}
	if clientID == "" {
		log.Fatalf("  ✗ Could not detect client ID from filenames (expected {clientID}_*.csv, e.g. BT2632_20200101_20201231.csv). Use --client to specify.")
	}

	// Parse F&O (optional, same source directory)
	fnoTrades, _ := fno.ParseDirectory(*sourceDir, nil, clientID)

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

	if *wizardMode {
		runInteractiveImport(trades, fnoTrades, triIdx, recon, reconPath, clientID, clientDir, *sourceDir, fnoCount)
	} else {
		runNonInteractiveImport(trades, fnoTrades, triIdx, recon, reconPath, clientID, clientDir, *sourceDir, fnoCount)
	}
}

func runNonInteractiveImport(
	trades []tradebook.ConsolidatedTrade,
	fnoTrades []fno.FnOTrade,
	triIdx *tri.TRIIndex,
	recon *reconciliation.ReconciliationData,
	reconPath, clientID, clientDir, sourceDir string,
	fnoCount int,
) {
	fmt.Println("  Step 2 of 2: Matching trades")
	fmt.Println()

	// ── Fetch dividends ──────────────────────────────────────
	fmt.Println("  ✓ Fetching dividends...")
	tickers := uniqueTickers(trades)
	fetchedDivs, err := dividend.Fetch(tickers)
	if err != nil {
		log.Printf("  ! Dividend fetch failed: %v (continuing without dividends)", err)
	}
	var divIdx *dividend.DividendIndex
	if len(fetchedDivs) > 0 {
		divIdx = dividend.BuildIndex(fetchedDivs)
		// Save dividends CSV for future score runs
		divPath := filepath.Join(clientDir, fmt.Sprintf("dividends_%s.csv", clientID))
		if err := dividend.SaveDividendsCSV(divPath, fetchedDivs); err != nil {
			log.Printf("  ! Could not save dividends CSV: %v", err)
		}
		fmt.Printf("  ✓ %d dividend events for %d tickers\n", len(fetchedDivs), len(tickers))
	} else {
		fmt.Printf("  ✓ No dividends found for %d tickers\n", len(tickers))
	}

	// ── FIFO matching ────────────────────────────────────────
	fmt.Println("  ✓ FIFO matching...")

	realized, open, _, warnings, err := matcher.Match(trades, triIdx, divIdx, recon)
	if err != nil {
		log.Fatalf("  ✗ FIFO match: %v", err)
	}

	// Transfer-in detection on unmatched sells
	var transferInRealized []matcher.RealizedTrade
	var remainingWarnings []matcher.Warning
	if len(warnings) > 0 {
		transferInRealized, remainingWarnings = matcher.DetectTransferIns(warnings, trades, triIdx, divIdx)
		if len(transferInRealized) > 0 {
			fmt.Printf("  ✓ Transfer-in detection... (%d transfer-ins auto-matched)\n", len(transferInRealized))
		}
	} else {
		remainingWarnings = warnings
	}

	// Combine all realized trades (Tier 1 from FIFO + Tier 2 from transfer-in)
	allRealized := append(realized, transferInRealized...)

	// ── F&O attribution ──────────────────────────────────────
	var unattributedFnO []fno.UnattributedFnO
	if len(fnoTrades) > 0 {
		fmt.Println("  ✓ F&O attribution...")
		contracts := fno.ComputeContractPnLs(fnoTrades)
		attribution, unattrib := fno.Attribute(contracts, allRealized)
		unattributedFnO = unattrib

		for idx, amount := range attribution {
			if idx < 0 || idx >= len(allRealized) {
				continue
			}
			rounded := math.Round(amount)
			allRealized[idx].OptionIncome += rounded
			allRealized[idx].EquityGL += rounded
		}

		totalAttributed := 0.0
		for _, v := range attribution {
			totalAttributed += v
		}
		fmt.Printf("  ✓ %d contracts, ₹%s attributed to equity trades\n",
			len(contracts), wizard.FormatLakhs(totalAttributed))
		if len(unattributedFnO) > 0 {
			totalUnattrib := 0.0
			for _, u := range unattributedFnO {
				totalUnattrib += u.NetPnL
			}
			fmt.Printf("  ! %d underlyings unattributed (₹%s) — index options or no equity position\n",
				len(unattributedFnO), wizard.FormatLakhs(totalUnattrib))
		}
	}

	// Compute coverage stats
	totalSellValue := 0.0
	totalSells := 0
	for _, w := range warnings {
		totalSellValue += w.Unmatched * w.SellPrice
		totalSells++
	}
	// Add matched sell value
	matchedSellValue := 0.0
	for _, r := range allRealized {
		matchedSellValue += r.SaleValue
	}
	// Total includes both matched + unmatched
	totalSellValue += matchedSellValue
	totalSells += len(allRealized)

	matchedSells := len(allRealized)

	pct := 0.0
	if totalSellValue > 0 {
		pct = matchedSellValue / totalSellValue * 100
	}

	fmt.Printf("  ✓ %d of %d sells matched (₹%s of ₹%s — %.1f%% by value)\n",
		matchedSells, totalSells,
		wizard.FormatLakhs(matchedSellValue), wizard.FormatLakhs(totalSellValue), pct)

	// Build review items from Tier 2 trades + remaining warnings (Tier 3)
	var reviewItems []cleandata.ReviewItem
	intelligentCount := 0
	intelligentValue := 0.0
	for _, rt := range transferInRealized {
		sellValue := rt.SaleValue
		reviewItems = append(reviewItems, cleandata.ReviewItem{
			Tier:      2,
			Status:    "auto",
			Symbol:    rt.Symbol,
			ISIN:      rt.ISIN,
			SellDate:  rt.SellDate.Format("2006-01-02"),
			SellPrice: rt.SellPrice,
			Quantity:  rt.Quantity,
			SellValue: sellValue,
			BuyDate:   rt.BuyDate.Format("2006-01-02"),
			BuyPrice:  rt.BuyPrice,
			Reason:    rt.TierReason,
		})
		intelligentCount++
		intelligentValue += sellValue
	}

	unresolvedCount := 0
	unresolvedValue := 0.0
	for _, w := range remainingWarnings {
		sellValue := math.Round(w.Unmatched * w.SellPrice)
		reviewItems = append(reviewItems, cleandata.ReviewItem{
			Tier:      3,
			Status:    "unresolved",
			Symbol:    w.Symbol,
			ISIN:      w.ISIN,
			SellDate:  w.SellDate,
			SellPrice: w.SellPrice,
			Quantity:  w.Unmatched,
			SellValue: sellValue,
			Reason:    "no_buy_found",
		})
		unresolvedCount++
		unresolvedValue += sellValue
	}

	if intelligentCount > 0 {
		fmt.Printf("\n  %d intelligent matches (₹%s) — review in review_%s.csv\n", intelligentCount, wizard.FormatLakhs(intelligentValue), clientID)
	}
	if unresolvedCount > 0 {
		fmt.Printf("  %d unmatched sells (₹%s) — needs manual input\n", unresolvedCount, wizard.FormatLakhs(unresolvedValue))
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

	// Write user-facing CSVs to source dir
	realizedPath := filepath.Join(sourceDir, fmt.Sprintf("realized_%s.csv", clientID))
	if err := cleandata.WriteRealizedTrades(realizedPath, allRealized); err != nil {
		log.Fatalf("  ✗ write realized trades: %v", err)
	}
	unrealizedPath := filepath.Join(sourceDir, fmt.Sprintf("unrealized_%s.csv", clientID))
	if err := cleandata.WriteOpenPositions(unrealizedPath, open); err != nil {
		log.Fatalf("  ✗ write open positions: %v", err)
	}

	// Write review CSV (only if there are items to review)
	reviewPath := filepath.Join(sourceDir, fmt.Sprintf("review_%s.csv", clientID))
	if len(reviewItems) > 0 {
		summary := cleandata.CoverageSummary{
			TotalSells:       totalSells,
			MatchedSells:     matchedSells,
			TotalSellValue:   totalSellValue,
			MatchedSellValue: matchedSellValue,
			IntelligentCount: intelligentCount,
			IntelligentValue: intelligentValue,
			UnresolvedCount:  unresolvedCount,
			UnresolvedValue:  unresolvedValue,
		}
		if err := cleandata.WriteReviewCSV(reviewPath, reviewItems, summary); err != nil {
			log.Fatalf("  ✗ write review CSV: %v", err)
		}
	}

	// ── Score ─────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ✓ Scoring...")
	summary := scorer.Score(allRealized)

	// Include unattributed F&O in overall alpha — it's real income earned
	totalUnattribFnO := 0.0
	for _, u := range unattributedFnO {
		totalUnattribFnO += u.NetPnL
	}
	summary.TotalMyReturn += math.Round(totalUnattribFnO)
	summary.NetAlpha += math.Round(totalUnattribFnO)

	scorecardPath := filepath.Join(sourceDir, fmt.Sprintf("scorecard_%s.json", clientID))
	if err := output.WriteJSON(scorecardPath, allRealized, open, remainingWarnings, summary, unattributedFnO); err != nil {
		log.Fatalf("  ✗ write scorecard: %v", err)
	}
	fmt.Printf("  ✓ Win rate: %d%%, Net alpha: ₹%s\n", summary.WinRate, wizard.FormatLakhs(summary.NetAlpha))

	// ── Done ───────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  Done!")
	fmt.Printf("    • %s  (scorecard)\n", scorecardPath)
	fmt.Printf("    • %s    (%d trades)\n", realizedPath, len(allRealized))
	if len(reviewItems) > 0 {
		fmt.Printf("    • %s      (%d items to review)\n", reviewPath, len(reviewItems))
	}
	fmt.Printf("    • %s  (%d open positions)\n", unrealizedPath, len(open))
	fmt.Println()
	if len(reviewItems) > 0 {
		fmt.Printf("  Tip:  Edit review_%s.csv to fix unresolved items, then run:\n", clientID)
		fmt.Printf("        stock-scorecard correct --input %s\n", reviewPath)
		fmt.Println()
	}

	// ── Auto-run cockpit for unrealized portfolio ────────────
	var cockpitData []byte
	if len(open) > 0 {
		fmt.Println("  ✓ Running cockpit for unrealized portfolio...")
		cockpitArgs := []string{"--client", clientID, "--data", filepath.Dir(filepath.Dir(tradesPath)), "--source", sourceDir, "--skip-deep-dive"}
		cockpitPath := filepath.Join(sourceDir, fmt.Sprintf("cockpit_%s.json", clientID))
		cockpitArgs = append(cockpitArgs, "--output", cockpitPath)
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("  ! Cockpit failed: %v (scorecard still available)\n", r)
				}
			}()
			runCockpit(cockpitArgs)
			var readErr error
			cockpitData, readErr = os.ReadFile(cockpitPath)
			if readErr != nil {
				fmt.Printf("  ! Could not read cockpit JSON: %v\n", readErr)
			}
		}()
	}

	// ── Auto-view in browser ─────────────────────────────────
	scorecardData, err := os.ReadFile(scorecardPath)
	if err != nil {
		log.Fatalf("  ✗ read scorecard for viewer: %v", err)
	}
	startViewServer(scorecardData, cockpitData, clientID)
}

func runInteractiveImport(
	trades []tradebook.ConsolidatedTrade,
	fnoTrades []fno.FnOTrade,
	triIdx *tri.TRIIndex,
	recon *reconciliation.ReconciliationData,
	reconPath, clientID, clientDir, sourceDir string,
	fnoCount int,
) {
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
	realizedPath := filepath.Join(sourceDir, fmt.Sprintf("realized_%s.csv", clientID))
	if err := cleandata.WriteRealizedTrades(realizedPath, realized); err != nil {
		log.Fatalf("  ✗ write realized trades: %v", err)
	}
	unrealizedPath := filepath.Join(sourceDir, fmt.Sprintf("unrealized_%s.csv", clientID))
	if err := cleandata.WriteOpenPositions(unrealizedPath, open); err != nil {
		log.Fatalf("  ✗ write open positions: %v", err)
	}

	// ── Done ───────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  ✓ We now have clean, matched data on your realised trades!")
	fmt.Println()
	fmt.Printf("  Client %s: %d realised trades, %d open positions\n", clientID, len(realized), len(open))
	fmt.Println()
	fmt.Println("  Your CSVs (open in Excel/Numbers):")
	fmt.Printf("    • %s\n", realizedPath)
	fmt.Printf("    • %s\n", unrealizedPath)
	fmt.Println()
	fmt.Printf("  Next: Run \"stock-scorecard score --client %s\" to generate your scorecard.\n", clientID)
	fmt.Println()
}

// uniqueTickers returns deduplicated ticker symbols from consolidated trades.
func uniqueTickers(trades []tradebook.ConsolidatedTrade) []string {
	seen := make(map[string]bool)
	var tickers []string
	for _, t := range trades {
		if !seen[t.Symbol] {
			seen[t.Symbol] = true
			tickers = append(tickers, t.Symbol)
		}
	}
	return tickers
}
