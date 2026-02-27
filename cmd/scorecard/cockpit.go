package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stock-scorecard/internal/cleandata"
	"stock-scorecard/internal/cockpit"
	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/tri"
)

func runCockpit(args []string) {
	home, _ := os.UserHomeDir()
	defaultSource := filepath.Join(home, "Downloads")

	fs := flag.NewFlagSet("cockpit", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "Data directory containing tri.csv and client subdirectories")
	client := fs.String("client", "", "Client ID (e.g. ZY7393) — required")
	outputPath := fs.String("output", "", "Path for output JSON file (default: cockpit_{clientID}.json)")
	reportDate := fs.String("report-date", "", "Report date (YYYY-MM-DD). Default: today or from config")
	sourceDir := fs.String("source", defaultSource, "Directory containing unrealized CSV (~/Downloads)")
	skipDeepDive := fs.Bool("skip-deep-dive", false, "Skip deep-dive analysis for failed stocks")

	fs.Parse(args)

	if *client == "" {
		fmt.Fprintf(os.Stderr, "Usage: stock-scorecard cockpit --client <id> [--data <dir>] [--output <path>]\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	clientID := strings.ToUpper(*client)
	clientDir := filepath.Join(*dataDir, clientID)

	// Default output path: cockpit_{clientID}.json
	if *outputPath == "" {
		*outputPath = fmt.Sprintf("./cockpit_%s.json", clientID)
	}

	// ── Step 1: Load cockpit config ──────────────────────────
	log.Printf("Loading cockpit config for %s...", clientID)
	cfg, err := cockpit.LoadConfig(*dataDir, clientID)
	if err != nil {
		log.Fatalf("load cockpit config: %v", err)
	}
	cfg.ClientID = clientID

	// Resolve tax rate from expected income or explicit setting
	resolvedRate := cfg.ResolveTaxRate()
	cfg.TaxRate = resolvedRate
	log.Printf("Tax rate: %.2f%% (income: %.0f)", resolvedRate, cfg.ExpectedTotalIncome)

	// Determine report date
	rDate := *reportDate
	if rDate == "" {
		if cfg.ReportDate != "" {
			rDate = cfg.ReportDate
		} else {
			rDate = time.Now().Format("2006-01-02")
		}
	}
	cfg.ReportDate = rDate
	log.Printf("Report date: %s", rDate)

	// ── Step 2: Load Nifty TRI ───────────────────────────────
	triPath := filepath.Join(*dataDir, "tri.csv")
	triIdx, err := tri.LoadTRI(triPath)
	if err != nil {
		log.Fatalf("load TRI from %s: %v", triPath, err)
	}
	log.Printf("Loaded TRI from %s", triPath)

	// ── Step 3: Load unrealized lots ─────────────────────────
	unrealizedPath := filepath.Join(*sourceDir, fmt.Sprintf("unrealized_%s.csv", clientID))
	positions, err := cleandata.ReadOpenPositions(unrealizedPath)
	if err != nil {
		log.Fatalf("read unrealized positions from %s: %v", unrealizedPath, err)
	}
	log.Printf("Loaded %d unrealized lots from %s", len(positions), unrealizedPath)

	// Inject manual lots from config
	for _, ml := range cfg.ManualLots {
		buyDt, err := time.Parse("2006-01-02", ml.BuyDate)
		if err != nil {
			log.Printf("WARNING: invalid manual lot buy_date %q: %v", ml.BuyDate, err)
			continue
		}
		positions = append(positions, matcher.OpenPosition{
			Symbol:   ml.Symbol,
			ISIN:     ml.ISIN,
			BuyDate:  buyDt,
			Quantity: float64(ml.Quantity),
			BuyPrice: ml.BuyPrice,
			Invested: math.Round(float64(ml.Quantity) * ml.BuyPrice),
		})
	}
	if len(cfg.ManualLots) > 0 {
		log.Printf("Added %d manual lots from config", len(cfg.ManualLots))
	}

	// ── Step 4: Get unique symbols ───────────────────────────
	symbolSet := make(map[string]bool)
	for _, p := range positions {
		symbolSet[p.Symbol] = true
	}
	var symbols []string
	for s := range symbolSet {
		symbols = append(symbols, s)
	}

	// ── Step 5: Fetch prices from Yahoo Finance ──────────────
	log.Printf("Fetching current prices...")
	pricer := cockpit.NewPricer(clientDir)
	prices := pricer.FetchAllPrices(symbols, rDate, cfg)
	log.Printf("Have prices for %d symbols", len(prices))

	// ── Step 6: Fetch stock TRI ──────────────────────────────
	log.Printf("Fetching stock TRI data...")
	stockTRI := pricer.FetchAllStockTRI(symbols, "2016-01-01", rDate, cfg)
	log.Printf("Have stock TRI for %d symbols", len(stockTRI))

	// ── Step 7: Enrich lots ──────────────────────────────────
	log.Printf("Enriching lots...")
	lotsBySymbol := cockpit.EnrichLots(positions, prices, triIdx, stockTRI, rDate, cfg)
	log.Printf("Enriched %d symbols", len(lotsBySymbol))

	// ── Step 8: F&O attribution ──────────────────────────────
	var fnoBySymbol map[string]int
	totalAttributed := 0
	totalUnattributed := 0

	fnoPath := filepath.Join(clientDir, fmt.Sprintf("fno_%s.csv", clientID))
	if _, err := os.Stat(fnoPath); err == nil {
		// Load F&O trades
		fnoTrades, err := cleandata.ReadFnOTrades(fnoPath)
		if err != nil {
			log.Fatalf("read F&O trades: %v", err)
		}
		log.Printf("Loaded %d F&O trades", len(fnoTrades))

		contracts := fno.ComputeContractPnLs(fnoTrades)

		// Load realized trades for attribution (to avoid over-attributing)
		var realizedTrades []matcher.RealizedTrade
		realizedPath := filepath.Join(*sourceDir, fmt.Sprintf("realized_%s.csv", clientID))
		if _, err := os.Stat(realizedPath); err == nil {
			realizedTrades, err = cleandata.ReadRealizedTrades(realizedPath)
			if err != nil {
				log.Printf("WARNING: Failed to read realized trades: %v", err)
			} else {
				log.Printf("Loaded %d realized trades for F&O attribution", len(realizedTrades))
			}
		}

		fnoBySymbol, totalAttributed, totalUnattributed = cockpit.AttributeFnO(
			contracts, lotsBySymbol, realizedTrades, rDate, stockTRI, cfg,
		)
		log.Printf("F&O: ₹%d attributed, ₹%d unattributed", totalAttributed, totalUnattributed)
	} else {
		log.Printf("No F&O file at %s, skipping F&O attribution", fnoPath)
	}

	// ── Step 9: Classify stocks ──────────────────────────────
	log.Printf("Classifying stocks...")
	passList, failList, tooEarlyList, noTestList := cockpit.ClassifyStocks(
		lotsBySymbol, cfg, stockTRI, rDate,
	)
	log.Printf("Pass: %d, Fail: %d, Too early: %d, No test: %d",
		len(passList), len(failList), len(tooEarlyList), len(noTestList))

	// ── Step 10: Build deep-dive cards ───────────────────────
	if !*skipDeepDive && len(failList) > 0 {
		cockpit.BuildDeepDiveCards(failList, lotsBySymbol, prices, pricer, triIdx, stockTRI, cfg)
	}

	// ── Step 11: Assemble + write JSON ───────────────────────
	result := cockpit.BuildCockpitJSON(
		rDate, lotsBySymbol,
		passList, failList, tooEarlyList, noTestList,
		cfg, fnoBySymbol, totalAttributed, totalUnattributed,
	)

	if err := cockpit.WriteCockpitJSON(*outputPath, result); err != nil {
		log.Fatalf("write cockpit JSON: %v", err)
	}
	log.Printf("Wrote cockpit to %s", *outputPath)

	// Print summary
	fmt.Println()
	fmt.Printf("  Cockpit for %s (report date: %s)\n", clientID, rDate)
	fmt.Printf("  Portfolio: ₹%.2f Cr invested, ₹%.2f Cr current\n",
		float64(result.Portfolio.TotalInvested)/10000000,
		float64(result.Portfolio.TotalCurrent)/10000000)
	fmt.Printf("  Pass: %d, Fail: %d, Too early: %d, No test: %d\n",
		len(passList), len(failList), len(tooEarlyList), len(noTestList))

	totalSurplus := 0
	for _, s := range passList {
		totalSurplus += s.Surplus
	}
	totalDeficit := 0
	for _, s := range failList {
		totalDeficit += s.Deficit
	}
	fmt.Printf("  Surplus: ₹%.1fL, Deficit: ₹%.1fL\n",
		float64(totalSurplus)/100000, float64(totalDeficit)/100000)
	fmt.Printf("  Output: %s\n", *outputPath)
	fmt.Println()
}
