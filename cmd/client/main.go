package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/yitech/candles/model/protobuf"
)

func main() {
	addr     := getEnv("SERVER_ADDR", "localhost:50051")
	exchange := getEnv("EXCHANGE",    "binance")
	symbol   := getEnv("SYMBOL",      "BTCUSDT")
	interval := getEnv("INTERVAL",    "1m")

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewCandleServiceClient(conn)

	for {
		if err := subscribe(client, exchange, symbol, interval); err != nil {
			log.Printf("subscription error: %v — retrying in 3s", err)
			time.Sleep(3 * time.Second)
		}
	}
}

func subscribe(client pb.CandleServiceClient, exchange, symbol, interval string) error {
	stream, err := client.Subscribe(context.Background(), &pb.SubscribeRequest{
		Exchange: exchange,
		Symbol:   symbol,
		Interval: interval,
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	log.Printf("connected — exchange=%s symbol=%s interval=%s", exchange, symbol, interval)

	for {
		candle, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		log.Printf("[%s/%s/%s] O=%-10s H=%-10s L=%-10s C=%-10s V=%s closed=%v",
			candle.Exchange, candle.Symbol, candle.Interval,
			candle.Open, candle.High, candle.Low, candle.Close,
			candle.Volume, candle.IsClosed)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
