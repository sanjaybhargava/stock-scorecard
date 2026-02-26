# CLAUDE.md — stock-scorecard

## Project Overview

**stock-scorecard** is a Go CLI that parses Zerodha tradebook CSVs, NIFTY 500 TRI data, dividend records, and F&O tradebooks to produce a JSON scorecard of realized trades with alpha calculations. Output is consumed by a React scorecard UI deployed to GitHub Pages.

This is a **standalone module**, separate from `broker-trade-sync`. Different concerns, no shared code.

## Architecture: Import / Score

The CLI has two subcommands plus a legacy all-in-one mode:

```
~/Downloads/ (raw CSVs)  →  import wizard  →  ./data/BT2632/ (clean, per-client)  →  score  →  scorecard.json
                              interactive         source of truth                       batch
                              one-time             permanent                            repeatable
```

**Clean data directory structure:**
```
./data/
  tri.csv                       # NIFTY 500 TRI — shared across all clients
  BT2632/
    trades_BT2632.csv           # deduplicated equity trades
    fno_BT2632.csv              # deduplicated F&O trades
    dividends_BT2632.csv        # confirmed dividends
    reconciliation_BT2632.json  # splits, demergers, manual trades, user decisions
  BT9999/                       # another client — same structure
    trades_BT9999.csv
    ...
```

## Folder Structure

```
stock-scorecard/
├── cmd/
│   └── scorecard/
│       ├── main.go              # CLI entry point + subcommand dispatch + legacy mode
│       ├── import.go            # "import" subcommand — parse raw CSVs, run wizard
│       └── score.go             # "score" subcommand — read clean data, generate scorecard
├── internal/
│   ├── tradebook/
│   │   └── parser.go            # Zerodha EQ CSV parsing + dedup + consolidation + client ID
│   ├── tri/
│   │   └── loader.go            # TRI CSV loading + date lookup with fallback
│   ├── matcher/
│   │   └── fifo.go              # FIFO buy-sell matching (reads reconciliation data)
│   ├── dividend/
│   │   ├── loader.go            # Dividend CSV loading + per-share lookup
│   │   └── fetcher.go           # Yahoo Finance API dividend fetching + Python fallback
│   ├── fno/
│   │   ├── parser.go            # F&O tradebook CSV parsing + dedup + consolidation
│   │   └── attributor.go        # Contract P&L computation + pro-rata attribution
│   ├── scorer/
│   │   └── scorer.go            # Alpha computation, FY grouping, aggregation
│   ├── output/
│   │   └── json.go              # JSON serialization
│   ├── reconciliation/
│   │   ├── types.go             # ReconciliationData struct (splits, demergers, manual trades, F&O renames)
│   │   ├── loader.go            # Load/Save JSON + Default() for BT2632 backward compat
│   │   └── loader_test.go       # Roundtrip test
│   ├── clientid/
│   │   ├── extract.go           # Extract client ID from BT{id}_*.csv filenames
│   │   └── extract_test.go
│   ├── cleandata/
│   │   ├── writer.go            # Write consolidated trades/F&O to CSV
│   │   ├── reader.go            # Read them back
│   │   └── roundtrip_test.go
│   └── wizard/
│       ├── wizard.go            # Interactive reconciliation engine (open positions, unmatched sells)
│       ├── dividends.go         # Dividend fetching + FY-level confirmation
│       └── wizard_test.go
├── scripts/
│   └── pull_dividends.py        # Fetches dividend data from Yahoo Finance (Python fallback)
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

### New: Two-Command Workflow

```bash
# Import: parse raw CSVs → interactive wizard → clean data files
stock-scorecard import \
  --source ~/Downloads \
  --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
  --output ./data/

# Score: read clean data → generate scorecard JSON (batch, repeatable)
stock-scorecard score \
  --data ./data/ \
  --client BT2632 \
  --output ./scorecard.json

# Second user — TRI already in ./data/, no --tri needed
stock-scorecard import --source ~/wife-downloads --output ./data/
stock-scorecard score --data ./data/ --client BT9999 --output ./wife-scorecard.json
```

**Import flags:**
- `--source` (required): Directory containing raw Zerodha tradebook CSVs
- `--tri` (required on first import): Path to NIFTY 500 TRI CSV — copied to `./data/tri.csv`
- `--output` (optional): Output directory for clean data. Default: `./data`
- `--exclude` (optional): Comma-separated symbols to skip. Default: `LIQUIDBEES`
- `--skip-dividends` (optional): Skip dividend fetching

**Score flags:**
- `--data` (optional): Data directory. Default: `./data`
- `--client` (required): Client ID (e.g. `BT2632` or `2632`)
- `--output` (optional): Path for output JSON. Default: `./scorecard.json`
- `--verbose` (optional): Print per-symbol FIFO summary

### Legacy Mode (Backward Compatible)

```bash
go run ./cmd/scorecard \
  --tradebooks ~/Downloads \
  --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
  --dividends ./dividends.csv \
  --fno ~/Downloads \
  --output ./scorecard.json
```

**Legacy flags:**
- `--tradebooks` (required): Directory containing Zerodha equity tradebook CSVs
- `--tri` (required): Path to NIFTY 500 TRI Indexed CSV file
- `--output` (required): Path for output JSON file
- `--dividends` (optional): Path to dividends CSV
- `--fno` (optional): Directory containing F&O tradebook CSVs
- `--reconciliation` (optional): Path to reconciliation JSON (auto-loads if matching file exists)
- `--wizard` (optional): Run interactive reconciliation wizard after FIFO matching
- `--exclude` (optional): Comma-separated symbols to skip. Default: `LIQUIDBEES`
- `--broker` (optional): Broker format. Default: `zerodha`
- `--verbose` (optional): Print per-symbol FIFO summary to stderr

## Processing Pipeline

### Import Pipeline
```
Parse EQ/FnO CSVs → Copy TRI → FIFO Match → Interactive Wizard → Fetch Dividends → Write Clean Data
```

### Score Pipeline
```
Read Clean Data → Load TRI → Load Dividends → FIFO Match → F&O Attribution → Score → JSON
```

### Step 1: Parse & Dedup Equity (internal/tradebook/parser.go)

1. Read all `*.csv` files from source directory; skip files that don't match Zerodha 13-column header
2. Dedup by `trade_id` globally (defensive against duplicate/overlapping files)
3. Skip ETFs (`INF*` ISIN prefix) and excluded symbols
4. Consolidate fills: group by `(ISIN, trade_date, trade_type, order_id)`, compute VWAP
5. Round quantities to integers (equity = whole shares)
6. Extract client ID from `BT{id}_*.csv` filenames

### Step 2: Load TRI Index (internal/tri/loader.go)

- Load NIFTY 500 TRI into `map[string]float64` keyed by `YYYY-MM-DD`
- Weekend/holiday fallback: binary search for most recent prior trading day

### Step 2b: Load Dividends (internal/dividend/loader.go + fetcher.go)

- **Loader:** Parse CSV with columns: `symbol, ex_date, amount` (split-adjusted per-share)
- **Fetcher:** Go-native Yahoo Finance API + Python `pull_dividends.py` fallback
- Lookup: sum all dividends where `buy_date <= ex_date < sell_date`

### Step 3: FIFO Matching (internal/matcher/fifo.go)

1. Apply corporate actions from reconciliation data: stock splits, demergers, manual trades
2. Group by ISIN, sort by (date, trade_type)
3. FIFO queue: on sell, consume oldest buy lots, splitting partial lots
4. Each matched pair → `RealizedTrade` enriched with TRI + dividends
5. Remaining buy lots → `OpenPosition`
6. Unmatched sells → `Warning` (pre-account holdings)

### Step 3a: Reconciliation Wizard (internal/wizard/)

Interactive prompts for:
- **Open positions:** `[H]eld / [S]old / [K]ip` — if sold, collect date + price → manual sell
- **Unmatched sells:** `[P]rovide buy / [S]kip` — collect buy date + price → manual buy
- **Dividends:** Fetch from Yahoo Finance, show FY totals, `[Y/n]` confirmation

### Step 3b: F&O Attribution (internal/fno/)

**Parser** (`parser.go`):
1. Read `*.csv` files from directory (14-column header with `expiry_date`)
2. Extract underlying + option type from symbol via regex: `^([A-Z][A-Z&-]*[A-Z])\d{2}[A-Z]{3}\d+(?:\.\d+)?(CE|PE)$`
3. Apply F&O symbol renames from reconciliation data
4. Dedup by `trade_id`, consolidate fills with VWAP

**Attributor** (`attributor.go`):
1. Group F&O trades by `(underlying, raw_symbol)` → one `ContractPnL` per option contract lifecycle
2. Contract P&L: `net_pnl = Σ(sell_value) - Σ(buy_value)`
3. **Two-pass attribution** to equity realized trades:
   - **Pass 1 (overlap):** For CE and PE contracts, find equity trades where holding period overlaps the contract's active period. Weight = `shares × overlap_days`.
   - **Pass 2 (next-buy, PE only):** For put contracts with no overlap, find the nearest equity buy_date ≥ put's expiry and distribute pro-rata by quantity.
4. Unattributed contracts → `UnattributedFnO` list

### Step 4: Score & Aggregate (internal/scorer/scorer.go)

- **Alpha** = EquityGL - NiftyReturn (per trade). EquityGL includes capital G/L + dividends + F&O income.
- **Win rate** = % of unique (ticker, FY, type) combos with aggregate alpha ≥ 0
- **FY:** Based on sell date. Apr 1 → Mar 31 (Indian fiscal year)
- **Long vs Short:** HoldDays > 365 = Long, else Short (Indian LTCG/STCG threshold)

### Step 5: JSON Output (internal/output/json.go)

Serializes to JSON with: `trades`, `open_positions`, `warnings`, `summary`, `dividend_summary`, `fno_summary`, `unattributed_fno`.

## Reconciliation Data (internal/reconciliation/)

Per-client JSON file storing corporate action data that was previously hardcoded:

```json
{
  "client_id": "BT2632",
  "splits": [
    {"old_isin": "INE935N01012", "new_isin": "INE935N01020", "ratio": 5, "note": "DIXON 1:5"}
  ],
  "demergers": [
    {"parent_isin": "INE002A01018", "child_isin": "INE758E01017", "child_symbol": "JIOFIN", "record_date": "2023-07-20", "parent_cost_pct": 0.9532}
  ],
  "manual_trades": [
    {"symbol": "MPHASIS", "isin": "INE356A01018", "date": "2022-01-27", "trade_type": "buy", "quantity": 700, "price": 3000}
  ],
  "fno_renames": {"MOTHERSUMI": "MOTHERSON", "HDFC": "HDFCBANK"}
}
```

- `Default()` returns BT2632 hardcoded data for backward compatibility
- Auto-loaded from `reconciliation_{clientID}.json` if found next to output
- Updated interactively by the wizard

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

### Dividends CSV

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
  "open_positions": [{...}],
  "warnings": [{...}],
  "summary": {...},
  "dividend_summary": {...},
  "fno_summary": {...},
  "unattributed_fno": [{...}]
}
```

## Edge Cases

- **Partial sells (FIFO splitting):** A buy of 1000 may sell across 3 dates (300, 500, 200) — split buy lots
- **Cross-FY holdings:** Bought FY20-21, sold FY24-25 — single realized trade, FY = sell date
- **Missing TRI dates:** Weekend/holiday → use most recent prior trading day
- **Open positions:** Buy lots without matching sells → `open_positions` array
- **Symbol name changes:** ISIN is stable across renames (MOTHERSUMI→MOTHERSON). Use ISIN for matching
- **Stock splits / demergers:** Handled via reconciliation JSON (previously hardcoded)
- **F&O decimal strikes:** Regex handles symbols like `NTPC23JUN182.5CE`, `POWERGRID23SEP198.75CE`
- **F&O symbol renames:** Stored in reconciliation JSON (e.g. MOTHERSUMI→MOTHERSON, HDFC→HDFCBANK)
- **Cash-secured puts:** No overlap with equity → attributed via "next buy" fallback
- **Index options (NIFTY, BANKNIFTY):** No underlying equity → unattributed, reported separately
- **Multi-client:** Each client gets own data directory, TRI shared across all
- **Backward compatibility:** Legacy flags still work identically; `Default()` provides BT2632 data

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
# New workflow: import + score
stock-scorecard import --source ~/Downloads --tri ~/Downloads/NIFTY500_TRI_Indexed.csv --output ./data/
stock-scorecard score --data ./data/ --client BT2632 --output ./scorecard.json

# Legacy: all-in-one (still works)
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
- No external dependencies beyond stdlib (except `net/http` for dividend fetching)
- All monetary values rounded to int (rupees) for display; floating point internally

## Future Enhancements

- Brokerage/STT deduction
- Unrealized scorecard (live prices)
- Multi-broker parsers (Groww, ICICI)
- Futures (not just options) attribution
