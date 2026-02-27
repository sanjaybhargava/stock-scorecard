package cleandata

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"stock-scorecard/internal/fno"
	"stock-scorecard/internal/matcher"
	"stock-scorecard/internal/tradebook"
)

// ReadTrades reads consolidated equity trades from a clean CSV file.
func ReadTrades(path string) ([]tradebook.ConsolidatedTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var trades []tradebook.ConsolidatedTrade
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 8 {
			continue
		}

		date, err := time.Parse("2006-01-02", row[2])
		if err != nil {
			return nil, fmt.Errorf("parse date %q: %w", row[2], err)
		}
		qty, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[4], err)
		}
		avgPrice, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("parse avg_price %q: %w", row[5], err)
		}
		value, err := strconv.ParseFloat(row[6], 64)
		if err != nil {
			return nil, fmt.Errorf("parse value %q: %w", row[6], err)
		}

		trades = append(trades, tradebook.ConsolidatedTrade{
			Symbol:    row[0],
			ISIN:      row[1],
			Date:      date,
			TradeType: row[3],
			Quantity:  qty,
			AvgPrice:  avgPrice,
			Value:     value,
			OrderID:   row[7],
		})
	}

	return trades, nil
}

// ReadFnOTrades reads consolidated F&O trades from a clean CSV file.
func ReadFnOTrades(path string) ([]fno.FnOTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var trades []fno.FnOTrade
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 11 {
			continue
		}

		expiryDate, err := time.Parse("2006-01-02", row[3])
		if err != nil {
			return nil, fmt.Errorf("parse expiry_date %q: %w", row[3], err)
		}
		tradeDate, err := time.Parse("2006-01-02", row[4])
		if err != nil {
			return nil, fmt.Errorf("parse trade_date %q: %w", row[4], err)
		}
		qty, err := strconv.ParseFloat(row[6], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[6], err)
		}
		price, err := strconv.ParseFloat(row[7], 64)
		if err != nil {
			return nil, fmt.Errorf("parse price %q: %w", row[7], err)
		}
		value, err := strconv.ParseFloat(row[8], 64)
		if err != nil {
			return nil, fmt.Errorf("parse value %q: %w", row[8], err)
		}

		trades = append(trades, fno.FnOTrade{
			RawSymbol:  row[0],
			Underlying: row[1],
			OptionType: row[2],
			ExpiryDate: expiryDate,
			TradeDate:  tradeDate,
			TradeType:  row[5],
			Quantity:   qty,
			Price:      price,
			Value:      value,
			TradeID:    row[9],
			OrderID:    row[10],
		})
	}

	return trades, nil
}

// ReadRealizedTrades reads matched buy-sell pairs from a CSV file.
// Backward compatible: if row has < 19 cols, defaults tier=1.
func ReadRealizedTrades(path string) ([]matcher.RealizedTrade, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // allow variable column count

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var trades []matcher.RealizedTrade
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 17 {
			continue
		}

		buyDate, err := time.Parse("2006-01-02", row[4])
		if err != nil {
			return nil, fmt.Errorf("parse buy_date %q: %w", row[4], err)
		}
		sellDate, err := time.Parse("2006-01-02", row[5])
		if err != nil {
			return nil, fmt.Errorf("parse sell_date %q: %w", row[5], err)
		}
		holdDays, err := strconv.Atoi(row[6])
		if err != nil {
			return nil, fmt.Errorf("parse hold_days %q: %w", row[6], err)
		}
		quantity, err := strconv.ParseFloat(row[7], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[7], err)
		}
		buyPrice, err := strconv.ParseFloat(row[8], 64)
		if err != nil {
			return nil, fmt.Errorf("parse buy_price %q: %w", row[8], err)
		}
		sellPrice, err := strconv.ParseFloat(row[9], 64)
		if err != nil {
			return nil, fmt.Errorf("parse sell_price %q: %w", row[9], err)
		}
		invested, err := strconv.ParseFloat(row[10], 64)
		if err != nil {
			return nil, fmt.Errorf("parse invested %q: %w", row[10], err)
		}
		saleValue, err := strconv.ParseFloat(row[11], 64)
		if err != nil {
			return nil, fmt.Errorf("parse sale_value %q: %w", row[11], err)
		}
		equityGL, err := strconv.ParseFloat(row[12], 64)
		if err != nil {
			return nil, fmt.Errorf("parse equity_gl %q: %w", row[12], err)
		}
		niftyBuy, err := strconv.ParseFloat(row[13], 64)
		if err != nil {
			return nil, fmt.Errorf("parse nifty_buy_tri %q: %w", row[13], err)
		}
		niftySell, err := strconv.ParseFloat(row[14], 64)
		if err != nil {
			return nil, fmt.Errorf("parse nifty_sell_tri %q: %w", row[14], err)
		}
		niftyReturn, err := strconv.ParseFloat(row[15], 64)
		if err != nil {
			return nil, fmt.Errorf("parse nifty_return %q: %w", row[15], err)
		}
		// alpha (row[16]) is computed: equity_gl - nifty_return — skip it

		// Tier columns (backward compatible: default to TierExact)
		tier := matcher.TierExact
		tierReason := ""
		if len(row) >= 19 {
			if t, err := strconv.Atoi(row[17]); err == nil {
				tier = matcher.MatchTier(t)
			}
			tierReason = row[18]
		}

		trades = append(trades, matcher.RealizedTrade{
			Symbol:      row[0],
			ISIN:        row[1],
			FY:          row[2],
			Type:        row[3],
			BuyDate:     buyDate,
			SellDate:    sellDate,
			HoldDays:    holdDays,
			Quantity:    quantity,
			BuyPrice:    buyPrice,
			SellPrice:   sellPrice,
			Invested:    invested,
			SaleValue:   saleValue,
			EquityGL:    equityGL,
			NiftyBuy:    niftyBuy,
			NiftySell:   niftySell,
			NiftyReturn: niftyReturn,
			Tier:        tier,
			TierReason:  tierReason,
		})
	}

	return trades, nil
}

// ReadReviewCSV reads review items from a CSV file, skipping comment lines (# prefix).
func ReadReviewCSV(path string) ([]ReviewItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Filter out comment lines
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		lines = append(lines, line)
	}

	r := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	r.FieldsPerRecord = -1

	// Read header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var items []ReviewItem
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 11 {
			continue
		}

		tier, _ := strconv.Atoi(row[0])
		sellPrice, _ := strconv.ParseFloat(row[5], 64)
		quantity, _ := strconv.ParseFloat(row[6], 64)
		sellValue, _ := strconv.ParseFloat(row[7], 64)
		buyPrice, _ := strconv.ParseFloat(row[9], 64)

		items = append(items, ReviewItem{
			Tier:      tier,
			Status:    row[1],
			Symbol:    row[2],
			ISIN:      row[3],
			SellDate:  row[4],
			SellPrice: sellPrice,
			Quantity:  quantity,
			SellValue: sellValue,
			BuyDate:   row[8],
			BuyPrice:  buyPrice,
			Reason:    row[10],
		})
	}

	return items, nil
}

// ReadOpenPositions reads open positions from a CSV file.
func ReadOpenPositions(path string) ([]matcher.OpenPosition, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// Skip header
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var positions []matcher.OpenPosition
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		if len(row) < 6 {
			continue
		}

		buyDate, err := time.Parse("2006-01-02", row[2])
		if err != nil {
			return nil, fmt.Errorf("parse buy_date %q: %w", row[2], err)
		}
		quantity, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, fmt.Errorf("parse quantity %q: %w", row[3], err)
		}
		buyPrice, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("parse buy_price %q: %w", row[4], err)
		}
		invested, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("parse invested %q: %w", row[5], err)
		}

		positions = append(positions, matcher.OpenPosition{
			Symbol:   row[0],
			ISIN:     row[1],
			BuyDate:  buyDate,
			Quantity: quantity,
			BuyPrice: buyPrice,
			Invested: invested,
		})
	}

	return positions, nil
}
