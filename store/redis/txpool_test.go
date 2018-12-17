package redis

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/mocks"
)

func TestInsertTransaction(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	symbol := "sym"
	tx := &blocc.Tx{
		Type:      "tx",
		Symbol:    "sym",
		BlockId:   "12345",
		TxId:      "67890",
		Height:    42,
		Time:      time.Now().UTC().Unix(),
		BlockTime: time.Now().UTC().Unix(),
		Addresses: []string{"abcd", "efgh"},
		Raw:       []byte("asdfasdfasdfasdf"),
		Data: map[string]string{
			"metric": "value1",
			"tag":    "tag1",
			"size":   "123",
		},
	}

	r.On("Set", c.symPrefix(symbol)+tx.TxId, tx.Data["size"], mock.AnythingOfType("time.Duration")).Once().Return(redis.NewStatusResult("OK", nil))
	assert.Nil(t, c.InsertTransaction(symbol, tx, time.Minute))

	r.AssertExpectations(t)

}

func TestDeleteTransaction(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	symbol := "sym"
	txId := "12345"
	r.On("Del", c.symPrefix(symbol)+txId).Once().Return(redis.NewIntResult(0, nil))
	assert.Nil(t, c.DeleteTransaction(symbol, txId))

	r.AssertExpectations(t)

}

func TestGetTransactionCount(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	symbol := "sym"
	count := int64(3)
	r.On("Eval", TxCountScript, mock.Anything, c.symPrefix(symbol)+"*").Once().Return(redis.NewCmdResult(fmt.Sprintf("%d", count), nil))
	testC, err := c.GetTransactionCount(symbol)
	assert.Nil(t, err)
	assert.Equal(t, count, testC)

	r.AssertExpectations(t)

}

func TestGetTransactionBytes(t *testing.T) {

	r := new(mocks.UniversalClient)
	c := &client{
		logger: zap.S().With("package", "cache.redis"),
		prefix: "test",
		client: r,
	}

	symbol := "sym"
	count := int64(3)
	r.On("Eval", TxSizeScript, mock.Anything, c.symPrefix(symbol)+"*").Once().Return(redis.NewCmdResult(fmt.Sprintf("%d", count), nil))
	testC, err := c.GetTransactionBytes(symbol)
	assert.Nil(t, err)
	assert.Equal(t, count, testC)

	r.AssertExpectations(t)

}
