package blocc

import (
	"time"
)

const (
	HeightUnknown = -1
)

// BlockChainStore is an interface that is used to get and store blocks and transactions
type BlockChainStore interface {
	Init(symbol string) error

	InsertBlock(symbol string, blk *Block) error
	UpsertBlock(symbol string, blk *Block) error
	InsertTransaction(symbol string, blk *Tx) error

	FlushBlocks(symbol string) error
	FlushTransactions(symbol string) error

	// Get the highest block id and height
	GetBlockHeight(symbol string) (string, int64, error)

	// Get Height <-> BlockId
	GetBlockIdByHeight(symbol string, height int64) (string, error)
	GetHeightByBlockId(symbol string, blockId string) (int64, error)

	// Get Block by height or id
	GetBlockByHeight(symbol string, height int64, includeRaw bool) (*Block, error)
	GetBlockByBlockId(symbol string, blockId string, includeRaw bool) (*Block, error)

	GetTxByTxId(symbol string, txId string, includeRaw bool) (*Tx, error)

	// GetBlockByTxId(symbol string, txId string, includeRaw bool) (*store.Block, error)

	// FindTxByHeight(symbol string, height int64, includeRaw bool) ([]*store.Tx, error)
	// FindTxByBlockId(symbol string, blockId string, includeRaw bool) ([]*store.Tx, error)

	// FindBlocksByAddresses(symbol string, addresses []string, includeRaw bool) ([]*store.Block, error)
	// FindTxByAddresses(symbol string, addresses []string, includeRaw bool) ([]*store.Tx, error)
}

// TxPool will store transactions (mempool)
type TxPool interface {
	Init(symbol string) error
	InsertTransaction(symbol string, tx *Tx, ttl time.Duration) error
	DeleteTransaction(symbol string, txId string) error
	GetTransactionCount(symbol string) (int64, error)
	GetTransactionBytes(symbol string) (int64, error)

	// GetTransactionByTxId(symbol string, txId string) (*store.Transaction, error)
}

// TxMsgBus is an interface to subscribe to transactions
type TxBus interface {
	Init(symbol string) error
	Publish(symbol string, key string, tx *Tx) error
	Subscribe(symbol string, ket string) (TxChannel, error)
}

// MetricStore is a simple interface for getting and storing metrics related to the block chain
type MetricStore interface {
	Init(string) error
	// InsertMetric(symbol string, t string, m *store.Metric) error
	// FindMetric(symbol string, t string, start time.Time, end time.Time) ([]*store.Metric, error)
}

// TxChannel is a MsgBus channel for transactions
type TxChannel interface {
	Channel() <-chan *Tx
	Close()
}

// BlockHeightMonitor is a lookup/cachce provider
type BlockTxMonitor interface {
	AddBlock(block *Block, expires time.Time)
	AddTx(tx *Tx, expires time.Time)
	ExpireBelowBlockHeight(height int64)
	Shutdown()
	BlockTxMonitorWaiter
}

type BlockTxMonitorWaiter interface {
	WaitForBlockId(blockId string, expires time.Time) <-chan *Block
	WaitForBlockHeight(height int64, expires time.Time) <-chan *Block
	WaitForTxId(txId string, expires time.Time) <-chan *Tx
}
