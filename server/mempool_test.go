package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"git.coinninja.net/backend/blocc/mocks"
	"git.coinninja.net/backend/blocc/server/rpc"
	"git.coinninja.net/backend/blocc/store"
)

func TestMempoolStats(t *testing.T) {

	// Mock Store and server
	txp := new(mocks.TxPool)
	txb := new(mocks.TxBus)
	dc := new(mocks.DistCache)
	s, err := New(dc, txp, txb)
	assert.Nil(t, err)

	i := &rpc.Symbol{Symbol: "test"}

	// Mock request to cache
	dc.On("GetScan", "mempool", "stats", mock.AnythingOfType("*rpc.MemPoolStats")).Once().Return(store.ErrNotFound)
	dc.On("Set", "mempool", "stats", mock.AnythingOfType("*rpc.MemPoolStats"), mock.AnythingOfType("time.Duration")).Once().Return(nil)

	// Mock call to item store
	txp.On("GetTransactionBytes", "test").Once().Return(int64(234), nil)
	txp.On("GetTransactionCount", "test").Once().Return(int64(123), nil)

	response, err := s.GetMemPoolStats(context.Background(), i)
	assert.Nil(t, err)
	assert.Equal(t, int64(234), response.MPSize)
	assert.Equal(t, int64(123), response.Count)

	// Check remaining expectations
	dc.AssertExpectations(t)
	txp.AssertExpectations(t)
	txb.AssertExpectations(t)

}
