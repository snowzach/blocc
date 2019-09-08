package bloccserver

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"git.coinninja.net/backend/blocc/blocc"
)

// GetMemPoolStats returns mempool statistics
func (s *Server) GetMemPoolStats(ctx context.Context, input *blocc.Symbol) (*blocc.MemPoolStats, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	mps := new(blocc.MemPoolStats)

	// Check the cache
	err := s.distCache.GetScan("mempool", "stats", mps)
	if err == nil {
		return mps, nil
	} else if err != nil && err != blocc.ErrNotFound {
		s.logger.Errorw("Could not check DistCache for stats", "error", err)
	}

	// Return the aggregate size and count of transactions with blockId = BlockIdMempool
	size, count, err := s.blockChainStore.GetMemPoolStats(input.Symbol)
	if err != nil {
		s.logger.Errorw("Could not blockChainStore.GetMemPoolStats", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not GetMemPoolStats")
	}

	// Build the return
	mps.Time = time.Now().UTC().Unix()
	mps.Count = count
	mps.MPSize = size

	// Set it in the cache
	err = s.distCache.Set("mempool", "stats", mps, s.cacheTimeout)
	if err != nil {
		s.logger.Errorw("Could not set DistCache stats", "error", err)
	}

	return mps, nil

}

// GetMemPoolStream streams mempool data
func (s *Server) GetMemPoolStream(input *blocc.Symbol, server blocc.BloccRPC_GetMemPoolStreamServer) error {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	sub, err := s.txBus.Subscribe(input.Symbol, "stream")
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
	//	return nil

}
