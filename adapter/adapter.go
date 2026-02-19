package adapter

import (
	"time"

	"github.com/yitech/candles/model/candle"
)

// CandleHandler is invoked for each incoming live candle update.
type CandleHandler func(*candle.Candle)

// Token represents an active subscription.
// Call Unsubscribe to stop receiving candle updates for that subscription.
type Token interface {
	Unsubscribe()
}

// Adapter is the contract for exchange market-data connectors.
type Adapter interface {
	// Subscribe registers handler to receive live candle updates for
	// symbol/interval. Returns a Token that cancels the subscription.
	Subscribe(symbol, interval string, handler CandleHandler) (Token, error)

	// Backfill fetches historical candles for symbol/interval in [start, end].
	// Uses the exchange REST API internally.
	Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error)

	// Close shuts down all active subscriptions and releases resources.
	Close() error
}
