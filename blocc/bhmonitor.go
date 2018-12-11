package blocc

import (
	"sync"
	"time"
)

type bhmblock struct {
	height    int64
	waiters   []chan int64
	timestamp time.Time
	sync.Mutex
}

type bhmonitor struct {
	blocks     map[string]*bhmblock
	lastAgeOut time.Time
	height     int64
	sync.RWMutex
}

func NewBHMonitor() *bhmonitor {

	bhm := &bhmonitor{
		blocks: make(map[string]*bhmblock),
	}

	return bhm

}

// retChan will create a channel and immediately return a value and close
func retChan(height int64) <-chan int64 {
	c := make(chan int64)
	go func() {
		c <- height
		close(c)
	}()
	return c
}

// Get the block height or wait for it
func (bhm *bhmonitor) WaitForBlockHeight(id string) <-chan int64 {
	bhm.RLock()
	// Do we have this block or have other waiters
	if b, ok := bhm.blocks[id]; ok {
		b.Lock()
		defer b.Unlock()
		defer bhm.RUnlock()
		// We already have the height
		if b.height != 0 {
			return retChan(b.height)
		}
		// We don't yet have the height, create a channel
		c := make(chan int64)
		b.waiters = append(b.waiters, c)
		return c
	}

	// Switch to write lock
	bhm.RUnlock()
	bhm.Lock()

	// We don't have record of this block yet
	b := &bhmblock{
		waiters: make([]chan int64, 0),
	}
	b.Lock()

	// Unlock when we're done
	defer b.Unlock()
	defer bhm.Unlock()

	bhm.blocks[id] = b
	c := make(chan int64)
	b.waiters = append(b.waiters, c)

	// Return the channel
	return c
}

func (bhm *bhmonitor) BlockHeight(id string, height int64) {
	bhm.Lock()
	// Do we have this block or have other waiters
	if b, ok := bhm.blocks[id]; ok {
		b.Lock()
		// Record height
		b.height = height
		// Send message to waiters
		for _, w := range b.waiters {
			w <- height
			close(w)
		}
		b.waiters = nil
		b.Unlock()
	} else {
		b := &bhmblock{
			height:    height,
			timestamp: time.Now(),
		}
		bhm.blocks[id] = b
	}

	// Record the height
	if height > bhm.height {
		bhm.height = height
	}

	// Every 10 minutes ageo out the monitor
	if time.Now().Sub(bhm.lastAgeOut) > (10 * time.Minute) {
		for key, b := range bhm.blocks {
			b.Lock()
			// Remove any blocks received more than 60 minutes ago
			if time.Now().Sub(b.timestamp) > 60*time.Minute {
				delete(bhm.blocks, key)
				for _, c := range b.waiters {
					close(c)
				}
			}
			b.Unlock()
		}
	}

	bhm.Unlock()

}
