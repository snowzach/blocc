package server

import (
	"context"
	"regexp"

	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"git.coinninja.net/backend/blocc/blocc"
)

const (
	blockIncludeDefault = blocc.BlockIncludeHeader
)

var (
	allNumbersRegexp = regexp.MustCompile(`^[0-9]*$`)
)

// GetBlock returns a block by Id
func (s *Server) GetBlock(ctx context.Context, input *blocc.Get) (*blocc.Block, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	// Set the bit values
	include := blockIncludeDefault
	if blocc.BlockInclude(input.Include) != blocc.BlockIncludeDefault {
		include = blocc.BlockInclude(input.Include)
	}
	if input.Data {
		include |= blocc.BlockIncludeData
	}
	if input.Raw {
		include |= blocc.BlockIncludeRaw
	}
	if input.Tx {
		include |= blocc.BlockIncludeTxIds
	}

	var blk *blocc.Block
	var err error

	// Determine what input is
	switch {
	case allNumbersRegexp.MatchString(input.Id):
		blks, err := s.blockChainStore.FindBlocksByHeight(input.Symbol, cast.ToInt64(input.Id), include)
		if err != nil {
			break
		}
		if len(blks) == 0 {
			return nil, blocc.ErrNotFound
		}
		// Return the first one we find
		blk = blks[0]
	case input.Id == "tip":
		blk, err = s.blockChainStore.GetBlockTopByStatuses(input.Symbol, nil, include)
	default: // Assume it's a block id
		blk, err = s.blockChainStore.GetBlockByBlockId(input.Symbol, input.Id, include)
	}

	if err == blocc.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "Not Found")
	} else if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Could not get block")
	}

	return blk, nil

}

// FindBlocks returns blocks by blockIds (with time and pagination)
func (s *Server) FindBlocks(ctx context.Context, input *blocc.Find) (*blocc.Blocks, error) {

	if input.Symbol == "" {
		input.Symbol = s.defaultSymbol
	}

	if input.Count == 0 {
		input.Count = int64(s.defaultCount)
	}

	start := blocc.ParseUnixTime(input.StartTime)
	end := blocc.ParseUnixTime(input.EndTime)

	// Set the bit values
	include := blockIncludeDefault
	if blocc.BlockInclude(input.Include) != blocc.BlockIncludeDefault {
		include = blocc.BlockInclude(input.Include)
	}
	if input.Data {
		include |= blocc.BlockIncludeData
	}
	if input.Raw {
		include |= blocc.BlockIncludeRaw
	}
	if input.Tx {
		include |= blocc.BlockIncludeTxIds
	}

	blks, err := s.blockChainStore.FindBlocksByBlockIdsAndTime(input.Symbol, input.Ids, start, end, include, int(input.Offset), int(input.Count))
	if err != nil && err != blocc.ErrNotFound {
		s.logger.Errorw("Could not blockChainStore.FindBlocksByBlockIdsAndTime", "error", err)
		return nil, grpc.Errorf(codes.Internal, "Could not get blocks")
	}

	return &blocc.Blocks{
		Blocks: blks,
	}, nil

}
