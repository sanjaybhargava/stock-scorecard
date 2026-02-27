# Stock Scorecard & Portfolio Cockpit

A Go CLI that scores your Zerodha equity trades against NIFTY 500 TRI. Parses tradebook CSVs, performs FIFO matching, and calculates per-trade alpha to answer: **did you beat the market?**

The **Cockpit** analyzes your unrealized portfolio: which stocks are beating Nifty+3%, which aren't, and what to do about the underperformers.

## What You Get

**Scorecard** (realized trades):
- Per-trade alpha: your return minus what Nifty would have earned on the same capital
- Win rate, FY breakdown, long vs short analysis
- Dividend income + F&O attribution per trade
- 3-level drill-down UI: overall → FY/ticker → individual lots

**Cockpit** (unrealized portfolio):
- Pass/fail test: is each stock beating Nifty + hurdle rate?
- Surplus/deficit per stock in rupees
- Deep-dive cards for underperformers: phase performance, terminal value, covered call analysis
- Support for stocks, mutual funds, ETFs, bonds

**Live sites:**
- [Scorecard](https://sanjaybhargava.github.io/stock-scorecard/)
- [Cockpit](https://sanjaybhargava.github.io/vimal-stock-scorecard/cockpit/?client=ZY7393)

## Quick Start (Beta Users)

Download the binary for your platform, place it in `~/Downloads` alongside your Zerodha tradebook CSVs.

| Platform | Binary |
|----------|--------|
| Mac M1/M2/M3/M4 | `stock-scorecard-mac-m` |
| Mac Intel | `stock-scorecard-mac-intel` |
| Windows | `stock-scorecard-windows.exe` |

### Step 1: Download tradebooks from Zerodha

Go to [Zerodha Console](https://console.zerodha.com) → Reports → Tradebook. Download equity (and F&O if applicable) CSVs for all years. Save to `~/Downloads`.

### Step 2: Import & score

**Mac:**
```bash
cd ~/Downloads
chmod +x stock-scorecard-mac-m
./stock-scorecard-mac-m import --client YOUR_CLIENT_ID
```

> If macOS blocks it: System Settings → Privacy & Security → scroll down → "Allow Anyway"

**Windows:**
```cmd
cd %USERPROFILE%\Downloads
stock-scorecard-windows.exe import --client YOUR_CLIENT_ID
```

Your client ID is the prefix of your tradebook filename (e.g. `BT2632` from `BT2632_20200101_20201231.csv`).

### Step 3: Run the cockpit

```bash
./stock-scorecard-mac-m cockpit --client YOUR_CLIENT_ID
```

### What gets created

| File | Description |
|------|-------------|
| `scorecard_{id}.json` | Full scorecard with per-trade alpha |
| `realized_{id}.csv` | All matched buy-sell pairs (open in Excel) |
| `unrealized_{id}.csv` | Open positions |
| `cockpit_{id}.json` | Portfolio cockpit analysis |
| `review_{id}.csv` | Items needing manual review |

## How It Works

### Scorecard Pipeline

```
Zerodha CSVs → Parse & Dedup → FIFO Match → Transfer-In Detection → Dividends → F&O Attribution → Score → JSON
```

1. **Parse** — Read all `*.csv` files, dedup by `trade_id`, consolidate fills into VWAP trades
2. **FIFO Match** — For each stock (by ISIN), match sells to oldest buy lots
3. **Transfer-In Detection** — Unmatched sells (bought before Zerodha) auto-matched with historical prices
4. **Dividends** — Fetched from Yahoo Finance, attributed to holding periods
5. **F&O Attribution** — Option income distributed to underlying equity trades (covered calls, cash-secured puts)
6. **Score** — Alpha = your return - Nifty return on same capital over same period

### Cockpit Pipeline

```
Unrealized CSV → Fetch Prices → Enrich with Nifty TRI → Classify (Pass/Fail) → Deep-Dive → JSON
```

Each stock is tested: **did it beat Nifty + hurdle rate over the holding period?**

| Result | Meaning |
|--------|---------|
| **Pass** | Current value ≥ what Nifty+hurdle would have earned |
| **Fail** | Underperforming — deficit shown in rupees |
| **Too Early** | Held < 1 year, not yet testable |
| **No Test** | Index funds, bonds, liquid funds — exempt from hurdle |

### 3-Tier Matching

| Tier | Method | Coverage |
|------|--------|----------|
| **Tier 1** | Exact FIFO from tradebooks | ~75% of sells |
| **Tier 2** | Transfer-in: auto-match with historical buy prices | ~20% |
| **Tier 3** | Unresolved (bonds, NCDs) — written to review CSV | ~5% |

## Developer Setup

### Prerequisites

- Go 1.21+
- Node.js 18+ (for the React UIs)
- Python 3 (optional, for dividend fetching fallback)

### Build & Run

```bash
# Build
go build -o stock-scorecard ./cmd/scorecard

# Import tradebooks (non-interactive, reads from ~/Downloads)
./stock-scorecard import --client BT2632

# Generate cockpit
./stock-scorecard cockpit --client ZY7393

# Run tests
go test ./...

# Cross-compile for distribution
./build-release.sh    # → ./dist/stock-scorecard-mac-m, -mac-intel, -windows.exe
```

### CLI Subcommands

```
stock-scorecard import   [flags]    Parse tradebooks, FIFO match, score
stock-scorecard score    [flags]    Re-score from clean data (batch, repeatable)
stock-scorecard cockpit  [flags]    Analyze unrealized portfolio
stock-scorecard correct  [flags]    Apply corrections from review CSV
```

**Import flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--source` | `~/Downloads` | Directory containing Zerodha CSVs |
| `--output` | `./data` | Output directory for clean data |
| `--client` | (auto-detect) | Filter files by client ID |
| `--exclude` | `LIQUIDBEES` | Symbols to skip |
| `--wizard` | false | Interactive mode |

**Cockpit flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--client` | (required) | Client ID |
| `--data` | `./data` | Data directory |
| `--source` | `~/Downloads` | Directory with unrealized CSV |
| `--report-date` | today | Price date (YYYY-MM-DD) |

### Deploy

```bash
# Scorecard UI to GitHub Pages
./deploy.sh

# Cockpit UI to GitHub Pages
./deploy-cockpit.sh ZY7393
./deploy-cockpit.sh --all
```

### Project Structure

```
cmd/scorecard/           CLI entry point + subcommands
internal/
  tradebook/             Zerodha CSV parsing + dedup
  tri/                   NIFTY 500 TRI loading
  matcher/               FIFO matching + transfer-in detection
  dividend/              Dividend fetching (Yahoo Finance + Python fallback)
  fno/                   F&O parsing + attribution
  scorer/                Alpha computation + FY grouping
  cockpit/               Unrealized portfolio analysis
  cleandata/             CSV read/write for clean data
  wizard/                Interactive reconciliation
  reconciliation/        Corporate actions (splits, demergers)
  output/                JSON serialization
ui/                      React scorecard UI (Vite + Tailwind)
ui-cockpit/              React cockpit UI (Vite + Tailwind)
scripts/                 Python helpers (dividends, prices, TRI)
```

### Cockpit Configuration

Per-client config at `data/{clientID}/cockpit_{clientID}.json`:

```json
{
  "client_id": "ZY7393",
  "client_name": "Vimal Kapur",
  "expected_total_income": 15000000,
  "default_hurdle_pct": 3,
  "classifications": {
    "MONIFTY500": {"asset_class": "index_mf", "hurdle_pct": 0},
    "PPFCF": {"asset_class": "active_mf", "hurdle_pct": 2},
    "GOLDBEES": {"asset_class": "gold_etf", "hurdle_pct": 3}
  },
  "price_only_symbols": ["GOLDBEES", "MONIFTY500", "LIQUIDBEES"],
  "ticker_map": {"MINDTREE": "LTIM.NS"},
  "display_names": {"RELIANCE": "Reliance Industries"}
}
```

**Asset classes:** `stock` (default, 3% hurdle), `active_mf` (2%), `gold_etf` (3%), `index_mf` (no test)

**Special cases:**
- Bonds: Add price manually to `data/{clientID}/prices_{date}.json`
- Mutual funds: NAVs not on Yahoo — fetch from [MFAPI](https://api.mfapi.in), add to price cache
- BSE symbols: Strip `-BE` suffix in unrealized CSV (e.g. `KWIL-BE` → `KWIL`)

## Input Files

### Zerodha Tradebook CSVs

Equity: 13 columns (`symbol,isin,trade_date,...,order_execution_time`)
F&O: 14 columns (same + `expiry_date`)

Files named `{clientID}_{startDate}_{endDate}.csv`

### NIFTY 500 TRI

Embedded in the binary. No external file needed.

### Unrealized Positions CSV

```csv
symbol,isin,buy_date,quantity,buy_price,invested
RELIANCE,INE002A01018,2020-10-27,2940,1004.94,2954530.0
```

Generated by `import`. Can be manually edited to add transfer-ins, MF holdings, bonds.

## Tech Stack

- **Backend:** Go (stdlib only, zero external dependencies)
- **Frontend:** React 19 + Vite + Tailwind CSS v4
- **Data:** Yahoo Finance (prices, dividends, stock TRI), MFAPI (mutual fund NAVs)
- **Hosting:** GitHub Pages
- **Build:** `go:embed` for TRI data, `CGO_ENABLED=0` for cross-compilation

## Known Limitations

- **Zerodha only** — parser expects Zerodha tradebook format
- **MF NAVs** — not available on Yahoo Finance, requires manual price injection
- **Bond prices** — not on Yahoo, requires manual price injection
- **BSE symbols** — `-BE` suffix must be stripped manually in unrealized CSV
- **Price staleness** — cached by report date, must clear cache or change date to refresh
