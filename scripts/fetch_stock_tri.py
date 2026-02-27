#!/usr/bin/env python3
"""
Fetch stock TRI (Total Return Index) for all portfolio tickers using Yahoo Finance Adj Close.

Adj Close adjusts for dividends + splits, so indexing to 100 gives a TRI proxy
directly comparable to Nifty 500 TRI.

Usage:
    pip install yfinance pandas
    python3 scripts/fetch_stock_tri.py

Output:
    scripts/stock_tri.json — {ticker: {date_str: tri_value, ...}, ...}
"""

import json
import os
import sys
import time

try:
    import yfinance as yf
    import pandas as pd
except ImportError:
    print("Install dependencies: pip install yfinance pandas", file=sys.stderr)
    sys.exit(1)

PROJECT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUTPUT_PATH = os.path.join(PROJECT, "scripts", "stock_tri.json")

START_DATE = "2016-01-01"
END_DATE = "2026-02-24"  # day after report date to include report date

# Yahoo Finance ticker overrides (same as build_cockpit_data.py)
TICKER_MAP = {
    "MOMENTUM50": "0P0001L1GH.BO",
    "MINDTREE": "LTIM.NS",
    "MONIFTY500": "0P0001NJBI.BO",
    "GOLDBEES": "GOLDBEES.NS",
    "HNDFDS": "HNDFDS.NS",
    "NAUKRI": "NAUKRI.NS",
}

# ETFs/MFs where Adj Close doesn't reflect meaningful dividends or isn't available.
# Use price-only for these (skip TRI fetch).
PRICE_ONLY = {"GOLDBEES", "MONIFTY500", "MOMENTUM50"}

# All portfolio tickers (from unrealized_ZY7393.csv + HUL manual lot)
PORTFOLIO_TICKERS = [
    "AFFLE", "APOLLOHOSP", "BHARTIARTL", "BRITANNIA", "CIPLA",
    "COALINDIA", "DIXON", "GOLDBEES", "HAL", "HAVELLS",
    "HINDUNILVR", "HNDFDS", "IDFCFIRSTB", "IGL", "INDIGO",
    "KFINTECH", "MINDTREE", "MOMENTUM50", "MONIFTY500", "MUTHOOTFIN",
    "NAUKRI", "NTPC", "PIDILITIND", "PRAJIND", "RBLBANK",
    "RELIANCE", "SIEMENS", "TATASTEEL", "THERMAX", "TORNTPHARM",
    "VAIBHAVGBL",
]

MIN_CACHED_DAYS = 100  # skip re-fetch if cache has enough data


def load_cache():
    """Load existing stock_tri.json if it exists."""
    if os.path.exists(OUTPUT_PATH):
        with open(OUTPUT_PATH) as f:
            return json.load(f)
    return {}


def save_cache(data):
    """Save stock_tri.json."""
    with open(OUTPUT_PATH, "w") as f:
        json.dump(data, f, indent=2)


def fetch_stock_tri(symbol):
    """Fetch Adj Close for a symbol, index to 100. Returns {date_str: tri_value}."""
    ticker = TICKER_MAP.get(symbol, f"{symbol}.NS")

    df = yf.download(ticker, start=START_DATE, end=END_DATE, auto_adjust=False, progress=False)

    if df.empty:
        return None

    # yfinance 1.2+ always returns multi-level columns: (Price, Ticker)
    # Extract Adj Close series regardless of column format
    if isinstance(df.columns, pd.MultiIndex):
        adj = df[("Adj Close", ticker)].dropna()
    elif "Adj Close" in df.columns:
        adj = df["Adj Close"].dropna()
    else:
        return None

    if len(adj) < 2:
        return None

    # Index to 100 from first available date
    first_val = float(adj.iloc[0])
    if first_val <= 0:
        return None

    tri = (adj / first_val) * 100.0

    # Convert to {date_str: value} dict
    result = {}
    for date_idx in tri.index:
        date_str = pd.Timestamp(date_idx).strftime("%Y-%m-%d")
        result[date_str] = round(float(tri.loc[date_idx]), 4)

    return result


def main():
    print("=== Fetching stock TRI (Adj Close indexed to 100) ===\n")

    cache = load_cache()
    fetched = 0
    skipped = 0
    failed = 0

    tickers_to_fetch = [t for t in PORTFOLIO_TICKERS if t not in PRICE_ONLY]

    for symbol in sorted(tickers_to_fetch):
        # Check cache
        if symbol in cache and len(cache[symbol]) >= MIN_CACHED_DAYS:
            dates = sorted(cache[symbol].keys())
            print(f"  {symbol:15s}  {len(cache[symbol]):5d} days  {dates[0]} → {dates[-1]}  (cached)")
            skipped += 1
            continue

        ticker = TICKER_MAP.get(symbol, f"{symbol}.NS")
        print(f"  {symbol:15s}  fetching {ticker}...", end=" ", flush=True)

        try:
            tri_data = fetch_stock_tri(symbol)
        except Exception as e:
            print(f"FAILED: {e}")
            failed += 1
            continue

        if tri_data is None or len(tri_data) < 10:
            print("FAILED: insufficient data")
            failed += 1
            continue

        cache[symbol] = tri_data
        dates = sorted(tri_data.keys())

        # Compute CAGR
        first_val = tri_data[dates[0]]
        last_val = tri_data[dates[-1]]
        years = (pd.Timestamp(dates[-1]) - pd.Timestamp(dates[0])).days / 365.25
        cagr = ((last_val / first_val) ** (1 / years) - 1) * 100 if years > 0 else 0

        print(f"{len(tri_data):5d} days  {dates[0]} → {dates[-1]}  CAGR: {cagr:.1f}%")
        fetched += 1

        time.sleep(0.5)  # rate limit

    # Save
    save_cache(cache)
    print(f"\nSaved {len(cache)} tickers to {OUTPUT_PATH}")
    print(f"  Fetched: {fetched}  Cached: {skipped}  Failed: {failed}")

    # Print summary for dividend-heavy stocks
    print("\n--- Dividend impact check (TRI CAGR vs price-only) ---")
    for sym in ["COALINDIA", "NTPC", "HINDUNILVR", "RELIANCE", "ITC", "BHARTIARTL"]:
        if sym in cache:
            dates = sorted(cache[sym].keys())
            first_val = cache[sym][dates[0]]
            last_val = cache[sym][dates[-1]]
            years = (pd.Timestamp(dates[-1]) - pd.Timestamp(dates[0])).days / 365.25
            cagr = ((last_val / first_val) ** (1 / years) - 1) * 100 if years > 0 else 0
            print(f"  {sym:15s}  TRI CAGR: {cagr:6.1f}%  (includes dividends)")


if __name__ == "__main__":
    main()
