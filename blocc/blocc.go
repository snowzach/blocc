package blocc

import (
	"errors"
	"strings"
	"time"
)

// This contains all of the interfaces for the blocc indexer

const (
	HeightUnknown  = -1
	BlockIdMempool = "mempool"

	// Selectively include things when fetching block
	BlockIncludeAll       = BlockIncludeHeader | BlockIncludeData | BlockIncludeRaw | BlockIncludeTxIds
	BlockIncludeAllButRaw = BlockIncludeHeader | BlockIncludeData | BlockIncludeTxIds

	// Selectively include things when fetching tx
	TxIncludeAll       = TxIncludeHeader | TxIncludeData | TxIncludeRaw | TxIncludeIn | TxIncludeOut
	TxIncludeAllButRaw = TxIncludeHeader | TxIncludeData | TxIncludeIn | TxIncludeOut

	// When searching transactions for addresses, apply a filter to which addresses you wish to search
	TxFilterAddressInput       = 1
	TxFilterAddressOutput      = 2
	TxFilterAddressInputOutput = TxFilterAddressInput | TxFilterAddressOutput

	// Status fields
	StatusNew      = "new"
	StatusValid    = "valid"
	StatusInvalid  = "invalid"
	StatusOrphaned = "orphaned"
)

var (
	// Predefined errors
	ErrUnknownSymbol    = errors.New("Unknown Symbol")
	ErrNotFound         = errors.New("Not Found")
	ErrInvalidBlock     = errors.New("Invalid Block")
	ErrMissingBlock     = errors.New("Missing Block")
	ErrMissingInput     = errors.New("Missing Input")
	ErrBestChainUnknown = errors.New("Could not determine best block chain")
)

// BlockChainStore is an interface that is used to get and store blocks and transactions
type BlockChainStore interface {
	Init(symbol string) error

	// Insert, update and delete blocks and transactions
	InsertBlock(symbol string, blk *Block) error
	UpdateBlock(symbol string, blockId string, status string, nextBlockId string, data map[string]string, metric map[string]float64) error
	UpdateBlockStatusByStatusesAndHeight(symbol string, statuses []string, startHeight int64, endHeight int64, status string) error
	DeleteBlockByBlockId(symbol string, blockId string) error
	InsertTransaction(symbol string, blk *Tx) error
	UpsertTransaction(symbol string, blk *Tx) error
	DeleteTransactionsByBlockIdAndTime(symbol string, blockId string, start *time.Time, end *time.Time) error

	// Flushing blocks and transactions
	// The block chain store is allowed to be async to allow batching - ie non-acid compliant
	// Calling these functions should however bring the store up to date such that subsequent query opterations would reflect the current state
	FlushBlocks(symbol string) error
	FlushTransactions(symbol string) error

	// This should delete all blocks and transactions above a block height
	DeleteAboveBlockHeight(symbol string, above int64) error

	// Return the highest block optionally with status (nil or empty status slice implies any status)
	// It's plausible there are multiple top blocks, this will pick one of them
	GetBlockHeaderTopByStatuses(symbol string, statuses []string) (*BlockHeader, error)
	GetBlockTopByStatuses(symbol string, statuses []string, include BlockInclude) (*Block, error) // There may be

	// Get Block,Tx by ___ - This is for things that could only match one thing
	GetBlockByBlockId(symbol string, blockId string, include BlockInclude) (*Block, error)
	GetTxsByTxIds(symbol string, txIds []string, include TxInclude) ([]*Tx, error)
	GetTxsByBlockId(symbol string, blockId string, include TxInclude) ([]*Tx, error)
	GetTxCountByBlockId(symbol string, blockId string, includeIncomplete bool) (int64, error)
	GetTxByTxId(symbol string, txId string, include TxInclude) (*Tx, error)

	// Return the aggregate size and count of transactions with blockId = BlockIdMempool
	GetMemPoolStats(symbol string) (int64, int64, error)
	// Return the aggregate address statistics: txCount, inputs, outputs
	GetAddressStats(symbol string, address string) (int64, int64, int64, error)

	// Find blocks by height - there could possibly be more than one
	FindBlocksByHeight(symbol string, height int64, include BlockInclude) ([]*Block, error)
	// Find blocks by PrevBlockId - there could be more tha one
	FindBlocksByPrevBlockId(symbol string, prevBlockId string, include BlockInclude) ([]*Block, error)
	// Find blocks by TxId - there could be more than one
	FindBlocksByTxId(symbol string, txId string, include BlockInclude) ([]*Block, error)
	// Find blocks by BlockIds and Time - ordered by time descending
	FindBlocksByBlockIdsAndTime(symbol string, blockIds []string, start *time.Time, end *time.Time, include BlockInclude, offset int, count int) ([]*Block, error)
	// Find blocks by status and height ascending (nil or empty status slice implies any status)
	FindBlocksByStatusAndHeight(symbol string, statuses []string, startHeight int64, endHeight int64, include BlockInclude, offset int, count int) ([]*Block, error)
	// Find transactions by address and time period, order by time descending - See filter constants for which addresses you wish to search (inputs or outputs)
	FindTxsByAddressesAndTime(symbol string, addresses []string, start *time.Time, end *time.Time, filter int, include TxInclude, offset int, count int) ([]*Tx, error)
	// Find transactions by txids and time period, order by time descending -
	FindTxs(symbol string, txIds []string, blockId string, dataFields map[string]string, start *time.Time, end *time.Time, include TxInclude, offset int, count int) ([]*Tx, error)

	// This will calculate the average of a data field between block heights
	AverageBlockDataFieldByHeight(symbol string, field string, omitZero bool, startHeight int64, endHeight int64) (float64, error)
	// This will calculate the percentile value of a datafield between block heights
	PercentileBlockDataFieldByHeight(symbol string, field string, percentile float64, omitZero bool, startHeight int64, endHeight int64) (float64, error)
}

// BlockHeaderCache is used to cache block headers and determine height from the block chain follower based on the values provided
// It has no intelligence other than can determine the block with the highest height. If you insert two headers with the same height
// and they have the highest height. The last block to be inserted should be the one returned from the GetTopBlockHeader
type BlockHeaderCache interface {
	Init(symbol string) error
	GetTopBlockHeader(symbol string) (*BlockHeader, error)
	InsertBlockHeader(symbol string, bh *BlockHeader, expires time.Duration) error
	GetBlockHeaderByBlockId(symbol string, blockId string) (*BlockHeader, error)
	GetBlockHeaderByPrevBlockId(symbol string, prevBlockId string) (*BlockHeader, error)
	ExpireBlockHeaderBelowHeight(symbol string, height int64) error
}

// ValidBlockStore is a BlockHeader store that tracks complete blockchain (by Height) but allows out of order insertion
// As you follow the block chain, you add completed blocks to the ValidBlockStore, not necessarily in order. When you GetValidBlock it will
// return you the highest complete BlockHeader it has by height.
// You can force the current valid BlockHeader using SetValidBlock - This removes all existing chain state being tracked
type ValidBlockStore interface {
	AddValidBlock(bh *BlockHeader)
	SetValidBlock(bh *BlockHeader)
	GetValidBlock() *BlockHeader
}

// TxBus is an interface to subscribe to incoming non block-related transactions
type TxBus interface {
	Init(symbol string) error
	Publish(symbol string, key string, tx *Tx) error
	Subscribe(symbol string, ket string) (TxChannel, error)
}

// TxChannel is a MsgBus channel for transactions
type TxChannel interface {
	Channel() <-chan *Tx
	Close()
}

// BlockHeaderTxMonitor is an in memory waiter interface that both caches and provides concurrency tools for parsing blocks
// You can add a BlockHeader or a Tx to the monitor. You can then use the BlockHeaderTxMonitorWaiter to access the data
type BlockHeaderTxMonitor interface {
	AddBlockHeader(bh *BlockHeader, expires time.Duration)
	AddTx(tx *Tx, expires time.Duration)
	ExpireBlockHeadersBelowBlockHeight(height int64)
	ExpireBlockHeadersAboveBlockHeight(height int64)
	ExpireBlockHeadersHeightUnknown()
	Shutdown()
	BlockHeaderTxMonitorWaiter
}

// BlockHeaderTxMonitorWaiter is the waiter portion of the header/tx monitor
// You can wait with a timeout for blockId, blockHeight and TxId as well as monitor the last time a block was put in
type BlockHeaderTxMonitorWaiter interface {
	WaitForBlockId(blockId string, expires time.Duration) <-chan *BlockHeader
	WaitForBlockHeight(height int64, expires time.Duration) <-chan *BlockHeader
	WaitForTxId(txId string, expires time.Duration) <-chan *Tx
	LastBlockHeaderWithHeightTime() time.Time
}

// When there is something wrong with the block chain, it should return validation error in the error string and can be checked with this
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "Validation Error") {
		return true
	}
	return false
}
