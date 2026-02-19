# Candles

Real-time aggregated candlestick (OHLCV) streamer across **Binance**, **Bybit**, and **OKX**, served over gRPC with a terminal candlestick chart client.

```
┌─────────────────────────────────────────────────────┐
│  Binance ──┐                                        │
│  Bybit  ───┼─► Aggregator ─► gRPC server ─► client │
│  OKX    ──┘                   :50051       (TUI)    │
└─────────────────────────────────────────────────────┘
```

## Architecture

| Package | Role |
|---|---|
| `adapter/{binance,bybit,okx}` | WebSocket live feed + HTTP backfill per exchange |
| `aggregator` | Merges candles across exchanges (max High, min Low, sum Volume); force-closes on period race |
| `cmd/srv` | gRPC server — fans subscriptions out to the aggregator |
| `cmd/client` | gRPC client with a bubbletea TUI candlestick chart |

## Prerequisites

- Go 1.22+
- `protoc` — Protocol Buffer compiler
- `protoc-gen-go` and `protoc-gen-go-grpc` plugins

Install the plugins once:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Install `protoc` via your package manager:

```sh
# macOS
brew install protobuf

# Ubuntu / Debian
apt-get install -y protobuf-compiler
```

## Build

```sh
make build
```

This runs `proto → go mod tidy → go build` and places binaries in `bin/`:

```
bin/srv
bin/client
```

Other targets:

```sh
make proto   # regenerate model/protobuf/ from proto/candle.proto
make deps    # go mod tidy only
make clean   # remove bin/
```

## Run locally

Start the server (connects to all three exchanges):

```sh
./bin/srv
```

Start the TUI client in another terminal:

```sh
./bin/client
```

### Client environment variables

| Variable | Default | Description |
|---|---|---|
| `SERVER_ADDR` | `localhost:50051` | gRPC server address |
| `SYMBOL` | `BTCUSDT` | Trading pair |
| `INTERVAL` | `1m` | Candle interval (`1m`, `5m`, `1h`, …) |
| `N_KLINE` | `48` | Number of candles shown on the chart |

Example — watch ETH on the 5-minute chart with 60 candles:

```sh
SYMBOL=ETHUSDT INTERVAL=5m N_KLINE=60 ./bin/client
```

Press **q** or **Ctrl-C** to quit.

## Run with Docker Compose

Starts the server plus three clients (BTC, ETH, SOL on 1m):

```sh
docker compose up --build
```

To add more symbols, copy a client block in [docker-compose.yml](docker-compose.yml) and set `SYMBOL` / `INTERVAL`.

> **Note:** The Docker client containers run headless (no TTY), so the TUI is not rendered — they emit log output instead. Attach a TTY (`docker compose run`) or run the client binary directly for the interactive chart.

## Project structure

```
.
├── proto/
│   └── candle.proto          # Protobuf schema
├── model/
│   ├── candle/candle.go      # Domain Candle struct
│   └── protobuf/             # Generated gRPC code (do not edit)
├── adapter/
│   ├── adapter.go            # CandleHandler / Token / Adapter interfaces
│   ├── binance/
│   ├── bybit/
│   └── okx/
├── aggregator/
│   └── aggregator.go
├── cmd/
│   ├── srv/main.go           # gRPC server
│   └── client/
│       ├── main.go           # Entry point + gRPC streaming goroutine
│       └── tui.go            # Bubbletea model + candlestick chart
├── Dockerfile
├── docker-compose.yml
└── Makefile
```
