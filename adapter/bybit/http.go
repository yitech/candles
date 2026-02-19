package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/yitech/candles/model/candle"
)

const (
	baseURL   = "https://api.bybit.com"
	klinePath = "/v5/market/kline"
	maxLimit  = 200
)

// fetchKlines requests historical klines from the Bybit REST API,
// paginating automatically until the full [startMs, endMs] range is covered.
//
// Bybit returns candles newest-first; this function reverses the result
// to chronological order before returning.
func fetchKlines(ctx context.Context, client *http.Client, category, symbol, interval string, startMs, endMs int64) ([]*candle.Candle, error) {
	var all []*candle.Candle
	end := endMs

	for {
		batch, err := fetchBatch(ctx, client, category, symbol, interval, startMs, end)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)

		if len(batch) < maxLimit {
			break
		}

		// batch is newest-first, so the oldest openTime is at the end.
		end = all[len(all)-1].OpenTime - 1
		if end < startMs {
			break
		}
	}

	// Reverse to chronological order.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

// fetchBatch fetches a single page from the Bybit kline endpoint.
func fetchBatch(ctx context.Context, client *http.Client, category, symbol, interval string, startMs, endMs int64) ([]*candle.Candle, error) {
	u, err := url.Parse(baseURL + klinePath)
	if err != nil {
		return nil, fmt.Errorf("bybit: parse url: %w", err)
	}

	q := u.Query()
	q.Set("category", category)
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("start", strconv.FormatInt(startMs, 10))
	q.Set("end", strconv.FormatInt(endMs, 10))
	q.Set("limit", strconv.Itoa(maxLimit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("bybit: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bybit: http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bybit: unexpected status %s", resp.Status)
	}

	// Bybit V5 envelope
	var envelope struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List [][]string `json:"list"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("bybit: decode response: %w", err)
	}
	if envelope.RetCode != 0 {
		return nil, fmt.Errorf("bybit: api error %d: %s", envelope.RetCode, envelope.RetMsg)
	}

	return parseKlines(symbol, interval, envelope.Result.List)
}

// parseKlines converts the Bybit wire format into candle.Candle values.
//
// Bybit kline array layout:
//
//	[0] startTime  (ms)
//	[1] openPrice
//	[2] highPrice
//	[3] lowPrice
//	[4] closePrice
//	[5] volume     (base coin)
//	[6] turnover   (quote coin) — unused
func parseKlines(symbol, interval string, rows [][]string) ([]*candle.Candle, error) {
	intervalMs := intervalToMs(interval)
	out := make([]*candle.Candle, 0, len(rows))

	for i, r := range rows {
		if len(r) < 6 {
			return nil, fmt.Errorf("bybit: kline[%d] has %d fields, want ≥6", i, len(r))
		}

		openTime, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bybit: kline[%d] open_time: %w", i, err)
		}

		out = append(out, &candle.Candle{
			Exchange:  "bybit",
			Symbol:    symbol,
			Interval:  interval,
			OpenTime:  openTime,
			Open:      r[1],
			High:      r[2],
			Low:       r[3],
			Close:     r[4],
			Volume:    r[5],
			CloseTime: openTime + intervalMs - 1,
			IsClosed:  true,
		})
	}
	return out, nil
}

// intervalToMs converts a Bybit interval string to milliseconds.
// Bybit uses plain minute numbers for sub-day intervals (e.g. "1", "60"),
// and "D", "W", "M" for day/week/month.
func intervalToMs(interval string) int64 {
	const min = 60_000
	switch interval {
	case "1":
		return min
	case "3":
		return 3 * min
	case "5":
		return 5 * min
	case "15":
		return 15 * min
	case "30":
		return 30 * min
	case "60":
		return 60 * min
	case "120":
		return 2 * 60 * min
	case "240":
		return 4 * 60 * min
	case "360":
		return 6 * 60 * min
	case "720":
		return 12 * 60 * min
	case "D":
		return 24 * 60 * min
	case "W":
		return 7 * 24 * 60 * min
	case "M":
		return 30 * 24 * 60 * min // approximate
	default:
		return 0
	}
}
