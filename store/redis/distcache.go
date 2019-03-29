package redis

import (
	"time"

	"github.com/go-redis/redis"

	"git.coinninja.net/backend/blocc/blocc"
)

// Set sets a distributed cache item
func (c *client) Set(bucket string, key string, value interface{}, expires time.Duration) error {
	return c.client.Set(c.prefix+Delimeter+bucket+Delimeter+key, value, expires).Err()
}

// GetScan gets a distributed cache item
func (c *client) GetScan(bucket string, key string, dest interface{}) error {
	err := c.client.Get(c.prefix + Delimeter + bucket + Delimeter + key).Scan(dest)
	if err == redis.Nil {
		return blocc.ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// GetBytes gets raw bytes from the distributed cache
func (c *client) GetBytes(bucket string, key string) ([]byte, error) {
	b, err := c.client.Get(c.prefix + Delimeter + bucket + Delimeter + key).Bytes()
	if err == redis.Nil {
		return nil, blocc.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return b, nil

}

// Del removes an item from the dist cache
func (c *client) Del(bucket string, key string) error {
	err := c.client.Del(c.prefix + Delimeter + bucket + Delimeter + key).Err()
	if err == redis.Nil {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

// Clear clears the distributed cache
func (c *client) Clear(bucket string) error {
	return c.DelPattern(c.prefix + Delimeter + bucket + Delimeter + "*")
}
