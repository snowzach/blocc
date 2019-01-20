package redis

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-redis/redis"
	config "github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	txPrefix  = "tx"
	blkPrefix = "blk"

	Delimeter = ":"
)

type client struct {
	logger *zap.SugaredLogger
	prefix string
	client redis.UniversalClient
}

func New(prefixes ...string) (*client, error) {

	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: strings.Join(prefixes, Delimeter),
	}

	// Initialize client
	c.client = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:       []string{net.JoinHostPort(config.GetString("redis.host"), config.GetString("redis.port"))},
		Password:    config.GetString("redis.password"),
		DB:          config.GetInt("redis.index"),
		MasterName:  config.GetString("redis.master_name"),
		PoolSize:    1000,
		PoolTimeout: 2 * time.Minute,
	})
	_, err := c.client.Ping().Result()
	if err != nil {
		return c, fmt.Errorf("Could not connect to redis: %s", err)
	}

	return c, nil

}

// Get a subprefix client of redis
func (c *client) Prefix(prefixes ...string) *client {

	newPrefix := c.prefix
	if c.prefix != "" {
		c.prefix += Delimeter
	}
	newPrefix += strings.Join(prefixes, Delimeter)

	return &client{
		logger: c.logger.With("prefix", newPrefix),
		prefix: newPrefix,
		client: c.client,
	}

}

func (c *client) symPrefix(symbol string) string {
	return c.prefix + Delimeter + symbol + Delimeter
}

// DelPattern will remove any keys matching the pattern
func (c *client) DelPattern(pattern string) error {
	err := c.client.Eval(`for _,k in ipairs(redis.call('KEYS',ARGV[1])) do redis.call('DEL',k) end`, nil, pattern).Err()
	if err == redis.Nil {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}
