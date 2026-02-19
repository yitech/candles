package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

const wsBaseURL = "wss://stream.binance.com:9443/ws"

// token implements adapter.Token for a single Binance kline subscription.
type token struct {
	cancel context.CancelFunc
}

func (t *token) Unsubscribe() { t.cancel() }

// subscribeKline opens a Binance WebSocket kline stream for symbol/interval,
// invoking handler for every update. It reconnects automatically on error.
// Returns a Token to cancel the subscription.
func subscribeKline(ctx context.Context, symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		backoff := time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			if err := connectAndRead(ctx, symbol, interval, handler); err != nil && ctx.Err() == nil {
				log.Printf("binance ws [%s/%s]: %v — reconnecting in %v", symbol, interval, err, backoff)
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

// connectAndRead maintains a single WebSocket session until the context is
// cancelled or an error occurs.
func connectAndRead(ctx context.Context, symbol, interval string, handler adapter.CandleHandler) error {
	streamName := strings.ToLower(symbol) + "@kline_" + interval
	u := wsBaseURL + "/" + streamName

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Close the connection when the context is cancelled.
	go func() {
		<-ctx.Done()
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("read: %w", err)
		}

		c, err := parseWsKline(msg)
		if err != nil {
			log.Printf("binance ws [%s/%s]: parse error: %v", symbol, interval, err)
			continue
		}
		handler(c)
	}
}

// wsKlineMsg is the Binance kline stream message envelope.
type wsKlineMsg struct {
	EventType string `json:"e"`
	Symbol    string `json:"s"`
	Kline     struct {
		OpenTime  int64  `json:"t"`
		CloseTime int64  `json:"T"`
		Interval  string `json:"i"`
		Open      string `json:"o"`
		High      string `json:"h"`
		Low       string `json:"l"`
		Close     string `json:"c"`
		Volume    string `json:"v"`
		IsClosed  bool   `json:"x"`
	} `json:"k"`
}

func parseWsKline(msg []byte) (*candle.Candle, error) {
	var m wsKlineMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}
	if m.EventType != "kline" {
		return nil, fmt.Errorf("unexpected event type: %s", m.EventType)
	}
	k := m.Kline
	return &candle.Candle{
		Exchange:  "binance",
		Symbol:    m.Symbol,
		Interval:  k.Interval,
		OpenTime:  k.OpenTime,
		Open:      k.Open,
		High:      k.High,
		Low:       k.Low,
		Close:     k.Close,
		Volume:    k.Volume,
		CloseTime: k.CloseTime,
		IsClosed:  k.IsClosed,
	}, nil
}
