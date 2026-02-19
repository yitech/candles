package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

const wsEndpoint = "wss://ws.okx.com:8443/ws/v5/public"

// token implements adapter.Token for a single OKX kline subscription.
type token struct {
	cancel context.CancelFunc
}

func (t *token) Unsubscribe() { t.cancel() }

// subscribeKline opens an OKX WebSocket candle stream for instID/bar,
// invoking handler for every update. It reconnects automatically on error.
func subscribeKline(ctx context.Context, instID, bar string, handler adapter.CandleHandler) (adapter.Token, error) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		backoff := time.Second
		for {
			if ctx.Err() != nil {
				return
			}
			if err := connectAndRead(ctx, instID, bar, handler); err != nil && ctx.Err() == nil {
				log.Printf("okx ws [%s/%s]: %v — reconnecting in %v", instID, bar, err, backoff)
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

// connectAndRead maintains a single OKX WebSocket session.
func connectAndRead(ctx context.Context, instID, bar string, handler adapter.CandleHandler) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsEndpoint, nil)
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

	// OKX channel name: "candle" + bar (e.g. "candle1m", "candle4H").
	channel := "candle" + bar

	subMsg := map[string]any{
		"op": "subscribe",
		"args": []map[string]string{
			{"channel": channel, "instId": instID},
		},
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		// OKX sends plain text "ping" frames (not WS protocol pings).
		if string(msg) == "ping" {
			if err := conn.WriteMessage(websocket.TextMessage, []byte("pong")); err != nil {
				return fmt.Errorf("pong: %w", err)
			}
			continue
		}

		candles, err := parseWsMessage(instID, bar, msg)
		if err != nil {
			log.Printf("okx ws [%s/%s]: parse error: %v", instID, bar, err)
			continue
		}
		for _, c := range candles {
			handler(c)
		}
	}
}

// okxWsMsg is the generic OKX WebSocket message envelope.
type okxWsMsg struct {
	Event string `json:"event"` // "subscribe", "error"
	Code  string `json:"code"`
	Msg   string `json:"msg"`
	Arg   struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data [][]string `json:"data"`
}

// parseWsMessage converts an OKX WebSocket message into candle.Candle values.
//
// OKX kline data array layout (same as REST):
//
//	[0] ts        (open time, ms)
//	[1] o         (open)
//	[2] h         (high)
//	[3] l         (low)
//	[4] c         (close)
//	[5] vol       (base currency volume)
//	[6] volCcy    — unused
//	[7] volCcyQuote — unused
//	[8] confirm   ("1"=closed, "0"=current)
func parseWsMessage(instID, bar string, msg []byte) ([]*candle.Candle, error) {
	var m okxWsMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}

	// Subscription ack or error — no candle data.
	if m.Event != "" {
		if m.Event == "error" {
			return nil, fmt.Errorf("api error %s: %s", m.Code, m.Msg)
		}
		return nil, nil
	}

	if len(m.Data) == 0 {
		return nil, nil
	}

	intervalMs := intervalToMs(bar)
	out := make([]*candle.Candle, 0, len(m.Data))

	for i, r := range m.Data {
		if len(r) < 6 {
			return nil, fmt.Errorf("kline[%d] has %d fields, want ≥6", i, len(r))
		}

		openTime, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("kline[%d] open_time: %w", i, err)
		}

		isClosed := len(r) > 8 && r[8] == "1"

		out = append(out, &candle.Candle{
			Exchange:  "okx",
			Symbol:    instID,
			Interval:  bar,
			OpenTime:  openTime,
			Open:      r[1],
			High:      r[2],
			Low:       r[3],
			Close:     r[4],
			Volume:    r[5],
			CloseTime: openTime + intervalMs - 1,
			IsClosed:  isClosed,
		})
	}
	return out, nil
}
