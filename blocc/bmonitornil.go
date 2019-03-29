package blocc

import (
	"time"
)

// This is a block monitor that returns nothing - it's really only for testing
type BlockMonitorNil struct {
}

// This returns a BlockMonitor that does not return anything - it's for testing
func NewBlockMonitorNil() *BlockMonitorNil {
	return &BlockMonitorNil{}
}

// This returns nil channel
func (bhm *BlockMonitorNil) WaitForBlockId(blockId string, expires time.Time) <-chan *Block {
	c := make(chan *Block)
	close(c)
	return c
}

// This returns nil channel
func (bhm *BlockMonitorNil) WaitForBlockHeight(height int64, expires time.Time) <-chan *Block {
	c := make(chan *Block)
	close(c)
	return c
}

// This pretends to add a block but actually does nothing
func (bhm *BlockMonitorNil) AddBlock(block *Block, expires time.Time) {}
