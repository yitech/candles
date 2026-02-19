package adapter

// Candle is the adapter-layer representation of an OHLCV candlestick.
// The server is responsible for converting this into the protobuf type.
type Candle struct {
	Exchange  string
	Symbol    string
	Interval  string
	OpenTime  int64
	Open      string
	High      string
	Low       string
	Close     string
	Volume    string
	CloseTime int64
	IsClosed  bool
}

// Adapter defines the contract for exchange market-data adapters.
// Each implementation connects to an exchange WebSocket and emits
// Candles on the returned channel.
type Adapter interface {
	// Subscribe starts streaming candles for the given symbol and interval.
	// The returned channel is closed when the adapter stops or Close is called.
	Subscribe(symbol, interval string) (<-chan *Candle, error)

	// Close shuts down the adapter and releases all resources.
	Close() error
}
