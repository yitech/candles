package bybit

import (
	"fmt"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the Bybit exchange adapter.
// TODO: implement WebSocket connection to wss://stream.bybit.com/v5/public/linear
// TODO: implement REST client for https://api.bybit.com/v5/market/kline
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("bybit: Subscribe not implemented")
}

func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return nil, fmt.Errorf("bybit: Backfill not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
