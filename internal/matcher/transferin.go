package matcher

import (
	_ "embed"
	"encoding/json"
	"time"

	"stock-scorecard/internal/dividend"
	"stock-scorecard/internal/tradebook"
	"stock-scorecard/internal/tri"
)

//go:embed nifty500_prices_20160224.json
var nifty500PricesJSON []byte

// nifty500Prices maps NSE symbol → closing price on 2016-02-24 (first TRI date).
var nifty500Prices map[string]float64

func init() {
	nifty500Prices = make(map[string]float64)
	json.Unmarshal(nifty500PricesJSON, &nifty500Prices)
}

// firstTRIDate is the earliest date in the TRI index (2016-02-24).
var firstTRIDate = time.Date(2016, 2, 24, 0, 0, 0, 0, time.UTC)

// DetectTransferIns identifies unmatched sells (pre-history holdings or
// transfer-ins) and creates realized trades using the first TRI date as
// a synthetic buy date.
//
// Every warning from FIFO represents shares that were sold but had no
// matching buy — either the stock was bought before tradebook history
// starts, or was partially held from before. In all cases, if we have
// a known price for that symbol, we create a Tier 2 realized trade.
//
// For symbols without a known price, the warning is kept for manual
// resolution via the review CSV.
func DetectTransferIns(
	warnings []Warning,
	trades []tradebook.ConsolidatedTrade,
	triIdx *tri.TRIIndex,
	divIdx *dividend.DividendIndex,
) (realized []RealizedTrade, remaining []Warning) {
	for _, w := range warnings {
		sellDate, _ := time.Parse("2006-01-02", w.SellDate)

		// Look up price from static map
		price, hasPrice := nifty500Prices[w.Symbol]
		if !hasPrice {
			// No price available — keep as warning for manual resolution
			remaining = append(remaining, w)
			continue
		}

		// Create a realized trade with the first TRI date as buy date
		rt, err := buildRealizedTrade(
			w.Symbol, w.ISIN,
			firstTRIDate, price,
			sellDate, w.SellPrice,
			w.Unmatched, triIdx, divIdx,
			TierIntelligent, "transfer_in",
		)
		if err != nil {
			// TRI lookup failed — keep as warning
			remaining = append(remaining, w)
			continue
		}
		realized = append(realized, rt)
	}

	return realized, remaining
}
