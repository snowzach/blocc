package redis

import (
	"time"

	"github.com/go-redis/redis"

	"git.coinninja.net/backend/blocc/store"
)

// Init will clear the cache of any existing records
func (c *client) Init(symbol string) error {
	return c.DelPattern(c.symPrefix(symbol) + "*")
}

// InsertTransaction will add a transaction
func (c *client) InsertTransaction(symbol string, tx *store.Tx, expire time.Duration) error {
	return c.client.Set(c.symPrefix(symbol)+tx.TxId, tx.Raw.Len(), expire).Err()
}

// DeleteTransaction will remove a transaction
func (c *client) DeleteTransaction(symbol string, txId string) error {
	err := c.client.Del(c.symPrefix(symbol) + txId).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

func (c *client) GetTransactionCount(symbol string) (int64, error) {
	count, err := c.client.Eval(`local count=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do count=count+1 end; return count`, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return count, nil
}

// KeysSum
func (c *client) GetTransactionBytes(symbol string) (int64, error) {
	size, err := c.client.Eval(`local size=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do size=size+tonumber(redis.call('GET',k)) end; return size`, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return size, nil
}
