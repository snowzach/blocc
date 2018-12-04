package redis

import (
	"fmt"
	"net"
	"time"

	"github.com/go-redis/redis"
	config "github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	txPrefix  = "tx"
	blkPrefix = "blk"
)

type cache struct {
	logger *zap.SugaredLogger
	prefix string
	client redis.UniversalClient
}

func New(prefix string) (*cache, error) {

	c := &cache{
		logger: zap.S().With("package", "cache.redis"),
		prefix: prefix,
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
func (c *cache) Init() error {
	return c.DelPattern(c.prefix + "-*")
}

// Set will create an entry
func (c *cache) Set(key string, data interface{}, expire time.Duration) error {
	return c.client.Set(c.prefix+key, data, expire).Err()
}

// Delete will remove an entry
func (c *cache) Delete(key string) error {
	err := c.client.Del(c.prefix + key).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

// Get will fetch the entry from the cache
func (c *cache) Get(key string, dest interface{}) error {
	return c.client.Get(c.prefix + key).Scan(dest)
}
