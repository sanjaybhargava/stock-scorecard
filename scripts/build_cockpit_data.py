#!/usr/bin/env python3
"""
Build cockpit.json and hulData.js for the unrealized holdings cockpit.

Usage:
    python3 scripts/build_cockpit_data.py

Inputs (from ~/Downloads/):
    - unrealized_ZY7393.csv          121 unrealized lots
    - NIFTY500_TRI_Indexed.csv       Nifty 500 TRI daily
    - HUL_daily_prices.csv           HUL closing prices
    - HUL_TRI_Indexed.csv            HUL TRI index
    - HUL_dividends_verified (1).csv HUL dividends
    - NSE_Monthly_Expiry_Dates.csv   Monthly expiry dates

Outputs:
    - ui-cockpit/public/cockpit.json
    - ui-cockpit/src/data/hulData.js
    - scripts/prices_20260223.json   (cache)
"""

import csv
import json
import os
import ssl
import sys
import urllib.request
import urllib.error
import time
from datetime import datetime, timedelta
from pathlib import Path

# Work around macOS Python SSL cert issues
SSL_CTX = ssl.create_default_context()
try:
    import certifi
    SSL_CTX.load_verify_locations(certifi.where())
except ImportError:
    SSL_CTX.check_hostname = False
    SSL_CTX.verify_mode = ssl.CERT_NONE

DOWNLOADS = os.path.expanduser("~/Downloads")
PROJECT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPORT_DATE = "2026-02-23"

# Symbols where stock TRI is not meaningful (ETFs/MFs) — use price-only CAGR
PRICE_ONLY_SYMBOLS = {"GOLDBEES", "MONIFTY500", "MOMENTUM50"}

# Yahoo Finance ticker overrides
TICKER_MAP = {
    "MOMENTUM50": "0P0001L1GH.BO",   # ICICI Pru Nifty200 Momentum 30 Index Fund
    "MINDTREE": "LTIM.NS",            # LTIMindtree (post-merger ticker)
    "MONIFTY500": "0P0001NJBI.BO",    # ICICI Pru Nifty 500 Index Fund
    "GOLDBEES": "GOLDBEES.NS",
    "HNDFDS": "HNDFDS.NS",
    "NAUKRI": "NAUKRI.NS",
}

# Classification: symbol -> (asset_class, hurdle_pct)
CLASSIFICATIONS = {
    "MONIFTY500": ("index_mf", 0),     # IS the benchmark — no test
    "MOMENTUM50": ("active_mf", 2),    # active fund, 2% hurdle
    "GOLDBEES":   ("gold_etf", 3),     # gold ETF, 3% hurdle
}

# Human-readable names
DISPLAY_NAMES = {
    "RELIANCE": "Reliance Industries",
    "BHARTIARTL": "Bharti Airtel",
    "NTPC": "NTPC",
    "COALINDIA": "Coal India",
    "TATASTEEL": "Tata Steel",
    "HAL": "Hindustan Aeronautics",
    "SIEMENS": "Siemens",
    "BRITANNIA": "Britannia Industries",
    "CIPLA": "Cipla",
    "APOLLOHOSP": "Apollo Hospitals",
    "HAVELLS": "Havells India",
    "PIDILITIND": "Pidilite Industries",
    "INDIGO": "InterGlobe Aviation",
    "NAUKRI": "Info Edge (Naukri)",
    "DIXON": "Dixon Technologies",
    "TORNTPHARM": "Torrent Pharma",
    "THERMAX": "Thermax",
    "AFFLE": "Affle India",
    "MUTHOOTFIN": "Muthoot Finance",
    "IGL": "Indraprastha Gas",
    "KFINTECH": "KFin Technologies",
    "PRAJIND": "Praj Industries",
    "VAIBHAVGBL": "Vaibhav Global",
    "RBLBANK": "RBL Bank",
    "IDFCFIRSTB": "IDFC First Bank",
    "HNDFDS": "Hindustan Foods",
    "MINDTREE": "LTIMindtree",
    "GOLDBEES": "Nippon Gold ETF",
    "MOMENTUM50": "ICICI Nifty200 Momentum 30",
    "MONIFTY500": "ICICI Nifty 500 Index",
    "HINDUNILVR": "Hindustan Unilever",
}


def load_csv(path):
    """Load CSV file, return list of dicts."""
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def load_tri(path):
    """Load TRI CSV into {date_str: value} dict."""
    tri = {}
    for row in load_csv(path):
        val = row["TRI_Indexed"].strip()
        if val:
            tri[row["Date"]] = float(val)
    return tri


def tri_lookup(tri, date_str):
    """Look up TRI with nearest prior date fallback."""
    if date_str in tri:
        return tri[date_str]
    # Binary search backward
    dt = datetime.strptime(date_str, "%Y-%m-%d")
    for i in range(1, 30):
        prev = (dt - timedelta(days=i)).strftime("%Y-%m-%d")
        if prev in tri:
            return tri[prev]
    return None


def fetch_yahoo_chart(ticker, period1, period2, interval="1d"):
    """Fetch chart data from Yahoo Finance. Returns list of (date_str, close)."""
    url = (
        f"https://query1.finance.yahoo.com/v8/finance/chart/{ticker}"
        f"?period1={period1}&period2={period2}&interval={interval}"
    )

    req = urllib.request.Request(url, headers={
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
    })

    with urllib.request.urlopen(req, timeout=15, context=SSL_CTX) as resp:
        data = json.loads(resp.read().decode())

    result = data["chart"]["result"][0]
    timestamps = result["timestamp"]
    closes = result["indicators"]["quote"][0]["close"]

    out = []
    for ts, close in zip(timestamps, closes):
        if close is not None:
            dt = datetime.utcfromtimestamp(ts)
            out.append((dt.strftime("%Y-%m-%d"), round(close, 2)))
    return out


def fetch_yahoo_price(ticker, report_date):
    """Fetch closing price from Yahoo Finance chart API."""
    dt = datetime.strptime(report_date, "%Y-%m-%d")
    period1 = int((dt - timedelta(days=7)).timestamp())
    period2 = int((dt + timedelta(days=1)).timestamp())

    try:
        rows = fetch_yahoo_chart(ticker, period1, period2)
        # Find the closest date on or before report_date
        best = None
        for date_str, close in rows:
            if date_str <= report_date:
                best = close
        return best
    except (urllib.error.URLError, KeyError, TypeError, json.JSONDecodeError) as e:
        print(f"  WARNING: Failed to fetch {ticker}: {e}")
        return None


def fetch_historical_prices(ticker, start_date="2016-01-01", end_date=REPORT_DATE):
    """Fetch daily prices from Yahoo Finance for a date range. Returns {date_str: close}."""
    dt_start = datetime.strptime(start_date, "%Y-%m-%d")
    dt_end = datetime.strptime(end_date, "%Y-%m-%d")
    period1 = int(dt_start.timestamp())
    period2 = int((dt_end + timedelta(days=1)).timestamp())

    rows = fetch_yahoo_chart(ticker, period1, period2)
    return {date_str: close for date_str, close in rows}


# --- Market phases (Nifty regime periods) ---
MARKET_PHASES = [
    {"regime": "Bull",     "icon": "↗", "start": "2016-02-24", "end": "2018-01-31"},
    {"regime": "Bear",     "icon": "↘", "start": "2018-02-01", "end": "2020-03-23"},
    {"regime": "Bull",     "icon": "↗", "start": "2020-03-24", "end": "2024-09-30"},
    {"regime": "Sideways", "icon": "→", "start": "2024-10-01", "end": "2026-02-23"},
]


def nearest_price(prices, date_str, direction="back"):
    """Find nearest price on or before (back) or on or after (forward) date."""
    if date_str in prices:
        return prices[date_str]
    dt = datetime.strptime(date_str, "%Y-%m-%d")
    for i in range(1, 30):
        d = (dt - timedelta(days=i) if direction == "back" else dt + timedelta(days=i))
        key = d.strftime("%Y-%m-%d")
        if key in prices:
            return prices[key]
    return None


def compute_cagr(start_val, end_val, years):
    """Compute CAGR as percentage."""
    if start_val <= 0 or years <= 0:
        return 0
    return ((end_val / start_val) ** (1 / years) - 1) * 100


def load_fno_contracts(csv_path):
    """Parse clean F&O CSV into ContractPnL list, grouped by (underlying, raw_symbol).

    Returns list of dicts: {underlying, raw_symbol, option_type, expiry_date,
                            first_date, net_pnl}
    """
    rows = load_csv(csv_path)
    # Group by (underlying, raw_symbol)
    groups = {}  # key -> accum dict
    key_order = []
    for row in rows:
        key = (row["underlying"], row["raw_symbol"])
        if key not in groups:
            groups[key] = {
                "underlying": row["underlying"],
                "raw_symbol": row["raw_symbol"],
                "option_type": row["option_type"],
                "expiry_date": row["expiry_date"],
                "first_date": row["trade_date"],
                "sell_value": 0.0,
                "buy_value": 0.0,
            }
            key_order.append(key)
        g = groups[key]
        if row["trade_date"] < g["first_date"]:
            g["first_date"] = row["trade_date"]
        value = float(row["value"])
        if row["trade_type"] == "sell":
            g["sell_value"] += value
        else:
            g["buy_value"] += value

    contracts = []
    for key in key_order:
        g = groups[key]
        contracts.append({
            "underlying": g["underlying"],
            "raw_symbol": g["raw_symbol"],
            "option_type": g["option_type"],
            "expiry_date": g["expiry_date"],
            "first_date": g["first_date"],
            "net_pnl": round(g["sell_value"] - g["buy_value"]),
        })
    print(f"Loaded {len(contracts)} F&O contracts from {csv_path}")
    return contracts


def attribute_fno(contracts, lots, report_date):
    """Two-pass F&O attribution (same algorithm as internal/fno/attributor.go).

    Args:
        contracts: list of ContractPnL dicts from load_fno_contracts()
        lots: list of lot dicts, each must have: symbol, buy_date, quantity,
              and optionally sell_date (for realized trades; defaults to report_date)
        report_date: string "YYYY-MM-DD" — used as sell_date for unrealized lots

    Returns:
        (attribution, unattributed_total, unattrib_by_symbol)
        - attribution: {lot_index: option_income}
        - unattributed_total: float
        - unattrib_by_symbol: {symbol: net_pnl}
    """
    # Build lookup: underlying -> [(index, lot)]
    by_underlying = {}
    for i, lot in enumerate(lots):
        sym = lot["symbol"]
        by_underlying.setdefault(sym, []).append((i, lot))

    attribution = {}  # lot_index -> total option income
    unattrib_by_sym = {}  # underlying -> total unattributed pnl

    for c in contracts:
        underlying = c["underlying"]
        candidates = by_underlying.get(underlying, [])
        if not candidates:
            unattrib_by_sym[underlying] = unattrib_by_sym.get(underlying, 0) + c["net_pnl"]
            continue

        contract_start = c["first_date"]
        contract_end = c["expiry_date"]

        # Pass 1: overlap-based attribution (CE + PE)
        weights = []
        total_weight = 0.0
        for idx, lot in candidates:
            buy_date = lot["buy_date"]
            sell_date = lot.get("sell_date", report_date)
            overlap = _overlap_days(contract_start, contract_end, buy_date, sell_date)
            if overlap > 0:
                w = lot["quantity"] * overlap
                weights.append((idx, w))
                total_weight += w

        # Pass 2: next-buy fallback (PE only, no overlap found)
        if total_weight == 0 and c["option_type"] == "PE":
            weights, total_weight = _next_buy_attribution(c, candidates)

        if total_weight == 0:
            unattrib_by_sym[underlying] = unattrib_by_sym.get(underlying, 0) + c["net_pnl"]
            continue

        # Distribute pro-rata
        for idx, w in weights:
            share = c["net_pnl"] * (w / total_weight)
            attribution[idx] = attribution.get(idx, 0) + share

    unattributed_total = sum(unattrib_by_sym.values())

    # Log summary
    total_attributed = sum(attribution.values())
    print(f"F&O attribution: {len(attribution)} lots received option income, "
          f"attributed: ₹{total_attributed/100000:.1f}L, "
          f"unattributed: ₹{unattributed_total/100000:.1f}L ({len(unattrib_by_sym)} underlyings)")

    return attribution, unattributed_total, unattrib_by_sym


def _overlap_days(contract_start, contract_end, buy_date, sell_date):
    """Compute overlap days between contract [start, end] and holding [buy, sell]."""
    start = max(contract_start, buy_date)
    end = min(contract_end, sell_date)
    if start >= end:
        return 0
    d_start = datetime.strptime(start, "%Y-%m-%d")
    d_end = datetime.strptime(end, "%Y-%m-%d")
    return (d_end - d_start).days + 1


def _next_buy_attribution(contract, candidates):
    """For PE contracts with no overlap, find nearest lot buy_date >= contract expiry."""
    expiry = contract["expiry_date"]
    nearest_buy = None
    for _, lot in candidates:
        buy = lot["buy_date"]
        if buy >= expiry:
            if nearest_buy is None or buy < nearest_buy:
                nearest_buy = buy

    if nearest_buy is None:
        return [], 0.0

    weights = []
    total_weight = 0.0
    for idx, lot in candidates:
        if lot["buy_date"] == nearest_buy:
            w = lot["quantity"]
            weights.append((idx, w))
            total_weight += w

    return weights, total_weight


def load_stock_tri(path):
    """Load stock TRI data from JSON. Returns {symbol: {date_str: tri_value}}."""
    if not os.path.exists(path):
        print(f"  WARNING: Stock TRI file not found: {path}")
        print(f"  Run: python3 scripts/fetch_stock_tri.py")
        return {}
    with open(path) as f:
        data = json.load(f)
    print(f"Loaded stock TRI for {len(data)} tickers from {path}")
    return data


def stock_tri_lookup(stock_tri, symbol, date_str):
    """Look up stock TRI value with nearest prior date fallback."""
    if symbol not in stock_tri or symbol in PRICE_ONLY_SYMBOLS:
        return None
    tri = stock_tri[symbol]
    if date_str in tri:
        return tri[date_str]
    dt = datetime.strptime(date_str, "%Y-%m-%d")
    for i in range(1, 30):
        prev = (dt - timedelta(days=i)).strftime("%Y-%m-%d")
        if prev in tri:
            return tri[prev]
    return None


def compute_max_drawdown(prices, start_date, end_date):
    """Compute max drawdown (%) for prices within a date range."""
    sorted_dates = sorted(d for d in prices if start_date <= d <= end_date)
    if len(sorted_dates) < 2:
        return 0
    peak = 0
    max_dd = 0
    for d in sorted_dates:
        p = prices[d]
        if p > peak:
            peak = p
        dd = (peak - p) / peak * 100 if peak > 0 else 0
        if dd > max_dd:
            max_dd = dd
    return round(max_dd, 1)


def compute_phase_data(stock_prices, nifty_tri, earliest_buy=None, symbol=None, stock_tri_data=None):
    """Compute phase performance and drawdown data for a stock.
    Uses stock TRI (dividend-adjusted) for CAGR if available, price-only for drawdowns."""
    phases = []
    drawdowns = []

    for phase in MARKET_PHASES:
        start = phase["start"]
        end = phase["end"]
        years = (datetime.strptime(end, "%Y-%m-%d") - datetime.strptime(start, "%Y-%m-%d")).days / 365.25

        # Check if we have price data covering this phase
        phase_prices = [d for d in stock_prices if start <= d <= end]
        listed = len(phase_prices) >= 20  # need at least ~1 month of data

        if not listed:
            phases.append({
                "regime": phase["regime"], "icon": phase["icon"],
                "period": f"{_fmt_date(start)} → {_fmt_date(end)}",
                "stockCagr": 0, "niftyCagr": 0, "listed": False,
            })
            drawdowns.append({
                "regime": phase["regime"], "icon": phase["icon"],
                "period": f"{_fmt_date(start)} → {_fmt_date(end)}",
                "stockDD": 0, "niftyDD": 0, "listed": False,
            })
            continue

        # Stock CAGR — prefer TRI (dividend-adjusted), fall back to price-only
        stock_cagr = 0
        used_tri = False
        if symbol and stock_tri_data:
            tri_start = stock_tri_lookup(stock_tri_data, symbol, start)
            tri_end = stock_tri_lookup(stock_tri_data, symbol, end)
            if tri_start and tri_end:
                stock_cagr = compute_cagr(tri_start, tri_end, years)
                used_tri = True
        if not used_tri:
            sp_start = nearest_price(stock_prices, start, "forward")
            sp_end = nearest_price(stock_prices, end, "back")
            stock_cagr = compute_cagr(sp_start, sp_end, years) if sp_start and sp_end else 0

        # Nifty CAGR
        nt_start = tri_lookup(nifty_tri, start)
        nt_end = tri_lookup(nifty_tri, end)
        nifty_cagr = compute_cagr(nt_start, nt_end, years) if nt_start and nt_end else 0

        phases.append({
            "regime": phase["regime"], "icon": phase["icon"],
            "period": f"{_fmt_date(start)} → {_fmt_date(end)}",
            "stockCagr": round(stock_cagr, 1), "niftyCagr": round(nifty_cagr, 1),
            "listed": True,
        })

        # Drawdowns
        stock_dd = compute_max_drawdown(stock_prices, start, end)

        # Nifty drawdown from TRI
        nifty_prices_phase = {d: v for d, v in nifty_tri.items() if start <= d <= end}
        nifty_dd = compute_max_drawdown(nifty_prices_phase, start, end)

        drawdowns.append({
            "regime": phase["regime"], "icon": phase["icon"],
            "period": f"{_fmt_date(start)} → {_fmt_date(end)}",
            "stockDD": stock_dd, "niftyDD": nifty_dd, "listed": True,
        })

    return phases, drawdowns


def _fmt_date(date_str):
    """Format '2016-02-24' -> 'Feb 2016'."""
    dt = datetime.strptime(date_str, "%Y-%m-%d")
    return dt.strftime("%b %Y")


def build_deep_dive_data(fail_list, lots_by_symbol, prices, nifty_tri, stock_tri_data=None):
    """Fetch historical prices and compute deep-dive card data for all failed stocks."""
    print("\n=== Building deep-dive data for failed stocks ===")

    hist_cache_path = os.path.join(PROJECT, "scripts", "historical_prices.json")
    if os.path.exists(hist_cache_path):
        with open(hist_cache_path) as f:
            hist_cache = json.load(f)
        print(f"Loaded historical prices for {len(hist_cache)} symbols from cache")
    else:
        hist_cache = {}

    for stock in fail_list:
        sym = stock["symbol"]
        asset_class = stock.get("asset_class", "stock")

        # Only compute deep dive for stocks (not MFs/ETFs)
        if asset_class != "stock":
            stock["deep_dive"] = False
            continue

        stock["deep_dive"] = True

        # Fetch historical prices if not cached
        if sym not in hist_cache or len(hist_cache[sym]) < 100:
            ticker = TICKER_MAP.get(sym, f"{sym}.NS")
            print(f"  Fetching 10y history for {ticker}...", end=" ", flush=True)
            try:
                hp = fetch_historical_prices(ticker)
                hist_cache[sym] = hp
                print(f"{len(hp)} days")
            except Exception as e:
                print(f"FAILED: {e}")
                stock["deep_dive"] = False
                continue
            time.sleep(0.5)
        else:
            print(f"  {sym}: {len(hist_cache[sym])} days (cached)")

        stock_prices = hist_cache[sym]

        # Earliest buy date for this stock
        lots = lots_by_symbol.get(sym, [])
        earliest_buy = min(l["buy_date"] for l in lots) if lots else "2016-02-24"

        # Compute phase data (uses stock TRI for CAGR, price-only for drawdowns)
        phases, drawdowns = compute_phase_data(stock_prices, nifty_tri, earliest_buy,
                                                symbol=sym, stock_tri_data=stock_tri_data)

        # Compute aggregate for redeployment/terminal value cards
        total_shares = sum(l["quantity"] for l in lots)
        total_invested = sum(l["invested"] for l in lots)
        current_price = prices.get(sym, 0)
        cost_per_share = total_invested / total_shares if total_shares > 0 else 0

        stock["cards"] = {
            "phases": phases,
            "drawdowns": drawdowns,
            "redeployment": {
                "name": stock["name"],
                "shares": total_shares,
                "price": round(current_price, 2),
                "costPerShare": round(cost_per_share, 2),
                "taxRate": 14.95,
            },
            "terminal": {
                "name": stock["name"],
                "ticker": sym,
                "totalShares": total_shares,
                "price": round(current_price, 2),
                "costPerShare": round(cost_per_share, 2),
                "taxRate": 14.95,
                "niftyCagr": 16.2,
                "hcCagr": 25,
                "niftyPct": 80,
                "hcPct": 20,
                "years": 5,
            },
        }

    # Save historical cache
    with open(hist_cache_path, "w") as f:
        json.dump(hist_cache, f)
    print(f"Saved historical prices to {hist_cache_path}")

    # Also do pass list deep dive (same logic)
    return fail_list


def fetch_all_prices(symbols, report_date, cache_path):
    """Fetch current prices for all symbols, with caching."""
    # Load cache if exists
    if os.path.exists(cache_path):
        with open(cache_path) as f:
            cache = json.load(f)
        print(f"Loaded {len(cache)} cached prices from {cache_path}")
    else:
        cache = {}

    for sym in symbols:
        if sym in cache and cache[sym] is not None:
            print(f"  {sym}: ₹{cache[sym]:,.2f} (cached)")
            continue

        ticker = TICKER_MAP.get(sym, f"{sym}.NS")
        print(f"  Fetching {ticker}...", end=" ", flush=True)
        price = fetch_yahoo_price(ticker, report_date)
        if price:
            print(f"₹{price:,.2f}")
            cache[sym] = price
        else:
            print("FAILED")

        time.sleep(0.5)  # rate limit

    # Save cache
    with open(cache_path, "w") as f:
        json.dump(cache, f, indent=2)
    print(f"Saved prices to {cache_path}")

    return cache


def build_cockpit_json():
    """Build cockpit.json from unrealized lots + TRI + prices."""
    print("=== Building cockpit.json ===")

    # Load unrealized lots
    lots_raw = load_csv(os.path.join(DOWNLOADS, "unrealized_ZY7393.csv"))
    print(f"Loaded {len(lots_raw)} lots from unrealized_ZY7393.csv")

    # Load HUL price for report date
    hul_prices = load_csv(os.path.join(DOWNLOADS, "HUL_daily_prices.csv"))
    hul_price_map = {r["Date"]: float(r["Close_INR"]) for r in hul_prices}
    hul_current = hul_price_map.get(REPORT_DATE)
    print(f"HUL price on {REPORT_DATE}: ₹{hul_current:,.2f}")

    # Add HUL manual lot (transfer-in)
    lots_raw.append({
        "symbol": "HINDUNILVR",
        "isin": "INE030A01027",
        "buy_date": "2016-02-26",
        "quantity": "25400",
        "buy_price": "830.0",
        "invested": "21082000",
    })

    # Load Nifty 500 TRI
    tri = load_tri(os.path.join(DOWNLOADS, "NIFTY500_TRI_Indexed.csv"))
    print(f"Loaded {len(tri)} TRI data points")

    # Get unique symbols
    symbols = sorted(set(r["symbol"] for r in lots_raw))
    print(f"Symbols: {len(symbols)}")

    # Fetch prices from Yahoo Finance
    cache_path = os.path.join(PROJECT, "scripts", "prices_20260223.json")
    # Add HUL to prices from local data
    prices = fetch_all_prices(
        [s for s in symbols if s != "HINDUNILVR"],
        REPORT_DATE,
        cache_path
    )
    prices["HINDUNILVR"] = hul_current

    # Load stock TRI (Adj Close indexed to 100) for dividend-adjusted CAGR
    stock_tri_path = os.path.join(PROJECT, "scripts", "stock_tri.json")
    stock_tri_data = load_stock_tri(stock_tri_path)

    # Process each lot
    lots_by_symbol = {}
    for row in lots_raw:
        sym = row["symbol"]
        if sym not in lots_by_symbol:
            lots_by_symbol[sym] = []

        buy_date = row["buy_date"]
        quantity = int(float(row["quantity"]))
        buy_price = float(row["buy_price"])
        invested = round(quantity * buy_price)

        nifty_buy = tri_lookup(tri, buy_date)
        nifty_report = tri_lookup(tri, REPORT_DATE)

        if nifty_buy is None or nifty_report is None:
            print(f"  WARNING: No TRI for {sym} buy_date={buy_date}")
            continue

        days_held = (datetime.strptime(REPORT_DATE, "%Y-%m-%d") -
                     datetime.strptime(buy_date, "%Y-%m-%d")).days
        years_held = days_held / 365.25

        shadow_nifty_value = invested * (nifty_report / nifty_buy)
        nifty_cagr = ((nifty_report / nifty_buy) ** (1 / years_held) - 1) * 100 if years_held > 0 else 0

        current_price = prices.get(sym)
        if current_price is None:
            print(f"  WARNING: No price for {sym}")
            continue

        current_value = round(quantity * current_price)

        # Stock CAGR — prefer TRI (dividend-adjusted), fall back to price-only
        stock_cagr = 0
        if years_held > 0 and invested > 0:
            tri_buy = stock_tri_lookup(stock_tri_data, sym, buy_date)
            tri_report = stock_tri_lookup(stock_tri_data, sym, REPORT_DATE)
            if tri_buy and tri_report and tri_buy > 0:
                stock_cagr = ((tri_report / tri_buy) ** (1 / years_held) - 1) * 100
            else:
                stock_cagr = ((current_value / invested) ** (1 / years_held) - 1) * 100

        lots_by_symbol[sym].append({
            "buy_date": buy_date,
            "quantity": quantity,
            "buy_price": round(buy_price, 2),
            "invested": invested,
            "current_price": round(current_price, 2),
            "current_value": current_value,
            "nifty_buy_tri": round(nifty_buy, 4),
            "nifty_report_tri": round(nifty_report, 4),
            "shadow_nifty": round(shadow_nifty_value),
            "nifty_cagr": round(nifty_cagr, 2),
            "stock_cagr": round(stock_cagr, 2),
            "days_held": days_held,
            "years_held": round(years_held, 2),
            "too_early": years_held < 1.0,
        })

    # --- F&O attribution ---
    fno_csv = os.path.join(PROJECT, "data", "ZY7393", "fno_ZY7393.csv")
    realized_csv = os.path.join(DOWNLOADS, "realized_ZY7393.csv")
    fno_summary = {"total_attributed": 0, "total_unattributed": 0, "by_symbol": {}}

    if os.path.exists(fno_csv):
        contracts = load_fno_contracts(fno_csv)

        # Build flat list of unrealized lots for attribution
        unrealized_flat = []
        unrealized_keys = []  # (symbol, index_in_lots_by_symbol)
        for sym in sorted(lots_by_symbol.keys()):
            for i, lot in enumerate(lots_by_symbol[sym]):
                unrealized_flat.append({
                    "symbol": sym,
                    "buy_date": lot["buy_date"],
                    "quantity": lot["quantity"],
                    # No sell_date — unrealized, will use report_date
                })
                unrealized_keys.append((sym, i))

        # Load realized trades as additional candidates (to avoid over-attributing)
        realized_lots = []
        if os.path.exists(realized_csv):
            realized_raw = load_csv(realized_csv)
            for row in realized_raw:
                realized_lots.append({
                    "symbol": row["symbol"],
                    "buy_date": row["buy_date"],
                    "sell_date": row["sell_date"],
                    "quantity": int(float(row["quantity"])),
                })
            print(f"Loaded {len(realized_lots)} realized trades for F&O attribution")

        # Combine: unrealized first (index 0..N-1), then realized (index N..N+M-1)
        all_lots = unrealized_flat + realized_lots
        attribution, unattrib_total, unattrib_by_sym = attribute_fno(
            contracts, all_lots, REPORT_DATE
        )

        # Apply option_income to unrealized lots only (indices 0..N-1)
        n_unrealized = len(unrealized_flat)
        fno_by_symbol = {}
        for idx, income in attribution.items():
            if idx < n_unrealized:
                sym, lot_idx = unrealized_keys[idx]
                lot = lots_by_symbol[sym][lot_idx]
                lot["option_income"] = round(income)
                fno_by_symbol[sym] = fno_by_symbol.get(sym, 0) + round(income)

                # Recompute stock_cagr including option_income
                invested = lot["invested"]
                years_held = lot["years_held"]
                if years_held > 0 and invested > 0:
                    tri_buy = stock_tri_lookup(stock_tri_data, sym, lot["buy_date"])
                    tri_report = stock_tri_lookup(stock_tri_data, sym, REPORT_DATE)
                    if tri_buy and tri_report and tri_buy > 0:
                        tri_return = invested * (tri_report / tri_buy)
                        total_return = tri_return + lot["option_income"]
                    else:
                        total_return = lot["current_value"] + lot["option_income"]
                    ratio = total_return / invested
                    if ratio > 0:
                        lot["stock_cagr"] = round(
                            (ratio ** (1 / years_held) - 1) * 100, 2
                        )
                    else:
                        lot["stock_cagr"] = round(-100.0, 2)

        # Ensure all lots have option_income key
        for sym_lots in lots_by_symbol.values():
            for lot in sym_lots:
                lot.setdefault("option_income", 0)

        total_attributed = sum(
            income for idx, income in attribution.items() if idx < n_unrealized
        )
        fno_summary = {
            "total_attributed": round(total_attributed),
            "total_unattributed": round(unattrib_total),
            "by_symbol": {k: round(v) for k, v in sorted(fno_by_symbol.items(),
                                                           key=lambda x: -abs(x[1]))},
        }
        print(f"\nF&O by symbol (unrealized):")
        for sym, inc in sorted(fno_by_symbol.items(), key=lambda x: -abs(x[1])):
            print(f"  {sym}: ₹{inc/100000:.1f}L")
    else:
        print(f"No F&O data found at {fno_csv}, skipping F&O attribution")
        for sym_lots in lots_by_symbol.values():
            for lot in sym_lots:
                lot.setdefault("option_income", 0)

    # Aggregate to stock level
    pass_list = []
    fail_list = []
    too_early_list = []
    no_test_list = []

    total_invested = 0
    total_current = 0
    by_class = {"stock": {"invested": 0, "current": 0, "count": 0},
                "gold_etf": {"invested": 0, "current": 0, "count": 0},
                "active_mf": {"invested": 0, "current": 0, "count": 0},
                "index_mf": {"invested": 0, "current": 0, "count": 0}}

    for sym in sorted(lots_by_symbol.keys()):
        lots = lots_by_symbol[sym]
        if not lots:
            continue

        asset_class, hurdle = CLASSIFICATIONS.get(sym, ("stock", 3))
        name = DISPLAY_NAMES.get(sym, sym)

        sym_invested = sum(l["invested"] for l in lots)
        sym_current = sum(l["current_value"] for l in lots)
        sym_nifty = sum(l["shadow_nifty"] for l in lots)

        total_invested += sym_invested
        total_current += sym_current
        by_class[asset_class]["invested"] += sym_invested
        by_class[asset_class]["current"] += sym_current
        by_class[asset_class]["count"] += 1

        # Separate LTCG lots from too-early lots
        ltcg_lots = [l for l in lots if not l["too_early"]]
        early_lots = [l for l in lots if l["too_early"]]

        # No test for index MF
        if asset_class == "index_mf":
            no_test_list.append({
                "name": name,
                "symbol": sym,
                "invested": sym_invested,
                "current": sym_current,
                "lots": lots,
            })
            continue

        # If ALL lots are too early
        if not ltcg_lots:
            too_early_list.append({
                "name": name,
                "symbol": sym,
                "invested": sym_invested,
                "current": sym_current,
                "gain_loss": sym_current - sym_invested,
                "lots": lots,
            })
            continue

        # Compute aggregate for LTCG lots only
        ltcg_invested = sum(l["invested"] for l in ltcg_lots)
        ltcg_current = sum(l["current_value"] for l in ltcg_lots)
        ltcg_nifty = sum(l["shadow_nifty"] for l in ltcg_lots)

        # Weighted CAGR
        total_weight = sum(l["invested"] for l in ltcg_lots)
        if total_weight > 0:
            weighted_stock_cagr = sum(l["stock_cagr"] * l["invested"] for l in ltcg_lots) / total_weight
            weighted_nifty_cagr = sum(l["nifty_cagr"] * l["invested"] for l in ltcg_lots) / total_weight
        else:
            weighted_stock_cagr = 0
            weighted_nifty_cagr = 0

        hurdle_rate = weighted_nifty_cagr + hurdle
        ltcg_option_income = sum(l.get("option_income", 0) for l in ltcg_lots)

        # For hurdle test: compare stock return (current + F&O income) to nifty hurdle
        # nifty_shadow_at_hurdle for each lot = invested * (1 + nifty_cagr + hurdle%)^years
        hurdle_nifty = 0
        for l in ltcg_lots:
            hurdle_nifty += l["invested"] * ((1 + (l["nifty_cagr"] + hurdle) / 100) ** l["years_held"])
        hurdle_surplus = (ltcg_current + ltcg_option_income) - round(hurdle_nifty)

        stock_entry = {
            "name": name,
            "symbol": sym,
            "cagr": round(weighted_stock_cagr, 1),
            "niftyCagr": round(weighted_nifty_cagr, 1),
            "hurdle": round(weighted_nifty_cagr + hurdle, 1),
            "hurdlePct": hurdle,
            "invested": ltcg_invested,
            "current": ltcg_current,
            "nifty": ltcg_nifty,
            "option_income": ltcg_option_income,
            "lots": lots,
            "deep_dive": sym == "HINDUNILVR" and asset_class == "stock",
            "asset_class": asset_class,
        }

        if hurdle_surplus >= 0:
            stock_entry["surplus"] = hurdle_surplus
            pass_list.append(stock_entry)
        else:
            stock_entry["deficit"] = abs(hurdle_surplus)
            fail_list.append(stock_entry)

    # Sort: fail by deficit desc, pass by surplus desc
    fail_list.sort(key=lambda x: x.get("deficit", 0), reverse=True)
    pass_list.sort(key=lambda x: x.get("surplus", 0), reverse=True)

    # Build deep-dive data for failed stocks
    build_deep_dive_data(fail_list, lots_by_symbol, prices, tri, stock_tri_data)

    total_surplus = sum(s.get("surplus", 0) for s in pass_list)
    total_deficit = sum(s.get("deficit", 0) for s in fail_list)

    cockpit = {
        "report_date": REPORT_DATE,
        "portfolio": {
            "total_invested": total_invested,
            "total_current": total_current,
            "by_class": by_class,
        },
        "stocks": {
            "pass": pass_list,
            "fail": fail_list,
            "tooEarly": too_early_list,
            "noTest": no_test_list,
        },
        "summary": {
            "pass_count": len(pass_list),
            "fail_count": len(fail_list),
            "too_early_count": len(too_early_list),
            "no_test_count": len(no_test_list),
            "total_surplus": total_surplus,
            "total_deficit": total_deficit,
        },
        "fno_summary": fno_summary,
    }

    # Write
    out_path = os.path.join(PROJECT, "ui-cockpit", "public", "cockpit.json")
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w") as f:
        json.dump(cockpit, f, indent=2)
    print(f"\nWrote {out_path}")

    # Print summary
    print(f"\nSummary:")
    print(f"  Total invested: ₹{total_invested/10000000:.2f} Cr")
    print(f"  Total current:  ₹{total_current/10000000:.2f} Cr")
    print(f"  Pass: {len(pass_list)} (surplus ₹{total_surplus/100000:.1f}L)")
    print(f"  Fail: {len(fail_list)} (deficit ₹{total_deficit/100000:.1f}L)")
    print(f"  Too early: {len(too_early_list)}")
    print(f"  No test: {len(no_test_list)}")

    return cockpit


def build_hul_data():
    """Build hulData.js from CSV files."""
    print("\n=== Building hulData.js ===")

    # HUL TRI
    hul_tri = load_csv(os.path.join(DOWNLOADS, "HUL_TRI_Indexed.csv"))
    # Nifty TRI
    nifty_tri = load_csv(os.path.join(DOWNLOADS, "NIFTY500_TRI_Indexed.csv"))
    # HUL prices
    hul_prices = load_csv(os.path.join(DOWNLOADS, "HUL_daily_prices.csv"))
    # HUL dividends
    hul_divs = load_csv(os.path.join(DOWNLOADS, "HUL_dividends_verified (1).csv"))
    # Expiry dates
    expiries = load_csv(os.path.join(DOWNLOADS, "NSE_Monthly_Expiry_Dates.csv"))

    lines = []
    lines.append("// Auto-generated by scripts/build_cockpit_data.py")
    lines.append("// Do not edit manually.\n")

    # HUL TRI
    lines.append("export const HUL_TRI = [")
    for row in hul_tri:
        val = row["TRI_Indexed"].strip()
        if val:
            lines.append(f'  {{ date: "{row["Date"]}", tri: {float(val):.6f} }},')
    lines.append("];\n")

    # Nifty TRI
    lines.append("export const NIFTY_TRI = [")
    for row in nifty_tri:
        val = row["TRI_Indexed"].strip()
        if val:
            lines.append(f'  {{ date: "{row["Date"]}", tri: {float(val):.6f} }},')
    lines.append("];\n")

    # HUL prices
    lines.append("export const HUL_PRICES = [")
    for row in hul_prices:
        val = row["Close_INR"].strip()
        if val:
            lines.append(f'  {{ date: "{row["Date"]}", close: {float(val):.2f} }},')
    lines.append("];\n")

    # HUL dividends
    lines.append("export const HUL_DIVIDENDS = [")
    for row in hul_divs:
        lines.append(f'  {{ exDate: "{row["Ex_Date"]}", type: "{row["Type"]}", dps: {float(row["DPS_INR"]):.1f}, total: {float(row["Total_Dividend_INR"]):.0f} }},')
    lines.append("];\n")

    # Expiry dates
    lines.append("export const EXPIRY_DATES = [")
    for row in expiries:
        lines.append(f'  {{ month: "{row["Month"]}", date: "{row["Expiry_Date"]}" }},')
    lines.append("];\n")

    out_path = os.path.join(PROJECT, "ui-cockpit", "src", "data", "hulData.js")
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w") as f:
        f.write("\n".join(lines))
    print(f"Wrote {out_path} ({len(lines)} lines)")


if __name__ == "__main__":
    build_cockpit_json()
    build_hul_data()
    print("\nDone!")
