package blocc

import (
	"sync"
	"time"
)

type bmblock struct {
	block   *Block
	waiters []chan *Block
	expires time.Time
	sync.Mutex
}

type bmonitor struct {
	byId        map[string]*bmblock
	byHeight    map[int64]*bmblock
	nextExpires time.Time
	sync.Mutex
}

func NewBMonitorMemory() *bmonitor {

	bhm := &bmonitor{
		byId:        make(map[string]*bmblock),
		byHeight:    make(map[int64]*bmblock),
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

				// Scan the blocks for the expiring one
				for key, b := range bhm.byId {
					b.Lock()
					// Remove any expired blocks
					if now.Sub(b.expires) > 0 {
						delete(bhm.byId, key)
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

				// Scan the blocks for the expiring one
				for key, b := range bhm.byHeight {
					b.Lock()
					// Remove any expired blocks
					if now.Sub(b.expires) > 0 {
						delete(bhm.byHeight, key)
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
	go func() {
		c <- b
		close(c)
	}()
	return c
}

// Wait for the block id
func (bhm *bmonitor) WaitForBlockId(blockId string, expires time.Time) <-chan *Block {
	bhm.Lock()
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
	b := &bmblock{
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
func (bhm *bmonitor) WaitForBlockHeight(height int64, expires time.Time) <-chan *Block {
	bhm.Lock()
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
	b := &bmblock{
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

func (bhm *bmonitor) AddBlock(block *Block, expires time.Time) {
	bhm.Lock()
	defer bhm.Unlock()

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
		b := &bmblock{
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
			b := &bmblock{
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
