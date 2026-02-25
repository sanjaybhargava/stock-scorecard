#!/usr/bin/env python3
"""Pull split-adjusted dividend history for tickers in scorecard.json.

Usage:
    python scripts/pull_dividends.py --scorecard ./scorecard.json --output ./dividends.csv

Requires: pip install yfinance>=1.0 pandas
Note: yfinance >=1.0 returns dividends already split-adjusted.
"""

import argparse
import csv
import json
import sys
from typing import List

import yfinance as yf


def extract_tickers(scorecard_path):
    # type: (str) -> List[str]
    """Extract unique tickers from scorecard.json trades and open_positions."""
    with open(scorecard_path) as f:
        data = json.load(f)

    tickers = set()
    for trade in data.get("trades", []):
        tickers.add(trade["ticker"])
    for pos in data.get("open_positions", []):
        tickers.add(pos["ticker"])

    return sorted(tickers)


def get_dividends(ticker_obj, symbol):
    # type: (yf.Ticker, str) -> List[dict]
    """Get split-adjusted dividends from yfinance (>=1.0 returns them pre-adjusted)."""
    dividends = ticker_obj.dividends

    if dividends is None or len(dividends) == 0:
        return []

    results = []
    for date, amount in dividends.items():
        # Normalize date to tz-naive string
        if hasattr(date, 'tz_localize') and date.tzinfo is not None:
            date = date.tz_localize(None)
        date_str = date.strftime("%Y-%m-%d")

        results.append({
            "symbol": symbol,
            "ex_date": date_str,
            "amount": round(float(amount), 4),
        })

    return results


def main():
    parser = argparse.ArgumentParser(description="Pull split-adjusted dividends from yfinance")
    parser.add_argument("--scorecard", required=True, help="Path to scorecard.json")
    parser.add_argument("--output", required=True, help="Path for output dividends.csv")
    args = parser.parse_args()

    tickers = extract_tickers(args.scorecard)
    print("Found {} unique tickers in {}".format(len(tickers), args.scorecard), file=sys.stderr)

    all_dividends = []
    failures = []

    for symbol in tickers:
        yf_symbol = "{}.NS".format(symbol)
        try:
            ticker_obj = yf.Ticker(yf_symbol)
            divs = get_dividends(ticker_obj, symbol)
            if divs:
                all_dividends.extend(divs)
                print("  {}: {} dividends".format(symbol, len(divs)), file=sys.stderr)
            else:
                print("  {}: 0 dividends".format(symbol), file=sys.stderr)
        except Exception as e:
            print("  {}: FAILED ({})".format(symbol, e), file=sys.stderr)
            failures.append(symbol)

    # Write CSV
    with open(args.output, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=["symbol", "ex_date", "amount"])
        writer.writeheader()
        for row in all_dividends:
            writer.writerow(row)

    print("\nSummary:", file=sys.stderr)
    print("  Tickers processed: {}".format(len(tickers)), file=sys.stderr)
    print("  Total dividend events: {}".format(len(all_dividends)), file=sys.stderr)
    print("  Failures: {}".format(len(failures)), file=sys.stderr)
    if failures:
        print("  Failed tickers: {}".format(', '.join(failures)), file=sys.stderr)
    print("  Output: {}".format(args.output), file=sys.stderr)


if __name__ == "__main__":
    main()
