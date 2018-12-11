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

	count, err := s.txp.GetTransactionCount(input.Symbol)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not GetTransactionCount: %v", err)
	}
	size, err := s.txp.GetTransactionBytes(input.Symbol)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not GetTransactionBytes: %v", err)
	}

	return &rpc.MemPoolStats{
		Time:   time.Now().UTC().Unix(),
		Count:  count,
		MPSize: size,
	}, nil

}

// GetMemPoolStream streams mempool data
func (s *Server) GetMemPoolStream(input *rpc.Symbol, server rpc.MemPoolRPC_GetMemPoolStreamServer) error {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	sub, err := s.txb.Subscribe(input.Symbol, "stream")
	if err != nil {
		return grpc.Errorf(codes.Internal, "Could not TxMsgBus.Subscribe: %v", err)
	}
	subChan := sub.Channel()

	for {
		select {
		case tx := <-subChan:
			server.Send(tx)
		case <-server.Context().Done():
			sub.Close()
			return nil
		}
	}
	return nil

}
