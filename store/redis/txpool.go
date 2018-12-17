package redis

import (
	"time"

	"github.com/go-redis/redis"

	"git.coinninja.net/backend/blocc/blocc"
)

const (
	TxCountScript = `local count=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do count=count+1 end; return count`
	TxSizeScript  = `local size=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do size=size+tonumber(redis.call('GET',k)) end; return size`
)

// Init will clear the cache of any existing records
func (c *client) Init(symbol string) error {
	return c.DelPattern(c.symPrefix(symbol) + "*")
}

// InsertTransaction will add a transaction
func (c *client) InsertTransaction(symbol string, tx *blocc.Tx, expire time.Duration) error {
	// Currently all we care about is size
	size, ok := tx.Data["size"]
	if !ok {
		size = "0"
	}
	return c.client.Set(c.symPrefix(symbol)+tx.TxId, size, expire).Err()
}

// DeleteTransaction will remove a transaction
func (c *client) DeleteTransaction(symbol string, txId string) error {
	err := c.client.Del(c.symPrefix(symbol) + txId).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

// GetTransactionCount will return the count of transactions in the pool
func (c *client) GetTransactionCount(symbol string) (int64, error) {
	count, err := c.client.Eval(TxCountScript, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return count, nil
}

// GetTransactionBytes will return the number of bytes in the transaction pool
func (c *client) GetTransactionBytes(symbol string) (int64, error) {
	size, err := c.client.Eval(TxSizeScript, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return size, nil
}
