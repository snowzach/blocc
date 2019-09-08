package bloccserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/mocks"
)

func TestMempoolStats(t *testing.T) {

	// Mock Store and server
	bcs := new(mocks.BlockChainStore)
	txb := new(mocks.TxBus)
	dc := new(mocks.DistCache)
	s, err := New(bcs, txb, dc)
	assert.Nil(t, err)

	i := &blocc.Symbol{Symbol: "test"}

	// Mock request to cache
	dc.On("GetScan", "mempool", "stats", mock.AnythingOfType("*blocc.MemPoolStats")).Once().Return(blocc.ErrNotFound)
	dc.On("Set", "mempool", "stats", mock.AnythingOfType("*blocc.MemPoolStats"), mock.AnythingOfType("time.Duration")).Once().Return(nil)

	// Mock call to item store
	bcs.On("GetMemPoolStats", "test").Once().Return(int64(234), int64(123), nil)

	response, err := s.GetMemPoolStats(context.Background(), i)
	assert.Nil(t, err)
	assert.Equal(t, int64(234), response.MPSize)
	assert.Equal(t, int64(123), response.Count)

	// Check remaining expectations
	bcs.AssertExpectations(t)
	txb.AssertExpectations(t)
	dc.AssertExpectations(t)

}
