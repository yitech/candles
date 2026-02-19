package okx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the OKX exchange adapter.
// TODO: implement WebSocket subscription (wss://ws.okx.com:8443/ws/v5/public)
type Adapter struct {
	httpClient *http.Client
}

func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("okx: Subscribe not implemented")
}

// Backfill fetches historical klines via the OKX REST API for
// instID/bar in the range [start, end].
//
// Note: OKX uses hyphenated instrument IDs (e.g. "BTC-USDT") and
// suffixed bar notation (e.g. "1m", "4H", "1D").
func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return fetchKlines(context.Background(), a.httpClient, symbol, interval, start.UnixMilli(), end.UnixMilli())
}

func (a *Adapter) Close() error {
	return nil
}
