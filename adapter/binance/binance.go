package binance

import (
	"fmt"

	"github.com/yitech/candles/adapter"
)

// Adapter is the Binance exchange adapter.
// TODO: implement WebSocket connection to wss://stream.binance.com/ws
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string) (<-chan *adapter.Candle, error) {
	return nil, fmt.Errorf("binance: not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
