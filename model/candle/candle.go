package candle

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
