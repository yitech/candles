package okx

import (
	"fmt"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// Adapter is the OKX exchange adapter.
// TODO: implement WebSocket connection to wss://ws.okx.com:8443/ws/v5/public
// TODO: implement REST client for https://www.okx.com/api/v5/market/history-candles
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	return nil, fmt.Errorf("okx: Subscribe not implemented")
}

func (a *Adapter) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	return nil, fmt.Errorf("okx: Backfill not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
