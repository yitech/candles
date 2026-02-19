package aggregator

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/yitech/candles/adapter"
	"github.com/yitech/candles/model/candle"
)

// MaxRequestLimit is the target buffer size after a resize.
// The buffer grows freely until it hits 2×MaxRequestLimit, then trims back.
const MaxRequestLimit = 365

// Aggregator multiplexes candle updates from multiple exchange adapters into a
// single aggregated stream per "symbol:interval" key.
//
// Closed semantics: a period is marked IsClosed only when every exchange has
// confirmed it.  If exchange A starts the next period before exchange B has
// closed the current one, the current period is force-closed immediately.
// Late-arriving candles for an already-finalized period are dropped.
type Aggregator struct {
	adapters []adapter.Adapter
	numEx    int
	maxLimit int

	mu     sync.Mutex
	states map[string]*symState
}

// symState holds runtime data for one "symbol:interval" key.
type symState struct {
	mu       sync.Mutex
	setup    bool
	setupErr error

	// Exchange-level subscription tokens (for cleanup).
	tokens []adapter.Token

	// Rolling history of finalized candles.
	candles []candle.Candle

	// In-flight periods, keyed by openTime.
	pending map[int64]*pendingCandle

	// openTimes that have been finalized (normally or force-closed).
	finalized map[int64]struct{}

	// Registered downstream handlers.
	handlers map[uint64]adapter.CandleHandler
	nextID   uint64
}

// pendingCandle tracks the merged state of one time period across all exchanges.
type pendingCandle struct {
	agg         candle.Candle
	perExchange map[string]*candle.Candle
	closedBy    map[string]struct{}
}

// aggregatorToken cancels a single handler registration.
type aggregatorToken struct {
	id    uint64
	state *symState
}

func (t *aggregatorToken) Unsubscribe() {
	t.state.mu.Lock()
	delete(t.state.handlers, t.id)
	t.state.mu.Unlock()
}

// New creates an Aggregator backed by the given exchange adapters.
func New(adapters ...adapter.Adapter) *Aggregator {
	return &Aggregator{
		adapters: adapters,
		numEx:    len(adapters),
		maxLimit: MaxRequestLimit,
		states:   make(map[string]*symState),
	}
}

// Subscribe registers handler to receive aggregated candle updates for
// symbol/interval.  Exchange subscriptions are created lazily on the first
// call for each key.
func (a *Aggregator) Subscribe(symbol, interval string, handler adapter.CandleHandler) (adapter.Token, error) {
	key := symbol + ":" + interval
	state := a.getOrCreateState(key)

	// Register the handler before starting exchange connections so we
	// never miss an early candle.
	state.mu.Lock()
	id := state.nextID
	state.nextID++
	state.handlers[id] = handler
	needsSetup := !state.setup
	if needsSetup {
		state.setup = true // claim the setup slot
	}
	state.mu.Unlock()

	if needsSetup {
		tokens, err := a.startExchangeSubs(key, symbol, interval, state)
		state.mu.Lock()
		if err != nil {
			state.setup = false // allow a future retry
			delete(state.handlers, id)
			state.setupErr = err
		} else {
			state.tokens = tokens
			state.setupErr = nil
		}
		state.mu.Unlock()
		if err != nil {
			return nil, err
		}
	} else {
		// Wait for setup that another goroutine may still be running.
		state.mu.Lock()
		err := state.setupErr
		state.mu.Unlock()
		if err != nil {
			state.mu.Lock()
			delete(state.handlers, id)
			state.mu.Unlock()
			return nil, err
		}
	}

	return &aggregatorToken{id: id, state: state}, nil
}

// Backfill fetches historical candles from every exchange, merges them by
// openTime, and returns them in chronological order.
func (a *Aggregator) Backfill(symbol, interval string, start, end time.Time) ([]*candle.Candle, error) {
	// Collect candles per openTime from all exchanges.
	groups := make(map[int64]map[string]*candle.Candle)

	for _, ad := range a.adapters {
		batch, err := ad.Backfill(symbol, interval, start, end)
		if err != nil {
			return nil, fmt.Errorf("aggregator backfill [%s:%s]: %w", symbol, interval, err)
		}
		for _, c := range batch {
			if groups[c.OpenTime] == nil {
				groups[c.OpenTime] = make(map[string]*candle.Candle)
			}
			groups[c.OpenTime][c.Exchange] = c
		}
	}

	// Sort openTimes chronologically.
	times := make([]int64, 0, len(groups))
	for t := range groups {
		times = append(times, t)
	}
	slices.Sort(times)

	out := make([]*candle.Candle, 0, len(times))
	for _, t := range times {
		agg := merge(groups[t])
		agg.IsClosed = true // historical candles are always closed
		out = append(out, &agg)
	}
	return out, nil
}

// Close cancels all exchange subscriptions managed by this aggregator.
func (a *Aggregator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, state := range a.states {
		state.mu.Lock()
		for _, tok := range state.tokens {
			tok.Unsubscribe()
		}
		state.tokens = nil
		state.mu.Unlock()
	}
}

// ── internal ─────────────────────────────────────────────────────────────────

func (a *Aggregator) getOrCreateState(key string) *symState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.states[key]; ok {
		return s
	}
	s := &symState{
		pending:   make(map[int64]*pendingCandle),
		finalized: make(map[int64]struct{}),
		handlers:  make(map[uint64]adapter.CandleHandler),
	}
	a.states[key] = s
	return s
}

func (a *Aggregator) startExchangeSubs(key, symbol, interval string, state *symState) ([]adapter.Token, error) {
	tokens := make([]adapter.Token, 0, len(a.adapters))
	for _, ad := range a.adapters {
		tok, err := ad.Subscribe(symbol, interval, func(c *candle.Candle) {
			a.handleCandle(state, c)
		})
		if err != nil {
			for _, t := range tokens {
				t.Unsubscribe()
			}
			return nil, fmt.Errorf("aggregator [%s]: %w", key, err)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

// handleCandle is called by every exchange adapter for every incoming candle.
func (a *Aggregator) handleCandle(state *symState, c *candle.Candle) {
	openTime := c.OpenTime
	var toPublish []candle.Candle

	state.mu.Lock()

	// 1. Drop candles for already-finalized periods.
	if _, done := state.finalized[openTime]; done {
		state.mu.Unlock()
		return
	}

	// 2. Force-close any pending period that is older than the incoming one.
	//    This handles the race where exchange A has moved to the next period
	//    before exchange B confirmed the close of the current period.
	for t, p := range state.pending {
		if t < openTime {
			p.agg.IsClosed = true
			appendAndResize(state, p.agg, a.maxLimit)
			toPublish = append(toPublish, p.agg)
			delete(state.pending, t)
			state.finalized[t] = struct{}{}
		}
	}

	// 3. Get or create the pending entry for this period.
	p, ok := state.pending[openTime]
	if !ok {
		p = &pendingCandle{
			perExchange: make(map[string]*candle.Candle),
			closedBy:    make(map[string]struct{}),
		}
		state.pending[openTime] = p
	}

	// 4. Store the latest candle from this exchange and re-merge.
	cp := *c
	p.perExchange[c.Exchange] = &cp
	if c.IsClosed {
		p.closedBy[c.Exchange] = struct{}{}
	}
	p.agg = merge(p.perExchange)

	// 5. Finalize the period when all exchanges have confirmed the close.
	if len(p.closedBy) == a.numEx {
		p.agg.IsClosed = true
		appendAndResize(state, p.agg, a.maxLimit)
		delete(state.pending, openTime)
		state.finalized[openTime] = struct{}{}
	}

	toPublish = append(toPublish, p.agg)

	// Snapshot handlers before releasing the lock to avoid holding it
	// while calling user code.
	hs := snapshotHandlers(state)
	state.mu.Unlock()

	for _, c := range toPublish {
		for _, h := range hs {
			h(&c)
		}
	}
}

// snapshotHandlers returns a copy of the handler slice (called under lock).
func snapshotHandlers(state *symState) []adapter.CandleHandler {
	hs := make([]adapter.CandleHandler, 0, len(state.handlers))
	for _, h := range state.handlers {
		hs = append(hs, h)
	}
	return hs
}

// appendAndResize appends c to the buffer and trims if it exceeds 2×limit.
func appendAndResize(state *symState, c candle.Candle, limit int) {
	state.candles = append(state.candles, c)
	if len(state.candles) > limit*2 {
		// Keep the most recent `limit` candles; wait for the buffer to
		// grow to 2×limit again before the next resize.
		state.candles = state.candles[len(state.candles)-limit:]
	}
}

// merge combines per-exchange candles into one aggregated candle.
//   - Exchange : "aggregated"
//   - Open     : from the first exchange (all share the same period open)
//   - High     : max across exchanges
//   - Low      : min across exchanges
//   - Close    : last update received (map iteration order is random;
//                for determinism, callers that care should sort by exchange)
//   - Volume   : sum across exchanges
//   - IsClosed : set by caller (not by merge)
func merge(perEx map[string]*candle.Candle) candle.Candle {
	var agg candle.Candle
	var sumVol, maxH, minL float64
	first := true

	for _, c := range perEx {
		if first {
			agg = *c
			agg.Exchange = "aggregated"
			maxH, _ = strconv.ParseFloat(c.High, 64)
			minL, _ = strconv.ParseFloat(c.Low, 64)
			sumVol, _ = strconv.ParseFloat(c.Volume, 64)
			first = false
			continue
		}
		if h, _ := strconv.ParseFloat(c.High, 64); h > maxH {
			maxH = h
			agg.High = c.High
		}
		if l, _ := strconv.ParseFloat(c.Low, 64); l < minL {
			minL = l
			agg.Low = c.Low
		}
		if v, _ := strconv.ParseFloat(c.Volume, 64); v > 0 {
			sumVol += v
		}
		agg.Close = c.Close
	}

	agg.Volume = strconv.FormatFloat(sumVol, 'f', -1, 64)
	agg.IsClosed = false // caller decides
	return agg
}
