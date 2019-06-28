package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/server/rpc"
)

const (
	txIncludeDefault = blocc.TxIncludeAllButRaw
)

// GetTransaction gets a transaction by Id
func (s *Server) GetTransaction(ctx context.Context, input *rpc.Get) (*blocc.Tx, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	// Set the bit values
	include := txIncludeDefault
	if blocc.TxInclude(input.Include) != blocc.TxIncludeDefault {
		include = blocc.TxInclude(input.Include)
	}
	if input.Data {
		include |= blocc.TxIncludeData
	}
	if input.Raw {
		include |= blocc.TxIncludeRaw
	}

	tx, err := s.blockChainStore.GetTxByTxId(input.Symbol, input.Id, include)
	if err == blocc.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "Not Found")
	} else if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not get transaction")
	}

	return tx, nil

}

// FindTransactions finds transaction by transaction ids
func (s *Server) FindTransactions(ctx context.Context, input *rpc.Find) (*rpc.Transactions, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	if input.Count == 0 {
		input.Count = int64(s.defaultCount)
	}

	start := blocc.ParseUnixTime(input.StartTime)
	end := blocc.ParseUnixTime(input.EndTime)

	// Set the bit values
	include := txIncludeDefault
	if blocc.TxInclude(input.Include) != blocc.TxIncludeDefault {
		include = blocc.TxInclude(input.Include)
	}
	if input.Data {
		include |= blocc.TxIncludeData
	}
	if input.Raw {
		include |= blocc.TxIncludeRaw
	}

	txs, err := s.blockChainStore.FindTxs(input.Symbol, input.Ids, "", nil, blocc.TxFilterIncompleteAll, start, end, include, int(input.Offset), int(input.Count))
	if err != nil && err != blocc.ErrNotFound {
		s.logger.Errorw("Could not blockChainStore.FindTxsByTxIdsAndTime", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not get blocks")
	}

	return &rpc.Transactions{
		Transactions: txs,
	}, nil

}

// FindTransactionsByAddresses finds transactions by addresses
func (s *Server) FindTransactionsByAddresses(ctx context.Context, input *rpc.Find) (*rpc.Transactions, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	if input.Count == 0 {
		input.Count = int64(s.defaultCount)
	}

	start := blocc.ParseUnixTime(input.StartTime)
	end := blocc.ParseUnixTime(input.EndTime)

	// Set the bit values
	include := txIncludeDefault
	if blocc.TxInclude(input.Include) != blocc.TxIncludeDefault {
		include = blocc.TxInclude(input.Include)
	}
	if input.Data {
		include |= blocc.TxIncludeData
	}
	if input.Raw {
		include |= blocc.TxIncludeRaw
	}

	txs, err := s.blockChainStore.FindTxsByAddressesAndTime(input.Symbol, input.Ids, start, end, blocc.TxFilterAddressInputOutput, include, int(input.Offset), int(input.Count))
	if err != nil && err != blocc.ErrNotFound {
		s.logger.Errorw("Could not blockChainStore.FindTransactionsByAddresses", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not get blocks")
	}

	return &rpc.Transactions{
		Transactions: txs,
	}, nil

}
