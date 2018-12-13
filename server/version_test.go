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
	txp := new(mocks.TxPool)
	txb := new(mocks.TxBus)
	s, err := New(txp, txb)
	assert.Nil(t, err)

	response, err := s.Version(context.Background(), nil)
	assert.Nil(t, err)
	assert.Equal(t, conf.GitVersion, response.Version)

	// Check remaining expectations
	txp.AssertExpectations(t)
	txb.AssertExpectations(t)

}
