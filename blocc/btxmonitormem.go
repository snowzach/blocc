package blocc

import (
	"sync"
	"time"
)

type BlockTxMonitorMemBlock struct {
	block   *Block
	wait    chan struct{}
	expires time.Time
}

type BlockTxMonitorMemTx struct {
	tx      *Tx
	wait    chan struct{}
	expires time.Time
}

type BlockTxMonitorMem struct {
	byBlockId     map[string]*BlockTxMonitorMemBlock
	byBlockHeight map[int64]*BlockTxMonitorMemBlock
	byTxId        map[string]*BlockTxMonitorMemTx
	sync.Mutex
}

func NewBlockTxMonitorMem() *BlockTxMonitorMem {

	btm := &BlockTxMonitorMem{
		byBlockId:     make(map[string]*BlockTxMonitorMemBlock),
		byBlockHeight: make(map[int64]*BlockTxMonitorMemBlock),
		byTxId:        make(map[string]*BlockTxMonitorMemTx),
	}

	// Expire anything waiting
	go func() {
		for {
			btm.Lock()
			now := time.Now()

			// Scan the blocks for the expiring by height
			for h, b := range btm.byBlockHeight {
				// Remove any expired blocks
				if now.Sub(b.expires) > 0 {
					// Close the wait channel if it's open
					closeIfOpen(b.wait)
					// Delete the reference
					delete(btm.byBlockHeight, h)
				}
			}

			// Scan the blocks for the expiring by Id (in case we never got block)
			for blockId, b := range btm.byBlockId {
				// Remove any expired blocks
				if now.Sub(b.expires) > 0 {
					// Close the wait channel if it's open
					closeIfOpen(b.wait)
					// Delete the reference
					delete(btm.byBlockId, blockId)
				}
			}

			// Scan for Tx that are expiring
			for txId, t := range btm.byTxId {
				// Remove any expired tx
				if now.Sub(t.expires) > 0 {
					// Close the wait channel if it's open
					closeIfOpen(t.wait)
					// Delete the reference
					delete(btm.byTxId, txId)
				}
			}

			btm.Unlock()
			time.Sleep(time.Minute)
		}
	}()

	return btm

}

// Close the wait channel if it's open
func closeIfOpen(c chan struct{}) {
	select {
	case <-c:
		// It's closed
	default:
		close(c)
	}
}

func (btm *BlockTxMonitorMem) AddBlock(block *Block, expires time.Time) {
	btm.Lock()
	defer btm.Unlock()

	// We're shutdown
	if btm.byBlockId == nil {
		return
	}

	// Do we have someone already waiting
	if b, ok := btm.byBlockId[block.BlockId]; ok {
		// Save the block
		b.block = block
		b.expires = expires
		// Signal waiters
		closeIfOpen(b.wait)
	} else {
		// We don't, create a record
		b := &BlockTxMonitorMemBlock{
			block:   block,
			wait:    make(chan struct{}),
			expires: expires,
		}
		close(b.wait) // Close the wait channel
		btm.byBlockId[block.BlockId] = b
	}

	// We have a block height
	if block.Height >= 0 {
		// Are there any waiters
		if b, ok := btm.byBlockHeight[block.Height]; ok {
			// Save the block
			b.block = block
			b.expires = expires
			// Signal waiters
			closeIfOpen(b.wait)
		} else {
			b := &BlockTxMonitorMemBlock{
				block:   block,
				wait:    make(chan struct{}),
				expires: expires,
			}
			close(b.wait) // Close the wait channel
			btm.byBlockHeight[block.Height] = b
		}
	}

}

func (btm *BlockTxMonitorMem) AddTx(tx *Tx, expires time.Time) {
	btm.Lock()
	defer btm.Unlock()

	// We're shutdown
	if btm.byTxId == nil {
		return
	}

	// Do we have this txId or have other waiters
	if t, ok := btm.byTxId[tx.TxId]; ok {
		// Record tx
		t.tx = tx
		t.expires = expires
		// Signal Waiters
		closeIfOpen(t.wait)
	} else {
		t := &BlockTxMonitorMemTx{
			tx:      tx,
			wait:    make(chan struct{}),
			expires: expires,
		}
		close(t.wait) // Close the wait channel
		btm.byTxId[tx.TxId] = t
	}
}

// Wait for the block id
func (btm *BlockTxMonitorMem) WaitForBlockId(blockId string, timeout time.Duration) <-chan *Block {
	btm.Lock()

	c := make(chan *Block, 1)

	// If shutdown
	if btm.byBlockId == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this block
	b, ok := btm.byBlockId[blockId]
	if !ok {
		// We, don't, create a new one
		b = &BlockTxMonitorMemBlock{
			block:   nil,
			wait:    make(chan struct{}),
			expires: time.Now().Add(timeout + (5 * time.Second)), // Make it timeout a little after the actual timeout
		}
		btm.byBlockId[blockId] = b
	}

	btm.Unlock()

	go func() {
		select {
		// We got the block or it expired
		case <-b.wait:
			c <- b.block
		// We timed out
		case <-time.After(timeout):
			c <- nil
		}
	}()

	return c

}

// Wait for the block height
func (btm *BlockTxMonitorMem) WaitForBlockHeight(blockHeight int64, timeout time.Duration) <-chan *Block {
	btm.Lock()

	c := make(chan *Block, 1)

	// If shutdown
	if btm.byBlockHeight == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this block
	b, ok := btm.byBlockHeight[blockHeight]
	if !ok {
		// We, don't, create a new one
		b = &BlockTxMonitorMemBlock{
			block:   nil,
			wait:    make(chan struct{}),
			expires: time.Now().Add(timeout + (5 * time.Second)), // Make it timeout a little after the actual timeout
		}
		btm.byBlockHeight[blockHeight] = b
	}

	btm.Unlock()

	go func() {
		select {
		// We got the block or it expired
		case <-b.wait:
			c <- b.block
		// It timed out
		case <-time.After(timeout):
			c <- nil
		}
	}()

	return c

}

// Wait for the txId
func (btm *BlockTxMonitorMem) WaitForTxId(txId string, timeout time.Duration) <-chan *Tx {

	btm.Lock()

	c := make(chan *Tx, 1)

	// If shutdown
	if btm.byTxId == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this block
	t, ok := btm.byTxId[txId]
	if !ok {
		// We, don't, create a new one
		t = &BlockTxMonitorMemTx{
			tx:      nil,
			wait:    make(chan struct{}),
			expires: time.Now().Add(timeout + (5 * time.Second)), // Make it timeout a little after the actual timeout
		}
		btm.byTxId[txId] = t
	}

	btm.Unlock()

	select {
	// We got the tx or it expired
	case <-t.wait:
		c <- t.tx
	// It timed out
	case <-time.After(timeout):
		c <- nil
	}

	return c

}

func (btm *BlockTxMonitorMem) ExpireBelowBlockHeight(blockHeight int64) {
	btm.Lock()
	defer btm.Unlock()
	for bh, b := range btm.byBlockHeight {
		if bh < blockHeight {
			closeIfOpen(b.wait)
			delete(btm.byBlockHeight, bh)
			if b.block != nil {
				delete(btm.byBlockId, b.block.BlockId)
			}
		}
	}
}

func (btm *BlockTxMonitorMem) Shutdown() {
	btm.Lock()
	defer btm.Unlock()
	for h, b := range btm.byBlockHeight {
		closeIfOpen(b.wait)
		delete(btm.byBlockHeight, h)
	}
	for id, b := range btm.byBlockId {
		closeIfOpen(b.wait)
		delete(btm.byBlockId, id)
	}
	for id, b := range btm.byTxId {
		closeIfOpen(b.wait)
		delete(btm.byTxId, id)
	}
	btm.byBlockId = nil
	btm.byBlockHeight = nil
	btm.byTxId = nil
}
