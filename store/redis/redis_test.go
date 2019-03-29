package redis

import (
	"testing"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/mocks"
)

func TestDelPattern(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	r.On("Eval", "for _,k in ipairs(redis.call('KEYS',ARGV[1])) do redis.call('DEL',k) end", mock.Anything, "test*").Once().Return(redis.NewCmdResult(nil, nil))

	assert.Nil(t, c.DelPattern("test*"))

	r.AssertExpectations(t)

}
