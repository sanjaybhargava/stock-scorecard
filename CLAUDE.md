# CLAUDE.md — stock-scorecard

## Project Overview

**stock-scorecard** is a Go CLI that parses Zerodha equity tradebook CSVs and NIFTY 500 TRI data to produce a JSON scorecard of realized trades with alpha calculations. Output is consumed by a React scorecard UI.

**Scope:** Equity segment only. No F&O. No option income attribution.

This is a **standalone module**, separate from `broker-trade-sync`. Different concerns, no shared code.

## Folder Structure

```
stock-scorecard/
├── cmd/
│   └── scorecard/
│       └── main.go              # CLI entry point
├── internal/
│   ├── tradebook/
│   │   └── parser.go            # Zerodha CSV parsing + dedup + consolidation
│   ├── tri/
│   │   └── loader.go            # TRI CSV loading + date lookup with fallback
│   ├── matcher/
│   │   └── fifo.go              # FIFO buy-sell matching
│   ├── scorer/
│   │   └── scorer.go            # Alpha computation, FY grouping, aggregation
│   └── output/
│       └── json.go              # JSON serialization
├── CLAUDE.md
├── go.mod
└── go.sum
```

## CLI Interface

```bash
go run ./cmd/scorecard \
  --tradebooks /path/to/tradebook/csvs/ \
  --tri /path/to/NIFTY500_TRI_Indexed.csv \
  --output ./output/scorecard.json \
  --exclude LIQUIDBEES,GOLDBEES
```

**Flags:**
- `--tradebooks` (required): Directory containing Zerodha tradebook CSV files
- `--tri` (required): Path to NIFTY 500 TRI Indexed CSV file
- `--output` (required): Path for output JSON file
- `--exclude` (optional): Comma-separated symbols to skip. Default: `LIQUIDBEES,GOLDBEES`
- `--broker` (optional): Broker format for parser selection. Default: `zerodha`

## Input Files

### Tradebook CSVs (Zerodha format)

Files follow naming convention `BT{client_id}_{startYYYYMMDD}_{endYYYYMMDD}.csv`.

**Columns:**
```
symbol,isin,trade_date,exchange,segment,series,trade_type,auction,quantity,price,trade_id,order_id,order_execution_time
```

**Key observations from real data:**
- `segment` is always `EQ` — equity-only tradebook
- `trade_type` is `buy` or `sell`
- `quantity` and `price` are floats (e.g., `4.000000`, `400.149994`)
- Multiple fill rows per order — same `order_id`, different `trade_id`, different quantities/prices
- Dates are `YYYY-MM-DD`; `order_execution_time` is `YYYY-MM-DDTHH:MM:SS`
- **Dedup by `trade_id`** globally — defensive against duplicate files
- **Exclude symbols:** `LIQUIDBEES`, `GOLDBEES` (parking instruments)

**Verified file inventory (6 files, all unique):**

| Filename | Rows | Unique Trade IDs | Date Range |
|---|---|---|---|
| `BT2632_20190401_20200331.csv` | 9,824 | 4,912 | Aug 2019 → Mar 2020 |
| `BT2632_20200401_20210331.csv` | 1,512 | 1,511 | Jul 2020 → Mar 2021 |
| `BT2632_20210401_20220331.csv` | 664 | 664 | May 2021 → Feb 2022 |
| `BT2632_20220401_20230331.csv` | 1,053 | 1,053 | Apr 2022 → Mar 2023 |
| `BT2632_20230401_20240331.csv` | 769 | 769 | Apr 2023 → Mar 2024 |
| `BT2632_20240401_20250331.csv` | 1,063 | 1,063 | Apr 2024 → Jan 2025 |

**Totals:** 14,885 rows, 9,972 unique trade IDs, continuous Aug 2019 → Jan 2025.

### NIFTY 500 TRI Indexed CSV

**Columns:**
```
Date,TRI_Indexed
```
- Dates `YYYY-MM-DD`, values are floats indexed to 100 on start date
- 2,478 rows, Feb 2016 → Feb 2026 — covers all tradebook dates

## Processing Pipeline

### Step 1: Parse & Dedup (internal/tradebook/parser.go)

1. Read all `*.csv` files from input directory; skip files that don't match Zerodha header
2. Parse each row into a `Trade` struct
3. **Dedup by `trade_id`** globally
4. **Consolidate fills:** Group by `(symbol, trade_date, trade_type, order_id)`, compute VWAP and total quantity
5. Round quantities to integers after consolidation (equity = whole shares)
6. Result: one `ConsolidatedTrade` per symbol per day per side per order

```go
type ConsolidatedTrade struct {
    Symbol    string
    ISIN      string
    Date      time.Time
    TradeType string    // "buy" or "sell"
    Quantity  float64
    AvgPrice  float64   // VWAP = Σ(qty × price) / Σ(qty)
    Value     float64   // Quantity × AvgPrice
    OrderID   string
}
```

### Step 2: Load TRI Index (internal/tri/loader.go)

- Load into `map[string]float64` keyed by `YYYY-MM-DD`
- For non-trading days (weekends/holidays): fall back to **most recent prior trading day**

### Step 3: FIFO Matching (internal/matcher/fifo.go)

For each symbol:
1. Sort consolidated trades by date
2. Maintain a FIFO queue of buy lots
3. On sell: consume oldest buy lot(s), splitting partial lots as needed
4. Each matched pair → `RealizedTrade`
5. Remaining buy lots → `OpenPosition` (still held)

```go
type RealizedTrade struct {
    Symbol     string
    BuyDate    time.Time
    SellDate   time.Time
    HoldDays   int
    Quantity   float64
    BuyPrice   float64
    SellPrice  float64
    Invested   float64   // Quantity × BuyPrice
    SaleValue  float64   // Quantity × SellPrice
    EquityGL   float64   // SaleValue - Invested
    NiftyBuy   float64   // TRI on buy date
    NiftySell  float64   // TRI on sell date
    NiftyReturn float64  // Invested × (NiftySell/NiftyBuy - 1)
    FY         string    // FY of SELL date, e.g. "FY 2024-25"
    Type       string    // "Long" if HoldDays > 365, else "Short"
}

type OpenPosition struct {
    Symbol   string
    BuyDate  time.Time
    Quantity float64
    BuyPrice float64
    Invested float64
}
```

### Step 4: Score & Aggregate (internal/scorer/scorer.go)

- **Alpha** = EquityGL - NiftyReturn (per trade)
- A ticker "passes" if aggregate alpha across all lots in a FY/type bucket >= 0
- **Win rate** = % of unique (ticker, FY, type) combos with positive alpha
- **FY determination:** Based on sell date. Apr 1 to Mar 31 → e.g., "FY 2024-25"
- **Long vs Short:** HoldDays > 365 = Long, else Short (Indian LTCG/STCG)

### Step 5: JSON Output (internal/output/json.go)

Serialize to the JSON structure defined below.

## Output JSON Structure

```json
{
  "generated_at": "2026-02-24T12:00:00Z",
  "trades": [
    {
      "fy": "FY 2024-25",
      "type": "Long",
      "ticker": "BHARTIARTL",
      "buy_date": "2023-08-30",
      "sell_date": "2025-01-06",
      "hold_days": 494,
      "quantity": 950,
      "buy_price": 859.70,
      "sell_price": 1600.00,
      "invested": 816715,
      "sale_value": 1520000,
      "equity_gl": 703285,
      "nifty_buy_tri": 280.50,
      "nifty_sell_tri": 410.20,
      "nifty_return": 377500,
      "alpha": 325785
    }
  ],
  "open_positions": [
    {
      "ticker": "SYMBOL",
      "buy_date": "2024-06-15",
      "quantity": 500,
      "buy_price": 1200.00,
      "invested": 600000,
      "note": "No matching sell — still held"
    }
  ],
  "summary": {
    "total_trades": 45,
    "total_invested": 12500000,
    "total_my_return": 2400000,
    "total_nifty_return": 1800000,
    "net_alpha": 600000,
    "win_rate": 62,
    "by_fy": [
      {
        "fy": "FY 2024-25",
        "type": "Long",
        "num_trades": 12,
        "invested": 4500000,
        "my_return": 800000,
        "nifty_return": 600000,
        "alpha": 200000
      }
    ]
  }
}
```

## Edge Cases

- **Partial sells (FIFO splitting):** A buy of 1000 may sell across 3 dates (300, 500, 200) — split buy lots
- **Cross-FY holdings:** Bought FY20-21, sold FY24-25 — single realized trade, FY = sell date
- **Missing TRI dates:** Weekend/holiday → use most recent prior trading day
- **Open positions:** Buy lots without matching sells → `open_positions` array, not in realized trades
- **Symbol name changes:** ISIN is stable across renames (e.g., MOTHERSUMI → MOTHERSON). Use ISIN for matching, display most recent symbol name
- **Quantity precision:** Round to int after consolidation — equity = whole shares

## Testing Strategy

1. **INDHOTEL trace:** ~2000 shares bought @ ~₹579.75 on Apr 8, 2024; sold @ ₹707 on Oct 15, 2024 → one realized trade, ~₹254K equity G/L
2. **BHARTIARTL cross-FY:** Bought in FY20-21 file, sold in FY24-25 → verify FIFO across file boundaries
3. **Open positions:** Stocks with unmatched buys → appear in `open_positions`
4. **Dedup resilience:** Duplicate a file, verify trade count unchanged
5. **TRI weekend fallback:** Trade on Monday, verify Friday TRI used if Monday missing

## Coding Conventions

- `gofmt` for formatting
- Idiomatic Go error handling: check and wrap with context
- `log` package for output (not `fmt.Println` for errors)
- Package names: lowercase, single word
- Clean separation of concerns per the folder structure
- No external dependencies beyond stdlib (CSV parsing, JSON, flags — all stdlib)

## Commands

```bash
# Run
go run ./cmd/scorecard --tradebooks ~/Downloads --tri ~/Downloads/NIFTY500_TRI_Indexed.csv --output ./scorecard.json

# Build
go build -o stock-scorecard ./cmd/scorecard

# Test
go test ./...
```

## Build Status

- [ ] Step 1: CSV parsing + dedup + consolidation
- [ ] Step 2: TRI index loading
- [ ] Step 3: FIFO matching
- [ ] Step 4: Scoring + aggregation
- [ ] Step 5: JSON output
- [ ] Step 6: CLI wiring + end-to-end test

## Future (not v1)

- F&O option income attribution
- Dividend income matching
- Brokerage/STT deduction
- Unrealized scorecard (live prices)
- Multi-broker parsers (Groww, ICICI)
- React component generation
