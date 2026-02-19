package main

import (
	"context"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/yitech/candles/model/protobuf"
)

func main() {
	addr     := getEnv("SERVER_ADDR", "localhost:50051")
	symbol   := getEnv("SYMBOL",      "BTCUSDT")
	interval := getEnv("INTERVAL",    "1m")
	nKline   := getEnvInt("N_KLINE",  48)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewCandleServiceClient(conn)

	ch := make(chan *pb.Candle, 128)
	go func() {
		for {
			if err := streamCandles(client, symbol, interval, ch); err != nil {
				log.Printf("stream error: %v — retrying in 3s", err)
			}
			time.Sleep(3 * time.Second)
		}
	}()

	p := tea.NewProgram(
		newModel(symbol, interval, nKline, ch),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		log.Fatalf("tui error: %v", err)
	}
}

func streamCandles(client pb.CandleServiceClient, symbol, interval string, ch chan<- *pb.Candle) error {
	stream, err := client.Subscribe(context.Background(), &pb.SubscribeRequest{
		Symbol:   symbol,
		Interval: interval,
	})
	if err != nil {
		return err
	}
	for {
		c, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		ch <- c
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
