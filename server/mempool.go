package server

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"git.coinninja.net/backend/blocc/server/rpc"
	"git.coinninja.net/backend/blocc/store"
)

// GetMemPoolStats returns mempool statistics
func (s *Server) GetMemPoolStats(ctx context.Context, input *rpc.Symbol) (*rpc.MemPoolStats, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	mps := new(rpc.MemPoolStats)

	// Check the cache
	err := s.dc.GetScan("mempool", "stats", mps)
	if err == nil {
		return mps, nil
	} else if err != nil && err != store.ErrNotFound {
		s.logger.Errorw("Could not check DistCache for stats", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not GetMemPoolStats")

	}

	// Fetch the values
	count, err := s.txp.GetTransactionCount(input.Symbol)
	if err != nil {
		s.logger.Errorw("Could not GetTransactionCount", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not GetMemPoolStats")
	}
	size, err := s.txp.GetTransactionBytes(input.Symbol)
	if err != nil {
		s.logger.Errorw("Could not GetTransactionBytes", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not GetMemPoolStats")
	}

	// Build the return
	mps.Time = time.Now().UTC().Unix()
	mps.Count = count
	mps.MPSize = size

	// Set it in the cache
	err = s.dc.Set("mempool", "stats", mps, 15*time.Second)
	if err != nil {
		s.logger.Errorw("Could not set DistCache stats", "error", err)
	}

	return mps, nil

}

// GetMemPoolStream streams mempool data
func (s *Server) GetMemPoolStream(input *rpc.Symbol, server rpc.MemPoolRPC_GetMemPoolStreamServer) error {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	sub, err := s.txb.Subscribe(input.Symbol, "stream")
	if err != nil {
		s.logger.Errorw("Could not TxMsgBus.Subscribe", "error", err)
		return grpc.Errorf(codes.Internal, "Could not GetMemPoolStream")
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
