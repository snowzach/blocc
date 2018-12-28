package blocc

import (
	"sync"
	"time"
)

type BlockMonitorMemBlock struct {
	block   *Block
	waiters []chan *Block
	expires time.Time
	sync.Mutex
}

type BlockMonitorMem struct {
	byId        map[string]*BlockMonitorMemBlock
	byHeight    map[int64]*BlockMonitorMemBlock
	nextExpires time.Time
	sync.Mutex
}

func NewBlockMonitorMem() *BlockMonitorMem {

	bhm := &BlockMonitorMem{
		byId:        make(map[string]*BlockMonitorMemBlock),
		byHeight:    make(map[int64]*BlockMonitorMemBlock),
		nextExpires: time.Time{}, // Zero Value
	}

	// Expire anything waiting
	go func() {
		for {
			bhm.Lock()
			now := time.Now()
			// Is anything ready to expire
			if !bhm.nextExpires.IsZero() && now.Sub(bhm.nextExpires) > 0 {
				bhm.nextExpires = time.Time{} // Zero Value - and then find the new one

				// Scan the blocks for the expiring by height
				for h, b := range bhm.byHeight {
					b.Lock()
					// Remove any expired blocks
					if now.Sub(b.expires) > 0 {
						delete(bhm.byHeight, h)
						for _, c := range b.waiters {
							close(c)
						}
					} else {
						// If nextExpires is not set or this one is expiring before the existing nextExpires, mark it next
						if bhm.nextExpires.IsZero() || bhm.nextExpires.Sub(b.expires) > 0 {
							bhm.nextExpires = b.expires
						}
					}
					b.Unlock()
				}

				// Scan the blocks for the expiring by Id (in case we never got block)
				for id, b := range bhm.byId {
					b.Lock()
					// Remove any expired blocks
					if now.Sub(b.expires) > 0 {
						delete(bhm.byId, id)
						for _, c := range b.waiters {
							close(c)
						}
					} else {
						// If nextExpires is not set or this one is expiring before the existing nextExpires, mark it next
						if bhm.nextExpires.IsZero() || bhm.nextExpires.Sub(b.expires) > 0 {
							bhm.nextExpires = b.expires
						}
					}
					b.Unlock()
				}

			}
			bhm.Unlock()
			time.Sleep(time.Minute)
		}
	}()

	return bhm

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

// Wait for the block id
func (bhm *BlockMonitorMem) WaitForBlockId(blockId string, expires time.Time) <-chan *Block {
	bhm.Lock()

	// If shutdown
	if bhm.byId == nil {
		bhm.Unlock()
		return blockChan(nil)
	}

	// Do we have this block or have other waiters
	if b, ok := bhm.byId[blockId]; ok {
		b.Lock()
		defer b.Unlock()
		defer bhm.Unlock()
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
	b := &BlockMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: expires,
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer bhm.Unlock()

	bhm.byId[blockId] = b
	c := make(chan *Block)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
}

// Wait for the block height
func (bhm *BlockMonitorMem) WaitForBlockHeight(height int64, expires time.Time) <-chan *Block {
	bhm.Lock()

	// If shutdown
	if bhm.byId == nil {
		bhm.Unlock()
		return blockChan(nil)
	}

	// Do we have this block or have other waiters
	if b, ok := bhm.byHeight[height]; ok {
		b.Lock()
		defer b.Unlock()
		defer bhm.Unlock()
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
	b := &BlockMonitorMemBlock{
		waiters: make([]chan *Block, 0),
		expires: expires,
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer bhm.Unlock()

	bhm.byHeight[height] = b
	c := make(chan *Block)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
}

func (bhm *BlockMonitorMem) AddBlock(block *Block, expires time.Time) {
	bhm.Lock()
	defer bhm.Unlock()

	// We're shutdown
	if bhm.byId == nil {
		return
	}

	// Do we have this block or have other waiters
	if b, ok := bhm.byId[block.BlockId]; ok {
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
		b := &BlockMonitorMemBlock{
			block:   block,
			expires: expires,
		}
		bhm.byId[block.BlockId] = b

		// Make sure we have a nextExpires value and that the new one is greater than the old one
		if bhm.nextExpires.IsZero() || bhm.nextExpires.Sub(expires) > 0 {
			bhm.nextExpires = expires
		}
	}

	if block.Height >= 0 {
		// Do we have this block or have other waiters
		if b, ok := bhm.byHeight[block.Height]; ok {
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
			b := &BlockMonitorMemBlock{
				block:   block,
				expires: expires,
			}
			bhm.byHeight[block.Height] = b

			// Make sure we have a nextExpires value and that the new one is greater than the old one
			if bhm.nextExpires.IsZero() || bhm.nextExpires.Sub(expires) > 0 {
				bhm.nextExpires = expires
			}
		}
	}

}

func (bhm *BlockMonitorMem) ExpireBelowBlockHeight(height int64) {
	bhm.Lock()
	defer bhm.Unlock()
	for h, b := range bhm.byHeight {
		if h <= height {
			b.Lock()
			delete(bhm.byHeight, h)
			delete(bhm.byId, b.block.BlockId)
			for _, c := range b.waiters {
				close(c)
			}
			b.Unlock()
		}
	}
}

func (bhm *BlockMonitorMem) Shutdown() {
	bhm.Lock()
	defer bhm.Unlock()
	for h, b := range bhm.byHeight {
		b.Lock()
		delete(bhm.byHeight, h)
		for _, c := range b.waiters {
			close(c)
		}
		b.Unlock()
	}
	for id, b := range bhm.byId {
		b.Lock()
		delete(bhm.byId, id)
		for _, c := range b.waiters {
			close(c)
		}
		b.Unlock()
	}
	bhm.byHeight = nil
	bhm.byId = nil
}
