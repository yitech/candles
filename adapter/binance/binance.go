package binance

import (
	"context"
	"net/http"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the Binance exchange adapter.
type Adapter struct {
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

func New() *Adapter {
	ctx, cancel := context.WithCancel(context.Background())
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Subscribe opens a WebSocket kline stream for symbol/interval.
// The returned Token cancels this specific subscription.
func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return subscribeKline(a.ctx, symbol, interval, handler)
}

// Backfill fetches historical klines via the Binance REST API.
func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return fetchKlines(a.ctx, a.httpClient, symbol, interval, start.UnixMilli(), end.UnixMilli())
}

// Close cancels all active subscriptions and releases resources.
func (a *Adapter) Close() error {
	a.cancel()
	return nil
}
