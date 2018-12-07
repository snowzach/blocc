package blocc

import (
	"time"
)

// BlockStore is an interface that is used to get and store blocks and transactions
type BlockStore interface {
	Init(symbol string) error
	Flush(symbol string) error

	InsertBlock(symbol string, blk *Block) error
	UpsertBlock(symbol string, blk *Block) error
	InsertTransaction(symbol string, blk *Tx) error

	// GetBlockHeight(symbol string) (int64, error)

	// GetBlockByHeight(symbol string, height int64, includeRaw bool) (*store.Block, error)
	// GetBlockByBlockId(symbol string, blockId string, includeRaw bool) (*store.Block, error)
	// GetBlockByTxId(symbol string, txId string, includeRaw bool) (*store.Block, error)
	// GetTxByTxId(symbol string, txId string, includeRaw bool) (*store.Tx, error)
	// FindTxByHeight(symbol string, height int64, includeRaw bool) ([]*store.Tx, error)
	// FindTxByBlockId(symbol string, blockId string, includeRaw bool) ([]*store.Tx, error)

	// FindBlocksByAddresses(symbol string, addresses []string, includeRaw bool) ([]*store.Block, error)
	// FindTxByAddresses(symbol string, addresses []string, includeRaw bool) ([]*store.Tx, error)
}

// TxStore will store transactions (mempool)
type TxStore interface {
	Init(symbol string) error
	InsertTransaction(symbol string, tx *Tx, ttl time.Duration) error
	DeleteTransaction(symbol string, txId string) error
	GetTransactionCount(symbol string) (int64, error)
	GetTransactionBytes(symbol string) (int64, error)

	// GetTransactionByTxId(symbol string, txId string) (*store.Transaction, error)
}

// MetricStore is a simple interface for getting and storing metrics related to the block chain
type MetricStore interface {
	Init(string) error
	// InsertMetric(symbol string, t string, m *store.Metric) error
	// FindMetric(symbol string, t string, start time.Time, end time.Time) ([]*store.Metric, error)
}

// TxMsgBus is an interface to subscribe to transactions
type TxMsgBus interface {
	Publish(symbol string, key string, tx *Tx) error
	Subscribe(symbol string, ket string) (TxChannel, error)
}

// TxChannel is a MsgBus channel for transactions
type TxChannel interface {
	Channel() <-chan *Tx
	Close()
}
