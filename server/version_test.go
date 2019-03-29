package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/mocks"
)

func TestVersionGet(t *testing.T) {

	// Mock Store and server
	bcs := new(mocks.BlockChainStore)
	txb := new(mocks.TxBus)
	dc := new(mocks.DistCache)
	s, err := New(bcs, txb, dc)
	assert.Nil(t, err)

	response, err := s.Version(context.Background(), nil)
	assert.Nil(t, err)
	assert.Equal(t, conf.GitVersion, response.Version)

	// Check remaining expectations
	bcs.AssertExpectations(t)
	txb.AssertExpectations(t)
	dc.AssertExpectations(t)

}
