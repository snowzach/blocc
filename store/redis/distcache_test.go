package redis

import (
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/mocks"
)

func TestSet(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	r.On("Set", c.prefix+Delimeter+"bucket"+Delimeter+"key", "WHATEVER", time.Minute).Once().Return(redis.NewStatusCmd(nil))
	assert.Nil(t, c.Set("bucket", "key", "WHATEVER", time.Minute))

	r.AssertExpectations(t)

}

func TestGetScan(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	var testString string
	r.On("Get", c.prefix+Delimeter+"bucket"+Delimeter+"key").Once().Return(redis.NewStringCmd("WHATEVER"))
	assert.Nil(t, c.GetScan("bucket", "key", &testString))

	r.AssertExpectations(t)

}

// func (c *client) GetBytes(bucket string, key string) ([]byte, error) {
// func (c *client) Del(bucket string, key string) error {
// func (c *client) Clear(bucket string) error {
