package binance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the Binance exchange adapter.
// TODO: implement WebSocket subscription (wss://stream.binance.com/ws)
type Adapter struct {
	httpClient *http.Client
}

func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("binance: Subscribe not implemented")
}

// Backfill fetches historical klines via the Binance REST API for
// symbol/interval in the half-open range [start, end].
func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return fetchKlines(context.Background(), a.httpClient, symbol, interval, start.UnixMilli(), end.UnixMilli())
}

func (a *Adapter) Close() error {
	return nil
}
