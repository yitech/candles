package okx

import (
	"fmt"

	"github.com/yitech/candles/adapter"
)

// Adapter is the OKX exchange adapter.
// TODO: implement WebSocket connection to wss://ws.okx.com:8443/ws/v5/public
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Subscribe(symbol, interval string) (<-chan *adapter.Candle, error) {
	return nil, fmt.Errorf("okx: not implemented")
}

func (a *Adapter) Close() error {
	return nil
}
