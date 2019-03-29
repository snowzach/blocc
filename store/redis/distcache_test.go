package redis

import (
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	r.On("Set", c.prefix+Delimeter+"bucket"+Delimeter+"key", "WHATEVER", time.Minute).Once().Return(redis.NewStatusResult("", nil))
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

	answer := "WHATEVER"
	var test string
	r.On("Get", c.prefix+Delimeter+"bucket"+Delimeter+"key").Once().Return(redis.NewStringResult(answer, nil))
	assert.Nil(t, c.GetScan("bucket", "key", &test))
	assert.Equal(t, answer, test)

	r.AssertExpectations(t)

}

func TestGetBytes(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	answer := "WHATEVER"
	r.On("Get", c.prefix+Delimeter+"bucket"+Delimeter+"key").Once().Return(redis.NewStringResult(answer, nil))
	b, err := c.GetBytes("bucket", "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte(answer), b)

	r.AssertExpectations(t)

}

func TestDel(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	r.On("Del", c.prefix+Delimeter+"bucket"+Delimeter+"key").Once().Return(redis.NewIntResult(0, nil))
	assert.Nil(t, c.Del("bucket", "key"))

	r.AssertExpectations(t)

}

func TestClear(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	r.On("Eval", "for _,k in ipairs(redis.call('KEYS',ARGV[1])) do redis.call('DEL',k) end", mock.Anything, c.prefix+Delimeter+"bucket"+Delimeter+"*").Once().Return(redis.NewCmdResult(nil, nil))
	assert.Nil(t, c.Clear("bucket"))

	r.AssertExpectations(t)

}
