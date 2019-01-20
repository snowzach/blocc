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
	nextExpires   time.Time
	sync.Mutex
}

func NewBlockTxMonitorMem() *BlockTxMonitorMem {

	btm := &BlockTxMonitorMem{
		byBlockId:     make(map[string]*BlockTxMonitorMemBlock),
		byBlockHeight: make(map[int64]*BlockTxMonitorMemBlock),
		byTxId:        make(map[string]*BlockTxMonitorMemTx),
		nextExpires:   time.Time{}, // Zero Value
	}

	// Expire anything waiting
	go func() {
		for {
			btm.Lock()
			now := time.Now()
			// Is anything ready to expire
			if !btm.nextExpires.IsZero() && now.Sub(btm.nextExpires) > 0 {
				btm.nextExpires = time.Time{} // Zero Value - and then find the new one

				// Scan the blocks for the expiring by height
				for h, b := range btm.byBlockHeight {
					b.Lock()
					// Remove any expired blocks
					if now.Sub(b.expires) > 0 {
						delete(btm.byBlockHeight, h)
						for _, c := range b.waiters {
							close(c)
						}
					} else {
						// If nextExpires is not set or this one is expiring before the existing nextExpires, mark it next
						if btm.nextExpires.IsZero() || btm.nextExpires.Sub(b.expires) > 0 {
							btm.nextExpires = b.expires
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
					} else {
						// If nextExpires is not set or this one is expiring before the existing nextExpires, mark it next
						if btm.nextExpires.IsZero() || btm.nextExpires.Sub(b.expires) > 0 {
							btm.nextExpires = b.expires
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
					} else {
						// If nextExpires is not set or this one is expiring before the existing nextExpires, mark it next
						if btm.nextExpires.IsZero() || btm.nextExpires.Sub(b.expires) > 0 {
							btm.nextExpires = b.expires
						}
					}
					b.Unlock()
				}

			}
			btm.Unlock()
			time.Sleep(time.Minute)
		}
	}()

	return btm

}

// blockChan will create a channel and immediately return a value and close
func blockChan(b *Block) <-chan *Block {
	c := make(chan *Block)
	if b != nil {
		go func() {
			c <- b
			close(c)
		}()
	} else {
		close(c)
	}
	return c
}

// txChan will create a channel and immediately return a value and close
func txChan(b *Tx) <-chan *Tx {
	c := make(chan *Tx)
	if b != nil {
		go func() {
			c <- b
			close(c)
		}()
	} else {
		close(c)
	}
	return c
}

// Wait for the block id
func (btm *BlockTxMonitorMem) WaitForBlockId(blockId string, expires time.Time) <-chan *Block {
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
		c := make(chan *Block)
		b.waiters = append(b.waiters, c)
		return c
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: expires,
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byBlockId[blockId] = b
	c := make(chan *Block)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
}

// Wait for the block height
func (btm *BlockTxMonitorMem) WaitForBlockHeight(height int64, expires time.Time) <-chan *Block {
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
		c := make(chan *Block)
		b.waiters = append(b.waiters, c)
		return c
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: expires,
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byBlockHeight[height] = b
	c := make(chan *Block)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
}

// Wait for the txId
func (btm *BlockTxMonitorMem) WaitForTxId(txId string, expires time.Time) <-chan *Tx {
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
		c := make(chan *Tx)
		b.waiters = append(b.waiters, c)
		return c
	}

	// We don't have record of this block yet
	b := &BlockTxMonitorMemTx{
		waiters: make([]chan *Tx, 0),
		expires: expires,
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer btm.Unlock()

	btm.byTxId[txId] = b
	c := make(chan *Tx)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
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
		// Record height
		b.block = block
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

		// Make sure we have a nextExpires value and that the new one is greater than the old one
		if btm.nextExpires.IsZero() || btm.nextExpires.Sub(expires) > 0 {
			btm.nextExpires = expires
		}
	}

	if block.Height >= 0 {
		// Do we have this block or have other waiters
		if b, ok := btm.byBlockHeight[block.Height]; ok {
			b.Lock()
			// Record height
			b.block = block
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

			// Make sure we have a nextExpires value and that the new one is greater than the old one
			if btm.nextExpires.IsZero() || btm.nextExpires.Sub(expires) > 0 {
				btm.nextExpires = expires
			}
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

		// Make sure we have a nextExpires value and that the new one is greater than the old one
		if btm.nextExpires.IsZero() || btm.nextExpires.Sub(expires) > 0 {
			btm.nextExpires = expires
		}
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
