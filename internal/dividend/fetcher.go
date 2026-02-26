package dividend

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"
)

// FetchedDividend represents a single dividend event fetched from Yahoo Finance.
type FetchedDividend struct {
	Symbol string
	ExDate time.Time
	Amount float64 // split-adjusted per-share
}

// Fetch retrieves dividend history for the given tickers from Yahoo Finance.
// Falls back to invoking pull_dividends.py if the Go-native API call fails.
func Fetch(tickers []string) ([]FetchedDividend, error) {
	var all []FetchedDividend
	var failures []string

	for _, ticker := range tickers {
		divs, err := fetchYahoo(ticker)
		if err != nil {
			log.Printf("  %s: Yahoo API failed (%v), will try Python fallback", ticker, err)
			failures = append(failures, ticker)
			continue
		}
		all = append(all, divs...)
	}

	// If all tickers succeeded via API, we're done
	if len(failures) == 0 {
		return all, nil
	}

	// Try Python fallback for failed tickers
	log.Printf("Attempting Python fallback for %d failed tickers...", len(failures))
	fallbackDivs, err := fetchViaPython(failures)
	if err != nil {
		log.Printf("Python fallback also failed: %v", err)
		// Return what we have — partial results are better than none
		return all, nil
	}
	all = append(all, fallbackDivs...)
	return all, nil
}

// fetchYahoo retrieves dividends for a single ticker using Yahoo Finance chart API.
func fetchYahoo(ticker string) ([]FetchedDividend, error) {
	// Yahoo Finance chart API with dividend events
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s.NS?period1=0&period2=%d&events=div&interval=1mo",
		ticker, time.Now().Unix())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Parse Yahoo Finance chart response
	var result struct {
		Chart struct {
			Result []struct {
				Events struct {
					Dividends map[string]struct {
						Amount float64 `json:"amount"`
						Date   int64   `json:"date"`
					} `json:"dividends"`
				} `json:"events"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if result.Chart.Error != nil {
		return nil, fmt.Errorf("API error: %s", result.Chart.Error.Description)
	}

	if len(result.Chart.Result) == 0 {
		return nil, nil
	}

	divMap := result.Chart.Result[0].Events.Dividends
	if len(divMap) == 0 {
		return nil, nil
	}

	var divs []FetchedDividend
	for _, d := range divMap {
		t := time.Unix(d.Date, 0).UTC()
		divs = append(divs, FetchedDividend{
			Symbol: ticker,
			ExDate: t,
			Amount: d.Amount,
		})
	}

	sort.Slice(divs, func(i, j int) bool {
		return divs[i].ExDate.Before(divs[j].ExDate)
	})

	return divs, nil
}

// fetchViaPython invokes pull_dividends.py as a subprocess for the given tickers.
// It creates a temporary scorecard JSON with just the tickers and reads the output CSV.
func fetchViaPython(tickers []string) ([]FetchedDividend, error) {
	// Create a minimal scorecard JSON for the Python script
	tmpScorecard, err := os.CreateTemp("", "scorecard-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpScorecard.Name())

	// Build minimal JSON with just trades containing the tickers
	type minTrade struct {
		Ticker string `json:"ticker"`
	}
	type minScorecard struct {
		Trades []minTrade `json:"trades"`
	}
	sc := minScorecard{}
	for _, t := range tickers {
		sc.Trades = append(sc.Trades, minTrade{Ticker: t})
	}
	data, _ := json.Marshal(sc)
	tmpScorecard.Write(data)
	tmpScorecard.Close()

	// Output CSV
	tmpOutput, err := os.CreateTemp("", "dividends-*.csv")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpOutput.Name())
	tmpOutput.Close()

	// Run Python script
	cmd := exec.Command("python3", "scripts/pull_dividends.py",
		"--scorecard", tmpScorecard.Name(),
		"--output", tmpOutput.Name())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python script failed: %w", err)
	}

	// Read output CSV
	f, err := os.Open(tmpOutput.Name())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) < 3 {
		return nil, fmt.Errorf("unexpected CSV header: %v", header)
	}

	var divs []FetchedDividend
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		exDate, err := time.Parse("2006-01-02", row[1])
		if err != nil {
			continue
		}
		amount, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			continue
		}
		divs = append(divs, FetchedDividend{
			Symbol: row[0],
			ExDate: exDate,
			Amount: amount,
		})
	}

	return divs, nil
}

// SaveDividendsCSV writes fetched dividends to a CSV file in the standard format.
func SaveDividendsCSV(path string, divs []FetchedDividend) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"symbol", "ex_date", "amount"}); err != nil {
		return err
	}

	for _, d := range divs {
		if err := w.Write([]string{
			d.Symbol,
			d.ExDate.Format("2006-01-02"),
			strconv.FormatFloat(d.Amount, 'f', 4, 64),
		}); err != nil {
			return err
		}
	}

	return nil
}
