package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

const wsURL = "wss://stream.bybit.com/v5/public/linear"

// pingInterval is how often we send a heartbeat to keep the connection alive.
const pingInterval = 20 * time.Second

// token implements adapter.Token for a single Bybit kline subscription.
type token struct {
	cancel context.CancelFunc
}

func (t *token) Unsubscribe() { t.cancel() }

// subscribeKline opens a Bybit WebSocket kline stream for category/symbol/interval,
// invoking handler for every update. It reconnects automatically on error.
func subscribeKline(ctx context.Context, category, symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		backoff := time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			if err := connectAndRead(ctx, category, symbol, interval, handler); err != nil && ctx.Err() == nil {
				log.Printf("bybit ws [%s/%s]: %v — reconnecting in %v", symbol, interval, err, backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
			} else {
				backoff = time.Second
			}
		}
	}()

	return &token{cancel: cancel}, nil
}

// connectAndRead maintains a single Bybit WebSocket session.
func connectAndRead(ctx context.Context, category, symbol, interval string, handler adapter.CandleHandler) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Close connection on context cancellation.
	go func() {
		<-ctx.Done()
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}()

	// Send subscribe message.
	topic := fmt.Sprintf("kline.%s.%s", interval, symbol)
	subMsg := map[string]any{
		"op":   "subscribe",
		"args": []string{topic},
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	// Heartbeat: Bybit requires a ping every 20 s or it closes the connection.
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteJSON(map[string]string{"op": "ping"}); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		candles, err := parseWsMessage(symbol, msg)
		if err != nil {
			log.Printf("bybit ws [%s/%s]: parse error: %v", symbol, interval, err)
			continue
		}
		for _, c := range candles {
			handler(c)
		}
	}
}

// bybitWsMsg is the generic Bybit V5 WebSocket message envelope.
type bybitWsMsg struct {
	Op      string          `json:"op"`      // "pong", "subscribe"
	Success bool            `json:"success"` // subscription ack
	Topic   string          `json:"topic"`   // "kline.1.BTCUSDT"
	Type    string          `json:"type"`    // "snapshot" | "delta"
	Data    json.RawMessage `json:"data"`
}

// bybitKlineEntry is one kline object inside the data array.
type bybitKlineEntry struct {
	Start    int64  `json:"start"`    // open time (ms)
	End      int64  `json:"end"`      // close time (ms)
	Interval string `json:"interval"` // "1", "60", "D", …
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
	Confirm  bool   `json:"confirm"` // true = candle is closed
}

func parseWsMessage(symbol string, msg []byte) ([]*candle.Candle, error) {
	var m bybitWsMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}

	// Ignore control messages (pong, subscribe ack).
	if m.Topic == "" {
		return nil, nil
	}

	var entries []bybitKlineEntry
	if err := json.Unmarshal(m.Data, &entries); err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}

	out := make([]*candle.Candle, 0, len(entries))
	for _, e := range entries {
		out = append(out, &candle.Candle{
			Exchange:  "bybit",
			Symbol:    symbol,
			Interval:  e.Interval,
			OpenTime:  e.Start,
			Open:      e.Open,
			High:      e.High,
			Low:       e.Low,
			Close:     e.Close,
			Volume:    e.Volume,
			CloseTime: e.End,
			IsClosed:  e.Confirm,
		})
	}
	return out, nil
}
