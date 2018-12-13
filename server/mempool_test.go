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
	txp := new(mocks.TxPool)
	txb := new(mocks.TxBus)
	s, err := New(txp, txb)
	assert.Nil(t, err)

	i := &rpc.Symbol{Symbol: "test"}

	// Mock call to item store
	txp.On("GetTransactionBytes", "test").Once().Return(int64(234), nil)
	txp.On("GetTransactionCount", "test").Once().Return(int64(123), nil)

	response, err := s.GetMemPoolStats(context.Background(), i)
	assert.Nil(t, err)
	assert.Equal(t, int64(234), response.MPSize)
	assert.Equal(t, int64(123), response.Count)

	// Check remaining expectations
	txp.AssertExpectations(t)
	txb.AssertExpectations(t)

}
