package binance

import (
	"fmt"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the Binance exchange adapter.
// TODO: implement WebSocket connection to wss://stream.binance.com/ws
// TODO: implement REST client for https://api.binance.com/api/v3/klines
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("binance: Subscribe not implemented")
}

func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return nil, fmt.Errorf("binance: Backfill not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
