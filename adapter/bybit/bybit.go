package bybit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the Bybit exchange adapter.
// TODO: implement WebSocket subscription (wss://stream.bybit.com/v5/public/linear)
type Adapter struct {
	httpClient *http.Client
	category   string // "linear" | "spot" | "inverse"
}

func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		category:   "linear",
	}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("bybit: Subscribe not implemented")
}

// Backfill fetches historical klines via the Bybit REST API for
// symbol/interval in the range [start, end].
func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return fetchKlines(context.Background(), a.httpClient, a.category, symbol, interval, start.UnixMilli(), end.UnixMilli())
}

func (a *Adapter) Close() error {
	return nil
}
