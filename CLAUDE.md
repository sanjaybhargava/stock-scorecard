# CLAUDE.md — stock-scorecard

## Project Overview

**stock-scorecard** is a Go CLI that parses Zerodha tradebook CSVs, NIFTY 500 TRI data, dividend records, and F&O tradebooks to produce a JSON scorecard of realized trades with alpha calculations. Output is consumed by a React scorecard UI deployed to GitHub Pages.

This is a **standalone module**, separate from `broker-trade-sync`. Different concerns, no shared code.

## Folder Structure

```
stock-scorecard/
├── cmd/
│   └── scorecard/
│       └── main.go              # CLI entry point
├── internal/
│   ├── tradebook/
│   │   └── parser.go            # Zerodha EQ CSV parsing + dedup + consolidation
│   ├── tri/
│   │   └── loader.go            # TRI CSV loading + date lookup with fallback
│   ├── matcher/
│   │   └── fifo.go              # FIFO buy-sell matching + corporate actions
│   ├── dividend/
│   │   └── loader.go            # Dividend CSV loading + per-share lookup
│   ├── fno/
│   │   ├── parser.go            # F&O tradebook CSV parsing + dedup + consolidation
│   │   └── attributor.go        # Contract P&L computation + pro-rata attribution
│   ├── scorer/
│   │   └── scorer.go            # Alpha computation, FY grouping, aggregation
│   └── output/
│       └── json.go              # JSON serialization
├── scripts/
│   └── pull_dividends.py        # Fetches dividend data from Google Finance
├── ui/                          # React scorecard UI (Vite + Tailwind)
│   ├── src/
│   │   └── StockScorecard.jsx   # Single-file React component (3-level drill-down)
│   └── public/
│       └── scorecard.json       # Generated scorecard (copied into build)
├── deploy.sh                    # Build + deploy to GitHub Pages
├── CLAUDE.md
├── go.mod
└── go.sum
```

## CLI Interface

```bash
go run ./cmd/scorecard \
  --tradebooks ~/Downloads \
  --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
  --dividends ./dividends.csv \
  --fno ~/Downloads \
  --output ./scorecard.json
```

**Flags:**
- `--tradebooks` (required): Directory containing Zerodha equity tradebook CSVs (`BT*.csv`)
- `--tri` (required): Path to NIFTY 500 TRI Indexed CSV file
- `--output` (required): Path for output JSON file
- `--dividends` (optional): Path to dividends CSV (from `scripts/pull_dividends.py`)
- `--fno` (optional): Directory containing F&O tradebook CSVs (`BT*_FO_*.csv`)
- `--exclude` (optional): Comma-separated symbols to skip. Default: `LIQUIDBEES,GOLDBEES`
- `--broker` (optional): Broker format. Default: `zerodha`
- `--verbose` (optional): Print per-symbol FIFO summary to stderr

## Processing Pipeline

```
Parse EQ CSVs → Load TRI → Load Dividends → FIFO Match → Parse F&O → Attribute F&O → Score → JSON
```

### Step 1: Parse & Dedup Equity (internal/tradebook/parser.go)

1. Read all `BT*.csv` files from `--tradebooks` directory; skip files that don't match Zerodha 13-column header
2. Dedup by `trade_id` globally (defensive against duplicate/overlapping files)
3. Skip ETFs (`INF*` ISIN prefix) and excluded symbols
4. Consolidate fills: group by `(ISIN, trade_date, trade_type, order_id)`, compute VWAP
5. Round quantities to integers (equity = whole shares)

### Step 2: Load TRI Index (internal/tri/loader.go)

- Load NIFTY 500 TRI into `map[string]float64` keyed by `YYYY-MM-DD`
- Weekend/holiday fallback: binary search for most recent prior trading day

### Step 2b: Load Dividends (internal/dividend/loader.go)

- Parse CSV with columns: `symbol, ex_date, amount` (split-adjusted per-share)
- Lookup: sum all dividends where `buy_date <= ex_date < sell_date`

### Step 3: FIFO Matching (internal/matcher/fifo.go)

1. Apply corporate actions: stock splits (`knownSplits`), demergers (`knownDemergers`), manual trades (`knownManualTrades`)
2. Group by ISIN, sort by (date, trade_type)
3. FIFO queue: on sell, consume oldest buy lots, splitting partial lots
4. Each matched pair → `RealizedTrade` enriched with TRI + dividends
5. Remaining buy lots → `OpenPosition`
6. Unmatched sells → `Warning` (pre-account holdings)

### Step 3b: F&O Attribution (internal/fno/)

**Parser** (`parser.go`):
1. Read `BT*_FO_*.csv` files from `--fno` directory (14-column header with `expiry_date`)
2. Extract underlying + option type from symbol via regex: `^([A-Z][A-Z&-]*[A-Z])\d{2}[A-Z]{3}\d+(?:\.\d+)?(CE|PE)$`
3. Apply symbol renames: `MOTHERSUMI→MOTHERSON`, `HDFC→HDFCBANK` (F&O has no ISIN)
4. Dedup by `trade_id`, consolidate fills with VWAP

**Attributor** (`attributor.go`):
1. Group F&O trades by `(underlying, raw_symbol)` → one `ContractPnL` per option contract lifecycle
2. Contract P&L: `net_pnl = Σ(sell_value) - Σ(buy_value)`
3. **Two-pass attribution** to equity realized trades:
   - **Pass 1 (overlap):** For CE and PE contracts, find equity trades where holding period overlaps the contract's active period (`first_trade_date → expiry_date`). Weight = `shares × overlap_days`. Distributes income pro-rata. Used for covered calls and protective puts.
   - **Pass 2 (next-buy, PE only):** For put contracts with no overlap, find the nearest equity buy_date ≥ put's expiry and distribute pro-rata by quantity. This handles cash-secured puts that led to stock purchases.
4. Unattributed contracts (no equity position at all) → `UnattributedFnO` list

**Why two passes:** The user's strategy is covered calls (CE) + cash-secured puts (PE). Covered calls overlap with equity holding periods, but cash-secured puts expire *before* the resulting stock purchase, so overlap is always zero. The next-buy fallback captures this pattern.

### Step 4: Score & Aggregate (internal/scorer/scorer.go)

- **Alpha** = EquityGL - NiftyReturn (per trade). EquityGL includes capital G/L + dividends + F&O income.
- **Win rate** = % of unique (ticker, FY, type) combos with aggregate alpha ≥ 0
- **FY:** Based on sell date. Apr 1 → Mar 31 (Indian fiscal year)
- **Long vs Short:** HoldDays > 365 = Long, else Short (Indian LTCG/STCG threshold)

### Step 5: JSON Output (internal/output/json.go)

Serializes to JSON with: `trades`, `open_positions`, `warnings`, `summary`, `dividend_summary`, `fno_summary`, `unattributed_fno`.

## Input Files

### Equity Tradebook CSVs (Zerodha)

Files: `BT{client_id}_{startYYYYMMDD}_{endYYYYMMDD}.csv` (13 columns)

```
symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time
```

### F&O Tradebook CSVs (Zerodha)

Files: `BT{client_id}_FO_{startYYYYMMDD}_{endYYYYMMDD}.csv` (14 columns — same as equity + `expiry_date`)

```
symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time,expiry_date
```

### NIFTY 500 TRI Indexed CSV

```
Date,TRI_Indexed
```

### Dividends CSV (from pull_dividends.py)

```
symbol,ex_date,amount
```

## Output JSON Structure

```json
{
  "generated_at": "2026-02-25T12:00:00Z",
  "trades": [{
    "fy": "FY 2024-25", "type": "Long", "ticker": "BHARTIARTL",
    "buy_date": "2023-08-30", "sell_date": "2025-01-06", "hold_days": 494,
    "quantity": 950, "buy_price": 859.70, "sell_price": 1600.00,
    "invested": 816715, "sale_value": 1520000,
    "equity_gl": 703285,
    "dividend_income": 12000,
    "option_income": 234567,
    "nifty_buy_tri": 280.50, "nifty_sell_tri": 410.20,
    "nifty_return": 377500, "alpha": 325785
  }],
  "open_positions": [{
    "ticker": "SYMBOL", "buy_date": "2024-06-15",
    "quantity": 500, "buy_price": 1200.00, "invested": 600000,
    "note": "No matching sell — still held"
  }],
  "warnings": [{
    "ticker": "SYMBOL", "sell_date": "2020-03-23",
    "unmatched_shares": 500, "total_shares": 500,
    "message": "..."
  }],
  "summary": {
    "total_trades": 657, "total_invested": 125000000,
    "total_my_return": 2400000, "total_nifty_return": 1800000,
    "net_alpha": 600000, "win_rate": 46,
    "by_fy": [{ "fy": "FY 2024-25", "type": "Long", "num_trades": 12, "invested": 4500000, "my_return": 800000, "nifty_return": 600000, "alpha": 200000 }]
  },
  "dividend_summary": {
    "total_dividend_income": 150000,
    "by_fy": [{ "fy": "FY 2024-25", "dividend_income": 50000 }]
  },
  "fno_summary": {
    "total_option_income": 7300000, "unattributed": 880000,
    "by_fy": [{ "fy": "FY 2024-25", "option_income": 2000000 }]
  },
  "unattributed_fno": [{ "underlying": "NIFTY", "net_pnl": -50000, "note": "No equity position to attribute to" }]
}
```

## Edge Cases

- **Partial sells (FIFO splitting):** A buy of 1000 may sell across 3 dates (300, 500, 200) — split buy lots
- **Cross-FY holdings:** Bought FY20-21, sold FY24-25 — single realized trade, FY = sell date
- **Missing TRI dates:** Weekend/holiday → use most recent prior trading day
- **Open positions:** Buy lots without matching sells → `open_positions` array
- **Symbol name changes:** ISIN is stable across renames (MOTHERSUMI→MOTHERSON). Use ISIN for matching
- **Stock splits / demergers:** Handled via `knownSplits` and `knownDemergers` in matcher
- **F&O decimal strikes:** Regex handles symbols like `NTPC23JUN182.5CE`, `POWERGRID23SEP198.75CE`
- **F&O symbol renames:** MOTHERSUMI→MOTHERSON, HDFC→HDFCBANK (explicit map, no ISIN in F&O)
- **Cash-secured puts:** No overlap with equity → attributed via "next buy" fallback
- **Index options (NIFTY, BANKNIFTY):** No underlying equity → unattributed, reported separately
- **Backward compatibility:** Running without `--fno` or `--dividends` produces identical output to v1

## Deploy

```bash
# Full deploy (generates scorecard + builds UI + deploys to gh-pages)
./deploy.sh

# Skip scorecard generation (reuse existing scorecard.json)
./deploy.sh --skip-scorecard
```

The deploy script:
1. Builds Go CLI, generates `dividends.csv` if missing (via `pull_dividends.py`), then runs scorecard with all flags
2. Builds React UI via `npm run build` in `ui/`
3. Verifies `index.html` + `scorecard.json` in build output
4. Copies build to `gh-pages` branch and force-pushes

**Live site:** https://sanjaybhargava.github.io/stock-scorecard/

## Commands

```bash
# Full run with all data sources
go run ./cmd/scorecard \
  --tradebooks ~/Downloads \
  --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
  --dividends ./dividends.csv \
  --fno ~/Downloads \
  --output ./scorecard.json

# Build binary
go build -o stock-scorecard ./cmd/scorecard

# Test
go test ./...

# Deploy to GitHub Pages
./deploy.sh
```

## Coding Conventions

- `gofmt` for formatting
- Idiomatic Go error handling: check and wrap with context
- `log` package for pipeline progress (not `fmt.Println`)
- Package names: lowercase, single word
- No external dependencies beyond stdlib
- All monetary values rounded to int (rupees) for display; floating point internally

## Future Enhancements

- Brokerage/STT deduction
- Unrealized scorecard (live prices)
- Multi-broker parsers (Groww, ICICI)
- Futures (not just options) attribution
