#!/usr/bin/env python3
"""
Fetch NIFTY 500 constituent closing prices on 2016-02-24 (first TRI date).

Usage:
    pip install yfinance
    python scripts/fetch_nifty500_prices.py

Output:
    internal/matcher/nifty500_prices_20160224.json
"""

import json
import sys
from datetime import datetime, timedelta

try:
    import yfinance as yf
except ImportError:
    print("Install yfinance: pip install yfinance", file=sys.stderr)
    sys.exit(1)

# Target date — first date in the TRI index
TARGET_DATE = "2016-02-24"
target = datetime.strptime(TARGET_DATE, "%Y-%m-%d")
# yfinance range: day before to day after to ensure we get the target date
start = (target - timedelta(days=5)).strftime("%Y-%m-%d")
end = (target + timedelta(days=1)).strftime("%Y-%m-%d")

# NIFTY 500 constituents as of 2024 — common symbols that appear in retail portfolios.
# This is a representative subset; add more as needed.
SYMBOLS = [
    "RELIANCE", "TCS", "HDFCBANK", "INFY", "ICICIBANK", "HINDUNILVR",
    "ITC", "SBIN", "BHARTIARTL", "KOTAKBANK", "LT", "HCLTECH",
    "AXISBANK", "ASIANPAINT", "MARUTI", "SUNPHARMA", "TITAN",
    "BAJFINANCE", "DMART", "NESTLEIND", "WIPRO", "ULTRACEMCO",
    "NTPC", "POWERGRID", "ONGC", "TATAMOTORS", "JSWSTEEL",
    "TATASTEEL", "ADANIENT", "ADANIPORTS", "TECHM", "DIVISLAB",
    "DRREDDY", "BAJAJFINSV", "BRITANNIA", "CIPLA", "EICHERMOT",
    "GRASIM", "HEROMOTOCO", "HINDALCO", "INDUSINDBK", "M&M",
    "SBILIFE", "HDFCLIFE", "BPCL", "COALINDIA", "GAIL",
    "IOC", "PIDILITIND", "HAVELLS", "DABUR", "GODREJCP",
    "MARICO", "COLPAL", "BERGEPAINT", "MCDOWELL-N", "TRENT",
    "APOLLOHOSP", "FORTIS", "MAXHEALTH", "LALPATHLAB",
    "METROPOLIS", "BIOCON", "AUROPHARMA", "LUPIN", "TORNTPHARM",
    "ALKEM", "IPCALAB", "LAURUSLABS", "NATCOPHARM", "ABBOTINDIA",
    "PFIZER", "GLAXO", "SANOFI", "MPHASIS", "LTIM",
    "PERSISTENT", "COFORGE", "LTTS", "TATAELXSI", "HAPPSTMNDS",
    "ZOMATO", "NYKAA", "PAYTM", "POLICYBZR", "DELHIVERY",
    "IRCTC", "INDIGO", "TATAPOWER", "ADANIGREEN", "ADANITRANS",
    "VEDL", "NMDC", "HINDPETRO", "BANKBARODA", "PNB",
    "CANBK", "IDFCFIRSTB", "FEDERALBNK", "BANDHANBNK", "RBLBANK",
    "MANAPPURAM", "MUTHOOTFIN", "BAJAJ-AUTO", "TVS", "ASHOKLEY",
    "MRF", "BALKRISIND", "APOLLOTYRE", "CEAT", "EXIDEIND",
    "AMBUJACEM", "ACC", "SHREECEM", "DALMIACEM", "RAMCOCEM",
    "JKCEMENT", "STARCEM", "PIIND", "ATUL", "SRF",
    "DEEPAKNTR", "AARTI", "CLEAN", "FLUOROCHEM", "NAVINFLUOR",
    "TATACOMM", "IDEA", "MTNL", "CONCOR", "CESC",
    "TATACONSUM", "VBL", "JUBLFOOD", "DEVYANI", "SAPPHIRE",
    "WHIRLPOOL", "VOLTAS", "BLUESTARLT", "CROMPTON", "ORIENTELEC",
    "DIXON", "KAYNES", "AFFLE", "ROUTE", "TANLA",
    "SYNGENE", "NAUKRI", "JIOFIN", "HAL", "BEL",
    "BHEL", "SAIL", "RECLTD", "PFC", "IRFC",
    "NHPC", "SJVN", "CESC", "TORNTPOWER", "JSL",
    "JINDALSTEL", "NATIONALUM", "HINDZINC", "SIEMENS", "ABB",
    "HONAUT", "PAGEIND", "RELAXO", "BATA", "CAMPUS",
    "TATACOMM", "STARHEALTH", "NIACL", "GICRE", "ICICIGI",
    "ICICIPRULI", "SBICARD", "CHOLAFIN", "SHRIRAMFIN", "L&TFH",
    "M&MFIN", "POONAWALLA", "CANFINHOME", "LICHSGFIN", "AAVAS",
    "OBEROIRLTY", "DLF", "GODREJPROP", "PRESTIGE", "BRIGADE",
    "PHOENIXLTD", "SOBHA", "SUNTV", "PVR", "NETWORK18",
    "TV18BRDCST", "ZEEL", "MOTHERSON", "BOSCHLTD", "SUNDRMFAST",
    "SCHAEFFLER", "CUMMINSIND", "THERMAX", "AIAENG", "GRINDWELL",
    "CARBORUNIV", "ELGIEQUIP", "ISGEC", "KSB",
]


def fetch_prices():
    prices = {}
    batch_size = 50
    total = len(SYMBOLS)

    for i in range(0, total, batch_size):
        batch = SYMBOLS[i : i + batch_size]
        tickers_str = " ".join(f"{s}.NS" for s in batch)
        print(
            f"Fetching batch {i // batch_size + 1}/{(total + batch_size - 1) // batch_size}...",
            file=sys.stderr,
        )

        data = yf.download(tickers_str, start=start, end=end, progress=False)
        if data.empty:
            continue

        close = data.get("Close")
        if close is None:
            continue

        # Find the row closest to target date
        for s in batch:
            ticker = f"{s}.NS"
            try:
                if hasattr(close, "columns"):
                    if ticker in close.columns:
                        val = close[ticker].dropna()
                    else:
                        continue
                else:
                    val = close.dropna()

                if len(val) > 0:
                    price = float(val.iloc[-1])
                    if price > 0:
                        prices[s] = round(price, 2)
            except Exception as e:
                print(f"  Skip {s}: {e}", file=sys.stderr)

    return prices


if __name__ == "__main__":
    prices = fetch_prices()
    out_path = "internal/matcher/nifty500_prices_20160224.json"
    with open(out_path, "w") as f:
        json.dump(prices, f, indent=2, sort_keys=True)
    print(f"Wrote {len(prices)} prices to {out_path}", file=sys.stderr)
