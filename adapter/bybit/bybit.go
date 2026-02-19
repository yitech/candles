package bybit

import (
	"fmt"

	"github.com/yitech/candles/adapter"
)

// Adapter is the Bybit exchange adapter.
// TODO: implement WebSocket connection to wss://stream.bybit.com/v5/public/linear
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string) (<-chan *adapter.Candle, error) {
	return nil, fmt.Errorf("bybit: not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
