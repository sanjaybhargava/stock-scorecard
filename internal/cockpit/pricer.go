package cockpit

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Pricer fetches and caches stock prices from Yahoo Finance.
type Pricer struct {
	clientDir string
	client    *http.Client
	lastFetch time.Time
}

// NewPricer creates a pricer that caches data in the given client directory.
func NewPricer(clientDir string) *Pricer {
	return &Pricer{
		clientDir: clientDir,
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// yahooChart is the response structure from Yahoo Finance chart API.
type yahooChart struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []interface{} `json:"close"`
				} `json:"quote"`
				AdjClose []struct {
					AdjClose []interface{} `json:"adjclose"`
				} `json:"adjclose"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

// rateLimit ensures at least 500ms between Yahoo Finance API calls.
func (p *Pricer) rateLimit() {
	if !p.lastFetch.IsZero() {
		elapsed := time.Since(p.lastFetch)
		if elapsed < 500*time.Millisecond {
			time.Sleep(500*time.Millisecond - elapsed)
		}
	}
	p.lastFetch = time.Now()
}

// fetchChart calls the Yahoo Finance chart API.
func (p *Pricer) fetchChart(ticker string, period1, period2 int64) (*yahooChart, error) {
	p.rateLimit()

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d",
		ticker, period1, period2)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var chart yahooChart
	if err := json.Unmarshal(body, &chart); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if chart.Chart.Error != nil {
		return nil, fmt.Errorf("API error: %v", chart.Chart.Error)
	}
	if len(chart.Chart.Result) == 0 {
		return nil, fmt.Errorf("no results for %s", ticker)
	}

	return &chart, nil
}

// FetchCurrentPrice fetches the closing price for a ticker on or near reportDate.
// Uses a 7-day window and returns the nearest price on or before reportDate.
func (p *Pricer) FetchCurrentPrice(ticker, reportDate string) (float64, error) {
	dt, err := time.Parse("2006-01-02", reportDate)
	if err != nil {
		return 0, err
	}
	period1 := dt.AddDate(0, 0, -7).Unix()
	period2 := dt.AddDate(0, 0, 1).Unix()

	chart, err := p.fetchChart(ticker, period1, period2)
	if err != nil {
		return 0, err
	}

	result := chart.Chart.Result[0]
	if len(result.Timestamp) == 0 {
		return 0, fmt.Errorf("no data points for %s", ticker)
	}

	closes := result.Indicators.Quote[0].Close
	var bestPrice float64
	var bestDate string
	for i, ts := range result.Timestamp {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		d := time.Unix(ts, 0).UTC().Format("2006-01-02")
		if d <= reportDate {
			price, ok := toFloat64(closes[i])
			if ok {
				bestPrice = price
				bestDate = d
			}
		}
	}

	if bestDate == "" {
		return 0, fmt.Errorf("no price on or before %s for %s", reportDate, ticker)
	}
	return bestPrice, nil
}

// FetchStockTRI fetches Adjusted Close data from Yahoo Finance and indexes it to 100.
// Returns a map of date → TRI value.
func (p *Pricer) FetchStockTRI(ticker, startDate, endDate string) (map[string]float64, error) {
	dtStart, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	dtEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}

	period1 := dtStart.Unix()
	period2 := dtEnd.AddDate(0, 0, 1).Unix()

	chart, err := p.fetchChart(ticker, period1, period2)
	if err != nil {
		return nil, err
	}

	result := chart.Chart.Result[0]
	if len(result.Timestamp) == 0 {
		return nil, fmt.Errorf("no data points for %s", ticker)
	}

	// Prefer adjclose, fall back to close
	var closeSeries []interface{}
	if len(result.Indicators.AdjClose) > 0 && len(result.Indicators.AdjClose[0].AdjClose) > 0 {
		closeSeries = result.Indicators.AdjClose[0].AdjClose
	} else {
		closeSeries = result.Indicators.Quote[0].Close
	}

	// Collect raw prices
	type datePrice struct {
		date  string
		price float64
	}
	var points []datePrice
	for i, ts := range result.Timestamp {
		if i >= len(closeSeries) || closeSeries[i] == nil {
			continue
		}
		price, ok := toFloat64(closeSeries[i])
		if !ok || price <= 0 {
			continue
		}
		d := time.Unix(ts, 0).UTC().Format("2006-01-02")
		points = append(points, datePrice{date: d, price: price})
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("no valid data points for %s", ticker)
	}

	// Index to 100 from first point
	base := points[0].price
	tri := make(map[string]float64, len(points))
	for _, pt := range points {
		tri[pt.date] = pt.price / base * 100
	}

	return tri, nil
}

// FetchHistoricalPrices fetches daily close prices from Yahoo Finance.
// Returns a map of date → close price.
func (p *Pricer) FetchHistoricalPrices(ticker, startDate, endDate string) (map[string]float64, error) {
	dtStart, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	dtEnd, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, err
	}

	period1 := dtStart.Unix()
	period2 := dtEnd.AddDate(0, 0, 1).Unix()

	chart, err := p.fetchChart(ticker, period1, period2)
	if err != nil {
		return nil, err
	}

	result := chart.Chart.Result[0]
	closes := result.Indicators.Quote[0].Close

	prices := make(map[string]float64)
	for i, ts := range result.Timestamp {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		price, ok := toFloat64(closes[i])
		if !ok {
			continue
		}
		d := time.Unix(ts, 0).UTC().Format("2006-01-02")
		prices[d] = round2(price)
	}

	return prices, nil
}

// PriceCache handles loading/saving prices keyed by report date.

// LoadPriceCache loads cached prices from data/{clientID}/prices_{YYYYMMDD}.json.
func (p *Pricer) LoadPriceCache(reportDate string) map[string]float64 {
	path := p.priceCachePath(reportDate)
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]float64)
	}
	var cache map[string]float64
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]float64)
	}
	log.Printf("Loaded %d cached prices from %s", len(cache), path)
	return cache
}

// SavePriceCache saves prices to data/{clientID}/prices_{YYYYMMDD}.json.
func (p *Pricer) SavePriceCache(reportDate string, cache map[string]float64) error {
	path := p.priceCachePath(reportDate)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (p *Pricer) priceCachePath(reportDate string) string {
	// Convert "2026-02-23" to "20260223"
	date := reportDate[:4] + reportDate[5:7] + reportDate[8:10]
	return filepath.Join(p.clientDir, fmt.Sprintf("prices_%s.json", date))
}

// LoadTRICache loads cached stock TRI data from data/{clientID}/stock_tri.json.
func (p *Pricer) LoadTRICache() map[string]map[string]float64 {
	path := filepath.Join(p.clientDir, "stock_tri.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]map[string]float64)
	}
	var cache map[string]map[string]float64
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]map[string]float64)
	}
	log.Printf("Loaded stock TRI for %d tickers from cache", len(cache))
	return cache
}

// SaveTRICache saves stock TRI data to data/{clientID}/stock_tri.json.
func (p *Pricer) SaveTRICache(cache map[string]map[string]float64) error {
	path := filepath.Join(p.clientDir, "stock_tri.json")
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadHistoricalCache loads cached historical prices from data/{clientID}/historical_prices.json.
func (p *Pricer) LoadHistoricalCache() map[string]map[string]float64 {
	path := filepath.Join(p.clientDir, "historical_prices.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]map[string]float64)
	}
	var cache map[string]map[string]float64
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]map[string]float64)
	}
	log.Printf("Loaded historical prices for %d tickers from cache", len(cache))
	return cache
}

// SaveHistoricalCache saves historical prices to data/{clientID}/historical_prices.json.
func (p *Pricer) SaveHistoricalCache(cache map[string]map[string]float64) error {
	path := filepath.Join(p.clientDir, "historical_prices.json")
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FetchAllPrices fetches current prices for all symbols, using cache for hits.
func (p *Pricer) FetchAllPrices(symbols []string, reportDate string, cfg *CockpitConfig) map[string]float64 {
	cache := p.LoadPriceCache(reportDate)

	for _, sym := range symbols {
		if _, ok := cache[sym]; ok {
			continue
		}
		ticker := cfg.YahooTicker(sym)
		log.Printf("  Fetching price for %s (%s)...", sym, ticker)
		price, err := p.FetchCurrentPrice(ticker, reportDate)
		if err != nil {
			log.Printf("  WARNING: Failed to fetch %s: %v", ticker, err)
			continue
		}
		cache[sym] = round2(price)
		log.Printf("  %s: %.2f", sym, cache[sym])
	}

	if err := p.SavePriceCache(reportDate, cache); err != nil {
		log.Printf("WARNING: Failed to save price cache: %v", err)
	}

	return cache
}

// FetchAllStockTRI fetches stock TRI for all symbols, using cache for hits.
// Skips symbols that are price-only or already cached with enough data points.
func (p *Pricer) FetchAllStockTRI(symbols []string, startDate, endDate string, cfg *CockpitConfig) map[string]map[string]float64 {
	cache := p.LoadTRICache()

	for _, sym := range symbols {
		if cfg.IsPriceOnly(sym) {
			continue
		}
		if existing, ok := cache[sym]; ok && len(existing) >= 100 {
			continue
		}
		ticker := cfg.YahooTicker(sym)
		log.Printf("  Fetching stock TRI for %s (%s)...", sym, ticker)
		tri, err := p.FetchStockTRI(ticker, startDate, endDate)
		if err != nil {
			log.Printf("  WARNING: Failed to fetch TRI for %s: %v", ticker, err)
			continue
		}
		cache[sym] = tri
		log.Printf("  %s: %d data points", sym, len(tri))
	}

	if err := p.SaveTRICache(cache); err != nil {
		log.Printf("WARNING: Failed to save TRI cache: %v", err)
	}

	return cache
}

// FetchAllHistorical fetches historical prices for specified symbols.
func (p *Pricer) FetchAllHistorical(symbols []string, startDate, endDate string, cfg *CockpitConfig) map[string]map[string]float64 {
	cache := p.LoadHistoricalCache()

	for _, sym := range symbols {
		if existing, ok := cache[sym]; ok && len(existing) >= 100 {
			continue
		}
		ticker := cfg.YahooTicker(sym)
		log.Printf("  Fetching historical prices for %s (%s)...", sym, ticker)
		prices, err := p.FetchHistoricalPrices(ticker, startDate, endDate)
		if err != nil {
			log.Printf("  WARNING: Failed to fetch history for %s: %v", ticker, err)
			continue
		}
		cache[sym] = prices
		log.Printf("  %s: %d days", sym, len(prices))
	}

	if err := p.SaveHistoricalCache(cache); err != nil {
		log.Printf("WARNING: Failed to save historical cache: %v", err)
	}

	return cache
}

// toFloat64 converts a JSON number (interface{}) to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// StockTRILookup looks up a stock TRI value with nearest prior date fallback.
func StockTRILookup(triData map[string]float64, dateStr string) (float64, bool) {
	if v, ok := triData[dateStr]; ok {
		return v, true
	}
	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, false
	}
	for i := 1; i < 30; i++ {
		prev := dt.AddDate(0, 0, -i).Format("2006-01-02")
		if v, ok := triData[prev]; ok {
			return v, true
		}
	}
	return 0, false
}

// NearestPrice finds the nearest price on or before (back) or on or after (forward) a date.
func NearestPrice(prices map[string]float64, dateStr, direction string) (float64, bool) {
	if v, ok := prices[dateStr]; ok {
		return v, true
	}
	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, false
	}
	for i := 1; i < 30; i++ {
		var d time.Time
		if direction == "forward" {
			d = dt.AddDate(0, 0, i)
		} else {
			d = dt.AddDate(0, 0, -i)
		}
		if v, ok := prices[d.Format("2006-01-02")]; ok {
			return v, true
		}
	}
	return 0, false
}

// SortedDatesInRange returns sorted dates from a price map within [start, end].
func SortedDatesInRange(prices map[string]float64, start, end string) []string {
	var dates []string
	for d := range prices {
		if d >= start && d <= end {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	return dates
}
