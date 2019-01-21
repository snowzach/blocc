package blocc

import (
	"sync"
	"time"
)

type BlockTxMonitorMemBlock struct {
	block   *Block
	waiters []chan *Block
	expires time.Time
	sync.Mutex
}

type BlockTxMonitorMemTx struct {
	tx      *Tx
	waiters []chan *Tx
	expires time.Time
	sync.Mutex
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
				b.Lock()
				// Remove any expired blocks
				if now.Sub(b.expires) > 0 {
					delete(btm.byBlockHeight, h)
					for _, c := range b.waiters {
						close(c)
					}
				}
				b.Unlock()
			}

			// Scan the blocks for the expiring by Id (in case we never got block)
			for id, b := range btm.byBlockId {
				b.Lock()
				// Remove any expired blocks
				if now.Sub(b.expires) > 0 {
					delete(btm.byBlockId, id)
					for _, c := range b.waiters {
						close(c)
					}
				}
				b.Unlock()
			}

			// Scan for Tx that are expiring
			for id, b := range btm.byTxId {
				b.Lock()
				// Remove any expired tx
				if now.Sub(b.expires) > 0 {
					delete(btm.byTxId, id)
					for _, c := range b.waiters {
						close(c)
					}
				}
				b.Unlock()
			}

			btm.Unlock()
			time.Sleep(time.Minute)
		}
	}()

	return btm

}

// blockChan will create a channel and immediately return a value and close
func blockChan(b *Block) <-chan *Block {
	c := make(chan *Block, 1)
	if b != nil {
		c <- b
	}
	close(c)
	return c
}

// txChan will create a channel and immediately return a value and close
func txChan(b *Tx) <-chan *Tx {
	c := make(chan *Tx, 1)
	if b != nil {
		c <- b
	}
	close(c)
	return c
}

func timeoutBlockChan(inChan <-chan *Block, timeout time.Duration) <-chan *Block {
	c := make(chan *Block, 1)
	go func() {
		select {
		case blk := <-inChan:
			c <- blk
		case <-time.After(timeout):
			// Timeout
		}
		close(c)
	}()
	return c

}

func timeoutTxChan(inChan <-chan *Tx, timeout time.Duration) <-chan *Tx {
	c := make(chan *Tx, 1)
	go func() {
		select {
		case tx := <-inChan:
			c <- tx
		case <-time.After(timeout):
		}
		close(c)
	}()
	return c
}

// Wait for the block id
func (btm *BlockTxMonitorMem) WaitForBlockId(blockId string, timeout time.Duration) <-chan *Block {
	btm.Lock()

	// If shutdown
	if btm.byBlockId == nil {
		btm.Unlock()
		return blockChan(nil)
	}

	// Do we have this block or have other waiters
	if b, ok := btm.byBlockId[blockId]; ok {
		b.Lock()
		defer b.Unlock()
		defer btm.Unlock()
		// We already have the block
		if b.block != nil {

			return blockChan(b.block)
		}
		// We don't yet have the height, create a channel
		c := make(chan *Block, 1)
		b.waiters = append(b.waiters, c)
		return timeoutBlockChan(c, timeout)
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: time.Now().Add(timeout),
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byBlockId[blockId] = b
	c := make(chan *Block, 1)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return timeoutBlockChan(c, timeout)
}

// Wait for the block height
func (btm *BlockTxMonitorMem) WaitForBlockHeight(height int64, timeout time.Duration) <-chan *Block {
	btm.Lock()

	// If shutdown
	if btm.byBlockHeight == nil {
		btm.Unlock()
		return blockChan(nil)
	}

	// Do we have this block or have other waiters
	if b, ok := btm.byBlockHeight[height]; ok {
		b.Lock()
		defer b.Unlock()
		defer btm.Unlock()
		// We already have the block
		if b.block != nil {
			return blockChan(b.block)
		}
		// We don't yet have the height, create a channel
		c := make(chan *Block, 1)
		b.waiters = append(b.waiters, c)
		return timeoutBlockChan(c, timeout)
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: time.Now().Add(timeout),
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byBlockHeight[height] = b
	c := make(chan *Block, 1)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return timeoutBlockChan(c, timeout)
}

// Wait for the txId
func (btm *BlockTxMonitorMem) WaitForTxId(txId string, timeout time.Duration) <-chan *Tx {
	btm.Lock()

	// If shutdown
	if btm.byTxId == nil {
		btm.Unlock()
		return txChan(nil)
	}

	// Do we have this tx or have other waiters
	if b, ok := btm.byTxId[txId]; ok {
		b.Lock()
		defer b.Unlock()
		defer btm.Unlock()
		// We already have the block
		if b.tx != nil {
			return txChan(b.tx)
		}
		// We don't yet have the tx, create a channel
		c := make(chan *Tx, 1)
		b.waiters = append(b.waiters, c)
		return timeoutTxChan(c, timeout)
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemTx{
		waiters: make([]chan *Tx, 0),
		expires: time.Now().Add(timeout),
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byTxId[txId] = b
	c := make(chan *Tx, 1)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return timeoutTxChan(c, timeout)
}

func (btm *BlockTxMonitorMem) AddBlock(block *Block, expires time.Time) {
	btm.Lock()
	defer btm.Unlock()

	// We're shutdown
	if btm.byBlockId == nil {
		return
	}

	// Do we have this block or have other waiters
	if b, ok := btm.byBlockId[block.BlockId]; ok {
		b.Lock()
		// Save the block
		b.block = block
		b.expires = expires
		// Send message to waiters
		for _, w := range b.waiters {
			w <- block
			close(w)
		}
		b.waiters = nil
		b.Unlock()
	} else {
		b := &BlockTxMonitorMemBlock{
			block:   block,
			expires: expires,
		}
		btm.byBlockId[block.BlockId] = b
	}

	if block.Height >= 0 {
		// Do we have this block or have other waiters
		if b, ok := btm.byBlockHeight[block.Height]; ok {
			b.Lock()
			// Save the block
			b.block = block
			b.expires = expires
			// Send message to waiters
			for _, w := range b.waiters {
				w <- block
				close(w)
			}
			b.waiters = nil
			b.Unlock()
		} else {
			b := &BlockTxMonitorMemBlock{
				block:   block,
				expires: expires,
			}
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
	if b, ok := btm.byTxId[tx.TxId]; ok {
		b.Lock()
		// Record tx
		b.tx = tx
		b.expires = expires
		// Send message to waiters
		for _, w := range b.waiters {
			w <- tx
			close(w)
		}
		b.waiters = nil
		b.Unlock()
	} else {
		b := &BlockTxMonitorMemTx{
			tx:      tx,
			expires: expires,
		}
		btm.byTxId[tx.TxId] = b
	}
}

func (btm *BlockTxMonitorMem) ExpireBelowBlockHeight(height int64) {
	btm.Lock()
	defer btm.Unlock()
	for h, b := range btm.byBlockHeight {
		if h < height {
			b.Lock()
			delete(btm.byBlockHeight, h)
			delete(btm.byBlockId, b.block.BlockId)
			for _, c := range b.waiters {
				close(c)
			}
			b.Unlock()
		}
	}
}

func (btm *BlockTxMonitorMem) Shutdown() {
	btm.Lock()
	defer btm.Unlock()
	for h, b := range btm.byBlockHeight {
		b.Lock()
		delete(btm.byBlockHeight, h)
		for _, c := range b.waiters {
			close(c)
		}
		b.Unlock()
	}
	for id, b := range btm.byBlockId {
		b.Lock()
		delete(btm.byBlockId, id)
		for _, c := range b.waiters {
			close(c)
		}
		b.Unlock()
	}
	for id, b := range btm.byTxId {
		b.Lock()
		delete(btm.byTxId, id)
		for _, c := range b.waiters {
			close(c)
		}
		b.Unlock()
	}
	btm.byBlockId = nil
	btm.byBlockHeight = nil
	btm.byTxId = nil
}
