package server

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"git.coinninja.net/backend/blocc/server/rpc"
)

// GetMemPoolStats returns mempool statistics
func (s *Server) GetMemPoolStats(ctx context.Context, input *rpc.Symbol) (*rpc.MemPoolStats, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	count, err := s.ts.GetTransactionCount(input.Symbol)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not GetTransactionCount: %v", err)
	}
	size, err := s.ts.GetTransactionBytes(input.Symbol)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not GetTransactionBytes: %v", err)
	}

	return &rpc.MemPoolStats{
		Time:  time.Now().UTC().Unix(),
		Count: count,
		Size:  size,
	}, nil

}

// GetMemPoolStream streams mempool data
func (s *Server) GetMemPoolStream(input *rpc.Symbol, server rpc.MemPoolRPC_GetMemPoolStreamServer) error {

	// TODO: Override websocket logger

	defer func() {
		s.logger.Debug("EXIT")
	}()

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	for server.Context().Err() == nil {
		server.Send(&rpc.Test{Test: "TEST"})
		time.Sleep(3 * time.Second)
	}

	return nil

}
