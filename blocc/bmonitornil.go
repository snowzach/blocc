package blocc

import (
	"time"
)

type BlockMonitorNil struct {
}

func NewBlockMonitorNil() *BlockMonitorNil {
	return &BlockMonitorNil{}
}

// Wait for the block id
func (bhm *BlockMonitorNil) WaitForBlockId(blockId string, expires time.Time) <-chan *Block {
	c := make(chan *Block)
	close(c)
	return c
}

// Wait for the block height
func (bhm *BlockMonitorNil) WaitForBlockHeight(height int64, expires time.Time) <-chan *Block {
	c := make(chan *Block)
	close(c)
	return c
}

func (bhm *BlockMonitorNil) AddBlock(block *Block, expires time.Time) {}
