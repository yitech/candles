package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yitech/candles/adapter/binance"
	"github.com/yitech/candles/adapter/bybit"
	"github.com/yitech/candles/adapter/okx"
	"github.com/yitech/candles/aggregator"
	"github.com/yitech/candles/model/candle"
	pb "github.com/yitech/candles/model/protobuf"
)

const streamBuf = 64 // per-stream channel buffer

type server struct {
	pb.UnimplementedCandleServiceServer
	agg *aggregator.Aggregator
}

// Subscribe fans out to all exchanges via the aggregator and streams merged
// candles to the gRPC client. A buffered channel decouples the aggregator's
// push goroutine from the gRPC send loop.
func (s *server) Subscribe(req *pb.SubscribeRequest, stream pb.CandleService_SubscribeServer) error {
	log.Printf("subscribe: symbol=%s interval=%s", req.Symbol, req.Interval)

	ch := make(chan *candle.Candle, streamBuf)

	tok, err := s.agg.Subscribe(req.Symbol, req.Interval, func(c *candle.Candle) {
		select {
		case ch <- c:
		default:
			log.Printf("warn: slow consumer [%s:%s], candle dropped", req.Symbol, req.Interval)
		}
	})
	if err != nil {
		return status.Errorf(codes.Internal, "aggregator subscribe: %v", err)
	}
	defer tok.Unsubscribe()

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("disconnect: symbol=%s interval=%s", req.Symbol, req.Interval)
			return stream.Context().Err()
		case c := <-ch:
			if err := stream.Send(toProto(c)); err != nil {
				return err
			}
		}
	}
}

func toProto(c *candle.Candle) *pb.Candle {
	return &pb.Candle{
		Exchange:  c.Exchange,
		Symbol:    c.Symbol,
		Interval:  c.Interval,
		OpenTime:  c.OpenTime,
		Open:      c.Open,
		High:      c.High,
		Low:       c.Low,
		Close:     c.Close,
		Volume:    c.Volume,
		CloseTime: c.CloseTime,
		IsClosed:  c.IsClosed,
	}
}

func main() {
	agg := aggregator.New(
		binance.New(),
		bybit.New(),
		okx.New(),
	)
	defer agg.Close()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterCandleServiceServer(s, &server{agg: agg})

	log.Printf("gRPC server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
