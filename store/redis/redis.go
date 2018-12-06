package redis

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-redis/redis"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/store"
)

const (
	txPrefix  = "tx"
	blkPrefix = "blk"

	Delimeter = ":"
)

type cache struct {
	logger *zap.SugaredLogger
	prefix string
	client redis.UniversalClient
}

func New(prefixes ...string) (*cache, error) {

	c := &cache{
		logger: zap.S().With("package", "cache.redis"),
		prefix: strings.Join(prefixes, Delimeter),
	}

	// Initialize client
	c.client = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{net.JoinHostPort(config.GetString("redis.host"), config.GetString("redis.port"))},
		Password: config.GetString("redis.password"),
		DB:       config.GetInt("redis.index"),
	})
	_, err := c.client.Ping().Result()
	if err != nil {
		return c, fmt.Errorf("Could not connect to redis: %s", err)
	}

	return c, nil

}

// DelPattern will remove any keys matching the pattern
func (c *cache) DelPattern(pattern string) error {
	err := c.client.Eval(`for _,k in ipairs(redis.call('KEYS',ARGV[1])) do redis.call('DEL',k) end`, nil, pattern).Err()
	if err == redis.Nil {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

// Init will clear the cache of any existing records
func (c *cache) Init(symbol string) error {
	return c.DelPattern(c.symPrefix(symbol) + "*")
}

// InsertTransaction will add a transaction
func (c *cache) InsertTransaction(symbol string, tx *store.Tx, expire time.Duration) error {
	return c.client.Set(c.symPrefix(symbol)+tx.TxId, tx.Raw.Len(), expire).Err()
}

// DeleteTransaction will remove a transaction
func (c *cache) DeleteTransaction(symbol string, txId string) error {
	err := c.client.Del(c.symPrefix(symbol) + txId).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

func (c *cache) GetTransactionCount(symbol string) (int64, error) {
	count, err := c.client.Eval(`local count=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do count=count+1 end; return count`, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return count, nil
}

// KeysSum
func (c *cache) GetTransactionBytes(symbol string) (int64, error) {
	size, err := c.client.Eval(`local size=0; for _,k in ipairs(redis.call('KEYS',ARGV[1])) do size=size+tonumber(redis.call('GET',k)) end; return size`, nil, c.symPrefix(symbol)+"*").Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return size, nil
}

func (c *cache) symPrefix(symbol string) string {
	return c.prefix + Delimeter + symbol + Delimeter
}

// // Get will fetch the entry from the cache
// func (c *cache) Get(key string, dest interface{}) error {
// 	return c.client.Get(c.prefix + key).Scan(dest)
// }
