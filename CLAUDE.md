# CLAUDE.md — stock-scorecard

## Project Overview

**stock-scorecard** is a Go CLI that parses Zerodha tradebook CSVs, NIFTY 500 TRI data, dividend records, and F&O tradebooks to produce:

1. **Scorecard** — JSON of realized trades with alpha calculations (consumed by React UI)
2. **Cockpit** — unrealized portfolio analysis: pass/fail vs Nifty+hurdle, deep-dive cards for underperformers

This is a **standalone module**, separate from `broker-trade-sync`. Different concerns, no shared code.

## Architecture

The CLI has four subcommands plus a legacy all-in-one mode:

```
~/Downloads/ (raw CSVs)  →  import  →  ./data/{client}/ (clean data)  →  score    →  scorecard.json
                                                                       →  cockpit  →  cockpit_{client}.json
```

| Subcommand | Purpose | Interactive? |
|------------|---------|-------------|
| `import` | Parse tradebooks, FIFO match, fetch dividends, write clean data | Optional (`--wizard`) |
| `score` | Read clean data, compute alpha, generate scorecard JSON | No |
| `cockpit` | Analyze unrealized portfolio vs Nifty+hurdle | No |
| `correct` | Apply corrections from edited review CSV | No |
| (default) | Runs `import` | — |

**Clean data directory structure:**
```
./data/
  tri.csv                       # NIFTY 500 TRI — embedded in binary, extracted on first run
  {clientID}/
    trades_{clientID}.csv       # deduplicated equity trades
    fno_{clientID}.csv          # deduplicated F&O trades
    dividends_{clientID}.csv    # confirmed dividends
    reconciliation_{clientID}.json  # splits, demergers, manual trades
    cockpit_{clientID}.json     # cockpit config (tax rate, hurdle, classifications)
    prices_{date}.json          # cached Yahoo Finance prices

~/Downloads/                    # user-facing outputs
  realized_{clientID}.csv       # matched buy-sell pairs with alpha
  unrealized_{clientID}.csv     # open positions (input to cockpit)
  review_{clientID}.csv         # items needing manual review
  scorecard_{clientID}.json     # scorecard output
  portfolio_{clientID}.csv      # ticker/qty/price/mkt_value summary
```

## Folder Structure

```
stock-scorecard/
├── cmd/
│   ├── scorecard/
│   │   ├── main.go              # CLI entry point + subcommand dispatch
│   │   ├── import.go            # "import" — parse raw CSVs, FIFO match, dividends, F&O
│   │   ├── score.go             # "score" — read clean data, generate scorecard JSON
│   │   ├── cockpit.go           # "cockpit" — unrealized portfolio analysis
│   │   └── correct.go           # "correct" — apply review CSV corrections
│   └── serve/
│       └── main.go              # Self-contained server (legacy, embeds UI)
├── internal/
│   ├── tradebook/parser.go      # Zerodha EQ CSV parsing + dedup + consolidation
│   ├── tri/loader.go            # TRI CSV loading + date lookup with fallback
│   ├── matcher/
│   │   ├── fifo.go              # FIFO buy-sell matching
│   │   ├── transferin.go        # Transfer-in detection (Tier 2 matching)
│   │   ├── transferin_test.go
│   │   ├── renames.go           # Symbol rename mappings
│   │   └── nifty500_prices_20160224.json  # Static buy prices for transfer-ins
│   ├── dividend/
│   │   ├── loader.go            # Dividend CSV loading + per-share lookup
│   │   └── fetcher.go           # Yahoo Finance API + Python fallback
│   ├── fno/
│   │   ├── parser.go            # F&O tradebook CSV parsing + dedup
│   │   └── attributor.go        # Contract P&L + pro-rata attribution
│   ├── scorer/scorer.go         # Alpha computation, FY grouping, aggregation
│   ├── output/json.go           # JSON serialization
│   ├── reconciliation/
│   │   ├── types.go             # ReconciliationData struct
│   │   ├── loader.go            # Load/Save JSON + Default() for BT2632
│   │   └── loader_test.go
│   ├── clientid/
│   │   ├── extract.go           # Extract client ID from filenames
│   │   └── extract_test.go
│   ├── cleandata/
│   │   ├── writer.go            # Write trades/F&O/realized/unrealized/review CSVs
│   │   ├── reader.go            # Read them back
│   │   └── roundtrip_test.go
│   ├── wizard/
│   │   ├── wizard.go            # Interactive reconciliation (open positions, unmatched sells)
│   │   ├── dividends.go         # Dividend fetching + FY-level confirmation
│   │   └── wizard_test.go
│   └── cockpit/
│       ├── config.go            # CockpitConfig: tax rate, hurdle, classifications, ticker_map
│       ├── hurdle.go            # EnrichLots, ClassifyStocks (pass/fail/too-early/no-test)
│       ├── pricer.go            # Yahoo Finance price + stock TRI fetching with caching
│       ├── fno.go               # F&O attribution for unrealized lots
│       ├── deepdive.go          # Deep-dive cards for failed stocks
│       └── output.go            # BuildCockpitJSON + WriteCockpitJSON
├── scripts/
│   ├── pull_dividends.py        # Yahoo Finance dividend fetcher (Python fallback)
│   ├── build_cockpit_data.py    # Python cockpit data builder (prices, TRI, deep-dive)
│   ├── fetch_stock_tri.py       # Fetch stock-level TRI (Adj Close indexed to 100)
│   └── fetch_nifty500_prices.py # Fetch static prices for transfer-in detection
├── ui/                          # React scorecard UI (Vite + Tailwind)
│   ├── src/StockScorecard.jsx   # 3-level drill-down scorecard
│   └── public/scorecard.json
├── ui-cockpit/                  # React cockpit UI (Vite + Tailwind)
│   ├── src/App.jsx              # Cockpit dashboard (pass/fail/deep-dive)
│   ├── src/components/
│   │   ├── StockDeepDive.jsx    # Deep-dive container for failed stocks
│   │   ├── TestResults.jsx      # Pass/fail test result cards
│   │   ├── PortfolioSummary.jsx # Portfolio overview
│   │   └── cards/
│   │       ├── ConvictionPicks.jsx   # High-conviction picks (11 stocks, slider, GTT matrix)
│   │       ├── MomentumPicks.jsx    # Momentum picks (3 picks, monthly review, slider)
│   │       ├── RedeploymentPlan.jsx  # 3-bucket redeployment (Nifty/Conviction/Momentum)
│   │       ├── TerminalValue.jsx     # Terminal value comparison with 3-bucket allocation
│   │       ├── PhasePerformance.jsx  # Bull/Bear/Sideways phase returns
│   │       ├── MaximumPain.jsx       # Maximum drawdown analysis
│   │       ├── CoveredCall.jsx       # Covered call income potential
│   │       └── FutureProspects.jsx   # AI-generated future outlook
│   └── public/cockpit.json
├── docs/
│   └── conviction-picks-reference.pdf  # Source data for 11 conviction picks
├── build-release.sh             # Cross-compile for Mac ARM/Intel + Windows
├── deploy.sh                    # Build + deploy scorecard to gh-pages
├── deploy-cockpit.sh            # Build + deploy cockpit to gh-pages
├── CLAUDE.md
├── go.mod
└── go.sum
```

## CLI Interface

### Import (Non-Interactive Default)

```bash
# Parse tradebooks, FIFO match, fetch dividends, F&O attribution, score
stock-scorecard import --client ZY7393

# With all flags
stock-scorecard import \
  --source ~/Downloads \
  --output ./data/ \
  --client ZY7393 \
  --exclude LIQUIDBEES

# Interactive wizard (step-by-step confirmation)
stock-scorecard import --client ZY7393 --wizard
```

**Flags:**
- `--source` (optional): Directory containing raw CSVs. Default: `~/Downloads`
- `--output` (optional): Output directory. Default: `./data`
- `--client` (optional): Filter files by client ID prefix
- `--exclude` (optional): Comma-separated symbols to skip. Default: `LIQUIDBEES`
- `--wizard` (optional): Run interactive mode

### Score

```bash
stock-scorecard score --client BT2632 --data ./data/ --output ./scorecard.json
```

### Cockpit

```bash
stock-scorecard cockpit --client ZY7393
```

**Flags:**
- `--client` (required): Client ID
- `--data` (optional): Data directory. Default: `./data`
- `--source` (optional): Directory with unrealized CSV. Default: `~/Downloads`
- `--output` (optional): Output JSON path. Default: `./cockpit_{clientID}.json`
- `--report-date` (optional): Price date. Default: today or from config
- `--skip-deep-dive` (optional): Skip deep-dive analysis

### Correct

```bash
stock-scorecard correct --input ~/Downloads/review_ZY7393.csv
```

### Cross-Platform Build

```bash
./build-release.sh    # → ./dist/stock-scorecard-mac-m, -mac-intel, -windows.exe
```

## Processing Pipeline

### Import Pipeline (3-Tier Matching)
```
Parse EQ/FnO CSVs → FIFO Match (Tier 1) → Transfer-In Detection (Tier 2) → Dividends → F&O Attribution → Score → JSON
```

- **Tier 1 (Exact):** FIFO buy-sell matching from tradebooks
- **Tier 2 (Intelligent):** Unmatched sells auto-matched with static 2016-02-24 buy prices
- **Tier 3 (Skipped):** Unresolved sells (bonds/NCDs, missing prices) → review CSV

### Cockpit Pipeline
```
Load Unrealized CSV → Fetch Prices (Yahoo) → Load Nifty TRI → Enrich Lots → F&O Attribution → Classify (Pass/Fail) → Deep-Dive → JSON
```

### Cockpit Classification

Each stock is tested against **Nifty CAGR + hurdle** over its holding period:

| Category | Condition |
|----------|-----------|
| **Pass** | Current value ≥ Nifty+hurdle target |
| **Fail** | Current value < Nifty+hurdle target (deficit shown) |
| **Too Early** | All lots held < 1 year |
| **No Test** | Asset class = `index_mf` (index funds, bonds, liquid funds) |

Default hurdle: 3% for stocks, configurable per asset class.

## Cockpit Config (`data/{clientID}/cockpit_{clientID}.json`)

```json
{
  "client_id": "ZY7393",
  "client_name": "Vimal Kapur",
  "report_date": "2026-02-23",
  "expected_total_income": 15000000,
  "default_hurdle_pct": 3,
  "ticker_map": {
    "MINDTREE": "LTIM.NS",
    "NAUKRI": "NAUKRI.NS"
  },
  "classifications": {
    "MONIFTY500": {"asset_class": "index_mf", "hurdle_pct": 0},
    "MOMENTUM50": {"asset_class": "active_mf", "hurdle_pct": 2},
    "GOLDBEES": {"asset_class": "gold_etf", "hurdle_pct": 3},
    "LIQUIDBEES": {"asset_class": "index_mf", "hurdle_pct": 0},
    "851HUDCO28-NB": {"asset_class": "index_mf", "hurdle_pct": 0},
    "PPFCF": {"asset_class": "active_mf", "hurdle_pct": 2}
  },
  "price_only_symbols": ["GOLDBEES", "MONIFTY500", "MOMENTUM50", "851HUDCO28-NB", "LIQUIDBEES"],
  "display_names": {"RELIANCE": "Reliance Industries", ...},
  "manual_lots": [],
  "market_phases": [
    {"regime": "Bull", "start": "2016-02-24", "end": "2018-01-31"},
    {"regime": "Bear", "start": "2018-02-01", "end": "2020-03-23"}
  ]
}
```

**Key config fields:**
- `ticker_map`: Override Yahoo Finance symbol (e.g. MINDTREE→LTIM.NS)
- `classifications`: Asset class + hurdle per symbol. `index_mf` = no test
- `price_only_symbols`: Skip stock TRI fetch, use price-only CAGR
- `manual_lots`: Extra positions not in unrealized CSV (added to portfolio)
- `market_phases`: Bull/Bear/Sideways regimes for deep-dive charts

**Special cases:**
- Bonds (e.g. `851HUDCO28-NB`): No Yahoo price — manually add to `data/{clientID}/prices_{date}.json`
- MF NAVs: No Yahoo ticker — manually add to price cache (fetch from MFAPI: `api.mfapi.in/mf/{schemeCode}/latest`)
- BSE suffixes: Strip `-BE` from symbols in unrealized CSV (e.g. `KWIL-BE` → `KWIL`)

## Unrealized CSV Format (Input to Cockpit)

```csv
symbol,isin,buy_date,quantity,buy_price,invested
RELIANCE,INE002A01018,2020-10-27,2940,1004.94,2954530.0
```

Generated by `import` command. Can be manually edited to add transfer-in positions, MF holdings, bonds.

## Edge Cases

- **Partial sells (FIFO splitting):** A buy of 1000 may sell across 3 dates — split buy lots
- **Cross-FY holdings:** FY = sell date (Indian fiscal year Apr 1 → Mar 31)
- **Missing TRI dates:** Weekend/holiday → binary search for most recent prior trading day
- **Missing Nifty TRI for buy date:** Lot still enriched with zero Nifty metrics (bonds, pre-2016 buys)
- **Symbol name changes:** ISIN is stable across renames. Use ISIN for matching
- **Stock splits / demergers:** Via reconciliation JSON
- **F&O decimal strikes:** Regex handles `NTPC23JUN182.5CE`
- **Cash-secured puts:** No overlap → "next buy" fallback attribution
- **Index options:** No underlying equity → unattributed, included in overall alpha
- **Transfer-ins:** Unmatched sells auto-matched with static historical prices
- **Bonds/NCDs:** No Yahoo Finance data — manual price injection in cache
- **MF NAVs:** Not on Yahoo — manual price injection (MFAPI source)
- **Multi-client:** Each client gets own data dir, TRI shared

## Deploy

```bash
# Scorecard (realized trades)
./deploy.sh

# Cockpit (unrealized portfolio) — generates + builds UI + pushes
./deploy-cockpit.sh ZY7393
./deploy-cockpit.sh --all

# Cross-platform binaries for beta distribution
./build-release.sh    # → ./dist/
```

**Live sites:**
- Scorecard: https://sanjaybhargava.github.io/stock-scorecard/
- MIL cockpit: https://sanjaybhargava.github.io/vimal-stock-scorecard/cockpit/?client=ZY7393

## Commands

```bash
# Import + score (default non-interactive)
stock-scorecard import --client ZY7393

# Cockpit
stock-scorecard cockpit --client ZY7393

# Apply corrections from review CSV
stock-scorecard correct --input ~/Downloads/review_ZY7393.csv

# Build binary
go build -o stock-scorecard ./cmd/scorecard

# Cross-compile for distribution
./build-release.sh

# Test
go test ./...

# Deploy
./deploy.sh                        # scorecard
./deploy-cockpit.sh ZY7393         # cockpit
```

## Coding Conventions

- `gofmt` for formatting
- Idiomatic Go error handling: check and wrap with context
- `log` package for pipeline progress (not `fmt.Println`)
- Package names: lowercase, single word
- No external dependencies beyond stdlib (except `net/http` for Yahoo Finance)
- All monetary values rounded to int (rupees) for display; floating point internally
- TRI embedded in binary via `//go:embed tri_embedded.csv`

## Clients

| Client | ID | Notes |
|--------|------|-------|
| Sanjay | BT2632 / CI8364 | Equity + F&O, wife CI8364 |
| MIL (Vimal Kapur) | ZY7393 | 452 realized, 39 unrealized tickers, transfer-ins from 2016 |

## Cockpit Navigation

Layer 1 (PortfolioSummary) has three navigation buttons at the bottom of the hero card:

| Button | Layer | Status |
|--------|-------|--------|
| **3% Test** | 2 → TestResults | Active (indigo) |
| **Conviction Picks** | 4 → ConvictionPicks | Active (amber) |
| **Momentum Picks** | 5 → MomentumPicks | Active (teal) |

## Cockpit Deep-Dive Cards

Each failed stock gets a deep-dive with these cards:

| Card | Purpose |
|------|---------|
| PhasePerformance | Stock vs Nifty returns across Bull/Bear/Sideways regimes |
| MaximumPain | Worst drawdown analysis and recovery timeline |
| RedeploymentPlan | Three-bucket redeployment model with interactive sliders |
| TerminalValue | Terminal value: hold vs sell-and-redeploy comparison |
| CoveredCall | Covered call income potential on retained shares |
| FutureProspects | AI-generated business outlook and catalysts |

## Conviction Picks Card

11 high-conviction stock picks (hardcoded in `ConvictionPicks.jsx`, source: `docs/conviction-picks-reference.pdf`):

| # | Stock | CEO | Status | GTT Price | Strategy | Role |
|---|-------|-----|--------|-----------|----------|------|
| 1 | KFin Tech | Sreekanth Nadella | Active | — | Core | Financial Infrastructure / SaaS |
| 2 | IDFC First | V. Vaidyanathan | Active | — | Opportunistic | Banking Value (Recovery Play) |
| 3 | Amara Raja | Jayadev Galla | Active | — | Core | Energy Transition / Gigafactory |
| 4 | Syrma SGS | J.S. Gujral | Active | — | Core | Defense & Industrial Electronics |
| 5 | Solar Inds. | Manish Nuwal | GTT | ₹11,800 | Structural | Ammunition & Rocket Propellants |
| 6 | Zen Tech | Ashok Atluri | GTT | ₹1,180 | Structural | Drone Warfare & Anti-Drone Tech |
| 7 | PTC Inds. | Sachin Agarwal | GTT | ₹17,200 | Moat | Strategic Titanium Castings |
| 8 | Netweb Tech | Sanjay Lodha | GTT | ₹3,000 | Tech | AI Infrastructure & Servers |
| 9 | Gravita | Yogesh Malhotra | GTT | ₹1,500 | Resilience | Circular Economy (Metal Recycling) |
| 10 | Apar Inds. | Kushal Desai | GTT | ₹9,200 | Infra | Global Power Transmission |
| 11 | KEI Inds. | Anil Gupta | GTT | ₹4,180 | Infra | Urban Underground Cables |

- **MAX_REDEPLOYABLE:** ₹7,43,18,246 (₹7.43 Cr)
- **Conviction slice:** 12% of slider value (from three-way allocation model)
- **Per stock:** conviction slice / 11
- **Active picks:** deploy at market price, no share count
- **GTT picks:** deploy at GTT limit price, shares = floor(amount / gttPrice)

### Three-Way Allocation Model

Redeployment and terminal value cards use a two-step slider UX for three-bucket allocation:

1. **Slider 1 — Passive vs Active:** 50–100% passive (default 80%), step 10
2. **Slider 2 — Conviction vs Momentum:** 0–100% conviction within active (default 60%), step 10. Hidden when passive = 100%.

At defaults (80/20 passive/active, 60/40 conviction/momentum):
- **Nifty 500** (indigo `#6366f1`): 80% — broad market index, 16.2% CAGR
- **High-Conviction** (amber `#d97706`): 12% — 3–5 year holds, 25% CAGR target
- **Momentum** (teal `#0d9488`): 8% — AI-assisted themes, 6–18 month holds, 20% CAGR target

## Momentum Picks Card

3 momentum picks reviewed monthly (hardcoded in `MomentumPicks.jsx`):

| # | Ticker | Name | Type | Entry | Exit GTT | Rationale |
|---|--------|------|------|-------|----------|-----------|
| 1 | SILVERBEES | Nippon India Silver BeES | ETF | ₹96 | ₹86 | Silver outperforming gold; industrial + monetary demand tailwind |
| 2 | METALIETF | ICICI Pru Nifty Metal ETF | ETF | ₹12,250 | ₹11,025 | Metals leading risk-on rotation; China stimulus + infra capex |
| 3 | MAZDOCK | Mazagon Dock Shipbuilders | Stock | ₹2,235 | ₹2,012 | Defence secular theme; order book visibility 3+ years |

- **Slider:** 0–₹1 Cr, step ₹1L, default ₹1 Cr, teal `#0d9488` accent
- **Per pick:** slider value / 3
- **Qty:** floor(perPick / entry price)
- **Exit discipline:** 10% trailing stop from entry, placed as GTT sell order on day one
- **Methodology:** Rules-based momentum screen — 6-month return, exclude blow-off top 5%, above 200-DMA, rank by risk-adjusted trend score
- **Disclaimer:** Always visible — "Picked by AI, do your own research, high risk"
- **Review cadence:** Monthly (March 2026 is first review)

## Future Enhancements

- ~~High-conviction picks suggestion card~~ ✓ Implemented (ConvictionPicks.jsx)
- ~~Momentum picks suggestion card~~ ✓ Implemented (MomentumPicks.jsx)
- Brokerage/STT deduction
- Auto-fetch MF NAVs from MFAPI
- Multi-broker parsers (Groww, ICICI)
- Futures (not just options) attribution
- Auto-strip BSE suffixes (-BE, -BL) from symbols
