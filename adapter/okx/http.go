package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yitech/candles/model/candle"
)

const (
	baseURL   = "https://www.okx.com"
	klinePath = "/api/v5/market/history-candles"
	maxLimit  = 100
)

// fetchKlines requests historical klines from the OKX REST API,
// paginating automatically until the full [startMs, endMs] range is covered.
//
// OKX returns candles newest-first using cursor-based pagination via the
// `after` parameter; this function reverses the result to chronological order.
func fetchKlines(ctx context.Context, client *http.Client, instID, bar string, startMs, endMs int64) ([]*candle.Candle, error) {
	var all []*candle.Candle

	// after=T returns candles with ts < T, so seed with endMs+1 to include endMs.
	after := strconv.FormatInt(endMs+1, 10)

	for {
		batch, err := fetchBatch(ctx, client, instID, bar, after)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		// Collect candles that fall within [startMs, endMs]; stop when we go older.
		done := false
		for _, c := range batch {
			if c.OpenTime < startMs {
				done = true
				break
			}
			all = append(all, c)
		}

		if done || len(batch) < maxLimit {
			break
		}

		// batch is newest-first; oldest openTime is at the end of all collected.
		after = strconv.FormatInt(all[len(all)-1].OpenTime, 10)
	}

	// Reverse to chronological order.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

// fetchBatch fetches a single page from the OKX history-candles endpoint.
func fetchBatch(ctx context.Context, client *http.Client, instID, bar, after string) ([]*candle.Candle, error) {
	u, err := url.Parse(baseURL + klinePath)
	if err != nil {
		return nil, fmt.Errorf("okx: parse url: %w", err)
	}

	q := u.Query()
	q.Set("instId", instID)
	q.Set("bar", bar)
	q.Set("after", after)
	q.Set("limit", strconv.Itoa(maxLimit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("okx: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("okx: http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx: unexpected status %s", resp.Status)
	}

	// OKX envelope
	var envelope struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("okx: decode response: %w", err)
	}
	if envelope.Code != "0" {
		return nil, fmt.Errorf("okx: api error %s: %s", envelope.Code, envelope.Msg)
	}

	return parseKlines(instID, bar, envelope.Data)
}

// parseKlines converts the OKX wire format into candle.Candle values.
//
// OKX kline array layout:
//
//	[0] ts        (open time, ms)
//	[1] o         (open)
//	[2] h         (high)
//	[3] l         (low)
//	[4] c         (close)
//	[5] vol       (base currency volume)
//	[6] volCcy    (quote currency volume)     — unused
//	[7] volCcyQuote                           — unused
//	[8] confirm   ("1"=closed, "0"=current)
func parseKlines(instID, bar string, rows [][]string) ([]*candle.Candle, error) {
	intervalMs := intervalToMs(bar)
	out := make([]*candle.Candle, 0, len(rows))

	for i, r := range rows {
		if len(r) < 6 {
			return nil, fmt.Errorf("okx: kline[%d] has %d fields, want ≥6", i, len(r))
		}

		openTime, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("okx: kline[%d] open_time: %w", i, err)
		}

		isClosed := len(r) > 8 && r[8] == "1"

		out = append(out, &candle.Candle{
			Exchange:  "okx",
			Symbol:    instID,
			Interval:  bar,
			OpenTime:  openTime,
			Open:      r[1],
			High:      r[2],
			Low:       r[3],
			Close:     r[4],
			Volume:    r[5],
			CloseTime: openTime + intervalMs - 1,
			IsClosed:  isClosed,
		})
	}
	return out, nil
}

// intervalToMs converts an OKX bar string to milliseconds.
// OKX uses suffixed notation: 1m, 3m, 1H, 4H, 1D, 1W, 1M, 3M, etc.
func intervalToMs(bar string) int64 {
	const min = 60_000
	if len(bar) < 2 {
		return 0
	}

	unit := bar[len(bar)-1]
	numStr := bar[:len(bar)-1]
	// OKX uses uppercase H/D/W/M for hours/day/week/month; lowercase m for minutes.
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(string(unit)) {
	case "M":
		if unit == 'm' { // lowercase = minutes
			return n * min
		}
		return n * 30 * 24 * 60 * min // uppercase M = months (approximate)
	case "H":
		return n * 60 * min
	case "D":
		return n * 24 * 60 * min
	case "W":
		return n * 7 * 24 * 60 * min
	default:
		return 0
	}
}
