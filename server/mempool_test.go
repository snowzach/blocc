package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"git.coinninja.net/backend/blocc/mocks"
	"git.coinninja.net/backend/blocc/server/rpc"
)

func TestMempoolStats(t *testing.T) {

	// Mock Store and server
	ts := new(mocks.TxStore)
	mb := new(mocks.TxMsgBus)
	s, err := New(ts, mb)
	assert.Nil(t, err)

	i := &rpc.Symbol{Symbol: "test"}

	// Mock call to item store
	ts.On("GetTransactionBytes", "test").Once().Return(int64(234), nil)
	ts.On("GetTransactionCount", "test").Once().Return(int64(123), nil)

	response, err := s.GetMemPoolStats(context.Background(), i)
	assert.Nil(t, err)
	assert.Equal(t, int64(234), response.MPSize)
	assert.Equal(t, int64(123), response.Count)

	// Check remaining expectations
	ts.AssertExpectations(t)

}
