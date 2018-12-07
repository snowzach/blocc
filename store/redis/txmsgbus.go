package redis

import (
	"encoding/json"

	"github.com/go-redis/redis"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

type channel struct {
	sub    *redis.PubSub
	client *client
}

// Publish will publish a message to any listeners on the channel
func (c *client) Publish(symbol string, key string, tx *store.Tx) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return c.client.Publish(c.symPrefix(symbol)+key, string(data)).Err()
}

// Publish will publish a message to any listeners on the channel
func (c *client) Subscribe(symbol string, key string) (blocc.TxChannel, error) {
	sub := c.client.Subscribe(c.symPrefix(symbol) + key)
	_, err := sub.Receive()
	if err != nil {
		return nil, err
	}
	return &channel{
		sub:    sub,
		client: c,
	}, nil
}

// Channel get a channel of transaction
func (c *channel) Channel() <-chan *store.Tx {
	txChan := make(chan *store.Tx)
	go func() {
		subChan := c.sub.Channel()
		for {
			// Get a message
			m := <-subChan
			// Channel closed
			if m == nil {
				close(txChan)
				return
			}
			//
			tx := new(store.Tx)
			err := json.Unmarshal([]byte(m.Payload), tx)
			if err != nil {
				c.client.logger.Errorw("Could not unmarshal tx", "error", err, "payload", m.Payload)
				continue
			}
			txChan <- tx
		}
	}()
	return txChan
}

func (c *channel) Close() {
	c.sub.Close()
}
