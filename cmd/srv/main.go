package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "github.com/yitech/candles/model/protobuf"
)

type server struct {
	pb.UnimplementedCandleServiceServer
}

func (s *server) Subscribe(req *pb.SubscribeRequest, stream pb.CandleService_SubscribeServer) error {
	log.Printf("new subscription: exchange=%s symbol=%s interval=%s",
		req.Exchange, req.Symbol, req.Interval)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("client disconnected: exchange=%s symbol=%s", req.Exchange, req.Symbol)
			return stream.Context().Err()
		case t := <-ticker.C:
			base := 40000 + rand.Float64()*1000
			candle := &pb.Candle{
				Exchange:  req.Exchange,
				Symbol:    req.Symbol,
				Interval:  req.Interval,
				OpenTime:  t.UnixMilli(),
				Open:      fmt.Sprintf("%.2f", base),
				High:      fmt.Sprintf("%.2f", base+rand.Float64()*200),
				Low:       fmt.Sprintf("%.2f", base-rand.Float64()*200),
				Close:     fmt.Sprintf("%.2f", base+rand.Float64()*100-50),
				Volume:    fmt.Sprintf("%.4f", rand.Float64()*100),
				CloseTime: t.UnixMilli() + 59999,
				IsClosed:  false,
			}
			if err := stream.Send(candle); err != nil {
				return err
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterCandleServiceServer(s, &server{})

	log.Printf("gRPC server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
