// Command serve is a self-contained stock-scorecard binary.
// It generates the scorecard from tradebook CSVs in ~/Downloads,
// serves the React UI on localhost, and opens the browser automatically.
//
// Usage:
//
//	./stock-scorecard-serve
//	./stock-scorecard-serve --port 9090
//	./stock-scorecard-serve --dir /path/to/tradebooks
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"stock-scorecard/internal/dividend"
	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/output"
	"stock-scorecard/internal/scorer"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

//go:embed embed/dist/*
var embeddedUI embed.FS

func main() {
	dir := flag.String("dir", "", "Directory containing tradebook CSVs and TRI file (default: ~/Downloads)")
	port := flag.Int("port", 0, "Port to serve on (default: auto-select)")
	exclude := flag.String("exclude", "LIQUIDBEES", "Comma-separated symbols to skip")
	noBrowser := flag.Bool("no-browser", false, "Don't auto-open the browser")
	flag.Parse()

	// Default to ~/Downloads
	dataDir := *dir
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Could not find home directory: %v", err)
		}
		dataDir = filepath.Join(home, "Downloads")
	}

	log.Println("=== Stock Scorecard ===")
	log.Printf("Reading data from: %s", dataDir)

	// Generate scorecard JSON in memory
	scorecardJSON, err := generateScorecard(dataDir, *exclude)
	if err != nil {
		log.Fatalf("Error generating scorecard: %v", err)
	}
	log.Println("Scorecard generated successfully")

	// Get the embedded UI filesystem (strip the embed/dist prefix)
	uiFS, err := fs.Sub(embeddedUI, "embed/dist")
	if err != nil {
		log.Fatalf("Error loading embedded UI: %v", err)
	}

	// Set up HTTP handlers
	mux := http.NewServeMux()

	// Serve scorecard.json from memory
	mux.HandleFunc("/scorecard.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(scorecardJSON)
	})

	// Serve embedded UI files
	mux.Handle("/", http.FileServer(http.FS(uiFS)))

	// Find an available port
	listenPort := *port
	if listenPort == 0 {
		listenPort = findFreePort()
	}

	addr := fmt.Sprintf("127.0.0.1:%d", listenPort)
	url := fmt.Sprintf("http://%s", addr)

	log.Printf("Serving scorecard at %s", url)
	log.Println("Press Ctrl+C to stop")

	// Open browser
	if !*noBrowser {
		go openBrowser(url)
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func generateScorecard(dataDir, excludeStr string) ([]byte, error) {
	// Step 1: Parse equity tradebooks
	excludes := strings.Split(excludeStr, ",")
	trades, err := tradebook.ParseDirectory(dataDir, excludes)
	if err != nil {
		return nil, fmt.Errorf("parse tradebooks: %w", err)
	}
	log.Printf("  Parsed %d consolidated equity trades", len(trades))

	// Step 2: Find and load TRI file
	triPath, err := findTRIFile(dataDir)
	if err != nil {
		return nil, fmt.Errorf("find TRI file: %w", err)
	}
	triIdx, err := tri.LoadTRI(triPath)
	if err != nil {
		return nil, fmt.Errorf("load TRI: %w", err)
	}
	log.Printf("  Loaded TRI index from %s", filepath.Base(triPath))

	// Step 2b: Load dividends if available
	var divIdx *dividend.DividendIndex
	divPath := filepath.Join(filepath.Dir(dataDir), "dividends.csv")
	// Also check in the data directory itself and current working dir
	for _, candidate := range []string{
		filepath.Join(dataDir, "dividends.csv"),
		divPath,
		"dividends.csv",
	} {
		if _, err := os.Stat(candidate); err == nil {
			divIdx, err = dividend.LoadDividends(candidate)
			if err != nil {
				log.Printf("  Warning: could not load dividends from %s: %v", candidate, err)
			} else {
				log.Printf("  Loaded dividends from %s", candidate)
			}
			break
		}
	}
	if divIdx == nil {
		log.Printf("  No dividends.csv found (optional)")
	}

	// Step 3: FIFO matching
	realized, open, _, warnings, err := matcher.Match(trades, triIdx, divIdx)
	if err != nil {
		return nil, fmt.Errorf("FIFO match: %w", err)
	}
	log.Printf("  Matched %d realized trades, %d open positions", len(realized), len(open))

	// Step 3b: F&O attribution (if F&O files exist)
	var unattributedFnO []fno.UnattributedFnO
	fnoTrades, err := fno.ParseDirectory(dataDir)
	if err == nil && len(fnoTrades) > 0 {
		log.Printf("  Parsed %d consolidated F&O trades", len(fnoTrades))
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
		log.Printf("  No F&O tradebooks found (optional)")
	}

	// Step 4: Score
	summary := scorer.Score(realized)
	log.Printf("  Win rate: %d%%, Net alpha: ₹%d", summary.WinRate, int(summary.NetAlpha))

	// Step 5: Build JSON in memory (reuse output package logic)
	// We write to a temp file and read it back, since WriteJSON writes to file
	tmpFile, err := os.CreateTemp("", "scorecard-*.json")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := output.WriteJSON(tmpPath, realized, open, warnings, summary, unattributedFnO); err != nil {
		return nil, fmt.Errorf("generate JSON: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read generated JSON: %w", err)
	}

	// Verify it's valid JSON
	var check json.RawMessage
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, fmt.Errorf("generated invalid JSON: %w", err)
	}

	return data, nil
}

// findTRIFile looks for the NIFTY 500 TRI CSV file in the given directory.
// Prefers NIFTY*TRI*.csv, falls back to any *TRI*.csv.
func findTRIFile(dir string) (string, error) {
	// Prefer NIFTY-specific TRI file
	niftyMatches, _ := filepath.Glob(filepath.Join(dir, "NIFTY*TRI*.csv"))
	if len(niftyMatches) > 0 {
		return niftyMatches[0], nil
	}
	// Fall back to any TRI file
	matches, err := filepath.Glob(filepath.Join(dir, "*TRI*.csv"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no *TRI*.csv file found in %s — please place NIFTY500_TRI_Indexed.csv there", dir)
	}
	return matches[0], nil
}

func findFreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 8080
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		log.Printf("Open %s in your browser", url)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Could not open browser: %v — open %s manually", err, url)
	}
}
