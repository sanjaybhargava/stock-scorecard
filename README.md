# stock-scorecard

A Go CLI + React UI that scores your equity trades against NIFTY 500 TRI. Parses Zerodha tradebook CSVs, performs FIFO matching, and calculates per-trade alpha to answer: **did you beat the market?**

**Live UI:** [https://sanjaybhargava.github.io/stock-scorecard/](https://sanjaybhargava.github.io/stock-scorecard/)

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+ (for the React UI)

### Build & Run

```bash
# Build the CLI
go build -o stock-scorecard ./cmd/scorecard

# Run the scorecard
./stock-scorecard \
  --tradebooks /path/to/tradebook/csvs/ \
  --tri /path/to/NIFTY500_TRI_Indexed.csv \
  --output ./ui/public/scorecard.json

# View the UI
cd ui && npm install && npm run dev
```

### CLI Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--tradebooks` | Yes | Directory containing Zerodha tradebook CSV files |
| `--tri` | Yes | Path to NIFTY 500 TRI Indexed CSV file |
| `--output` | Yes | Path for output JSON file |
| `--exclude` | No | Comma-separated symbols to skip (default: `LIQUIDBEES,GOLDBEES`) |
| `--broker` | No | Broker format (default: `zerodha`) |
| `--verbose` | No | Print per-symbol FIFO summary to stderr |

## Input Files

### Tradebook CSVs (Zerodha)

Files must match the naming convention `BT*.csv` with the standard Zerodha tradebook header:

```
symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time
```

### NIFTY 500 TRI Indexed CSV

Two columns: `Date,TRI_Indexed` with dates in `YYYY-MM-DD` format. Values are indexed to 100 on the start date.

## Architecture

```
Tradebook CSVs ──> Parse & Dedup ──> FIFO Match ──> Score & Aggregate ──> JSON
                        |                |                |                |
                   tradebook/        matcher/          scorer/          output/
                   parser.go         fifo.go           scorer.go        json.go
```

**Pipeline:**

1. **Parse & Dedup** — Read all `BT*.csv` files, deduplicate by `trade_id`, consolidate fills into VWAP trades
2. **Corporate Actions** — Adjust for known stock splits, demergers, and inject manual trades
3. **FIFO Match** — For each ISIN, match sells to oldest buy lots; unmatched buys become open positions
4. **Score** — Calculate alpha (equity G/L minus NIFTY 500 return on same capital over same period)
5. **JSON Output** — Serialize realized trades, open positions, warnings, and summary

The React UI (`ui/`) reads the output JSON and renders a 3-level drill-down scorecard:
- **Level 1:** Overall verdict, FY breakdown, COVID panic analysis, winner/loser rankings
- **Level 2:** Per-ticker breakdown within a FY/type bucket
- **Level 3:** Individual trade lots for a ticker

## Output JSON

See [CLAUDE.md](CLAUDE.md#output-json-structure) for the full JSON schema.

## Deploy to GitHub Pages

```bash
./deploy.sh
```

This builds the Go CLI, generates the scorecard JSON, builds the React UI, and pushes to the `gh-pages` branch.

## Known Limitations

- **No tests:** v1 shipped without test files. Adding tests is recommended for the team.
- **Stock splits that change ISIN:** Pre-split buys handled via `knownSplits` table in `fifo.go`. New splits require manual addition.
- **Zerodha only:** Parser expects Zerodha tradebook format. Other brokers require new parsers.
- **ETFs excluded:** ISINs starting with `INF` are filtered out.

## Tech Stack

- **Backend:** Go (stdlib only, no external dependencies)
- **Frontend:** React 19 + Vite + Tailwind CSS v4
- **Hosting:** GitHub Pages
