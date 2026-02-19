package binance

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
	baseURL   = "https://api.binance.com"
	klinePath = "/api/v3/klines"
	maxLimit  = 1000
)

// fetchKlines requests historical klines from the Binance REST API,
// paginating automatically until the full [startMs, endMs] range is covered.
func fetchKlines(ctx context.Context, client *http.Client, symbol, interval string, startMs, endMs int64) ([]*candle.Candle, error) {
	var out []*candle.Candle

	for {
		batch, err := fetchBatch(ctx, client, symbol, interval, startMs, endMs)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)

		// Fewer than maxLimit means we've reached the end of the range.
		if len(batch) < maxLimit {
			break
		}

		// Advance start to just after the last candle's open time.
		startMs = batch[len(batch)-1].OpenTime + 1
		if startMs > endMs {
			break
		}
	}

	return out, nil
}

// fetchBatch fetches a single page (up to maxLimit candles) from the API.
func fetchBatch(ctx context.Context, client *http.Client, symbol, interval string, startMs, endMs int64) ([]*candle.Candle, error) {
	u, err := url.Parse(baseURL + klinePath)
	if err != nil {
		return nil, fmt.Errorf("binance: parse url: %w", err)
	}

	q := u.Query()
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("startTime", strconv.FormatInt(startMs, 10))
	q.Set("endTime", strconv.FormatInt(endMs, 10))
	q.Set("limit", strconv.Itoa(maxLimit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("binance: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("binance: http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance: unexpected status %s", resp.Status)
	}

	// Each kline is a JSON array. Binance returns [][]json.RawMessage.
	var raw [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("binance: decode response: %w", err)
	}

	return parseKlines(symbol, interval, raw)
}

// parseKlines converts the raw Binance wire format into candle.Candle values.
//
// Binance kline array layout:
//
//	[0]  Open time       (int64, Unix ms)
//	[1]  Open            (string)
//	[2]  High            (string)
//	[3]  Low             (string)
//	[4]  Close           (string)
//	[5]  Volume          (string, base asset)
//	[6]  Close time      (int64, Unix ms)
//	[7]  Quote volume    (string)  — unused
//	[8]  Trade count     (int64)   — unused
//	[9]  Taker buy base  (string)  — unused
//	[10] Taker buy quote (string)  — unused
//	[11] Ignore          (string)
func parseKlines(symbol, interval string, raw [][]json.RawMessage) ([]*candle.Candle, error) {
	out := make([]*candle.Candle, 0, len(raw))
	for i, r := range raw {
		if len(r) < 7 {
			return nil, fmt.Errorf("binance: kline[%d] has %d fields, want ≥7", i, len(r))
		}

		openTime, err := parseInt64(r[0])
		if err != nil {
			return nil, fmt.Errorf("binance: kline[%d] open_time: %w", i, err)
		}
		closeTime, err := parseInt64(r[6])
		if err != nil {
			return nil, fmt.Errorf("binance: kline[%d] close_time: %w", i, err)
		}

		out = append(out, &candle.Candle{
			Exchange:  "binance",
			Symbol:    symbol,
			Interval:  interval,
			OpenTime:  openTime,
			Open:      jsonString(r[1]),
			High:      jsonString(r[2]),
			Low:       jsonString(r[3]),
			Close:     jsonString(r[4]),
			Volume:    jsonString(r[5]),
			CloseTime: closeTime,
			IsClosed:  true, // historical candles are always closed
		})
	}
	return out, nil
}

// parseInt64 unmarshals a JSON number into an int64.
func parseInt64(raw json.RawMessage) (int64, error) {
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// jsonString strips surrounding quotes from a JSON string token.
func jsonString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		// Fallback: return the raw token as-is.
		return string(raw)
	}
	return s
}
