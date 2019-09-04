package btools

import (
	"sync"
	"time"

	"git.coinninja.net/backend/blocc/blocc"
)

type blockHeaderTxMonitorMemBlockHeader struct {
	bh      *blocc.BlockHeader
	wait    chan struct{}
	expires time.Time
}

type blockHeaderTxMonitorMemTx struct {
	tx      *blocc.Tx
	wait    chan struct{}
	expires time.Time
}

type blockHeaderTxMonitorMem struct {
	byBlockId            map[string]*blockHeaderTxMonitorMemBlockHeader
	byBlockHeight        map[int64]*blockHeaderTxMonitorMemBlockHeader
	byTxId               map[string]*blockHeaderTxMonitorMemTx
	lastBHWithHeightTime time.Time
	sync.Mutex
}

// NewBlockHeaderTxMonitorMem returns a tool for monitoring BlockHeader and Transactions as they come in.
// It allows them to wait for them as they are processed
func NewBlockHeaderTxMonitorMem() *blockHeaderTxMonitorMem {

	btm := &blockHeaderTxMonitorMem{
		byBlockId:     make(map[string]*blockHeaderTxMonitorMemBlockHeader),
		byBlockHeight: make(map[int64]*blockHeaderTxMonitorMemBlockHeader),
		byTxId:        make(map[string]*blockHeaderTxMonitorMemTx),
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

// AddBlockHeader to the monitor
func (btm *blockHeaderTxMonitorMem) AddBlockHeader(bh *blocc.BlockHeader, expires time.Duration) {
	btm.Lock()
	defer btm.Unlock()

	// We're shutdown
	if btm.byBlockId == nil {
		return
	}

	if bh.Height != blocc.HeightUnknown {
		btm.lastBHWithHeightTime = time.Now()
	}

	// Do we have someone already waiting
	if b, ok := btm.byBlockId[bh.BlockId]; ok {
		// Save the block
		b.bh = bh
		b.expires = time.Now().Add(expires)
		// Signal waiters
		closeIfOpen(b.wait)
	} else {
		// We don't, create a record
		b := &blockHeaderTxMonitorMemBlockHeader{
			bh:      bh,
			wait:    make(chan struct{}),
			expires: time.Now().Add(expires),
		}
		close(b.wait) // Close the wait channel
		btm.byBlockId[bh.BlockId] = b
	}

	// We have a block height
	if bh.Height >= 0 {
		// Are there any waiters
		if b, ok := btm.byBlockHeight[bh.Height]; ok {
			// Save the block
			b.bh = bh
			b.expires = time.Now().Add(expires)
			// Signal waiters
			closeIfOpen(b.wait)
		} else {
			b := &blockHeaderTxMonitorMemBlockHeader{
				bh:      bh,
				wait:    make(chan struct{}),
				expires: time.Now().Add(expires),
			}
			close(b.wait) // Close the wait channel
			btm.byBlockHeight[bh.Height] = b
		}
	}

}

// AddTx to the monitor
func (btm *blockHeaderTxMonitorMem) AddTx(tx *blocc.Tx, expires time.Duration) {
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
		t.expires = time.Now().Add(expires)
		// Signal Waiters
		closeIfOpen(t.wait)
	} else {
		t := &blockHeaderTxMonitorMemTx{
			tx:      tx,
			wait:    make(chan struct{}),
			expires: time.Now().Add(expires),
		}
		close(t.wait) // Close the wait channel
		btm.byTxId[tx.TxId] = t
	}
}

// WaitForBlockId uses the monitor to wait for a Block by Id with a timeout
func (btm *blockHeaderTxMonitorMem) WaitForBlockId(blockId string, timeout time.Duration) <-chan *blocc.BlockHeader {
	btm.Lock()

	c := make(chan *blocc.BlockHeader, 1)

	// If shutdown
	if btm.byBlockId == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this block
	b, ok := btm.byBlockId[blockId]
	if !ok {

		// We don't have it, no wait requested, return
		if timeout == 0 {
			btm.Unlock()
			close(c)
			return c
		}

		// We, don't, create a new one
		b = &blockHeaderTxMonitorMemBlockHeader{
			bh:      nil,
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
			c <- b.bh
			close(c)
		// We timed out
		case <-time.After(timeout):
			close(c)
		}
	}()

	return c

}

// WaitForBlockHeight uses the monitor to wait for a Block by height with a timeout
func (btm *blockHeaderTxMonitorMem) WaitForBlockHeight(blockHeight int64, timeout time.Duration) <-chan *blocc.BlockHeader {
	btm.Lock()

	c := make(chan *blocc.BlockHeader, 1)

	// If shutdown
	if btm.byBlockHeight == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this block
	b, ok := btm.byBlockHeight[blockHeight]
	if !ok {

		// We don't have it, no wait requested, return
		if timeout == 0 {
			btm.Unlock()
			close(c)
			return c
		}

		// We, don't, create a new one
		b = &blockHeaderTxMonitorMemBlockHeader{
			bh:      nil,
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
			c <- b.bh
			close(c)
		// It timed out
		case <-time.After(timeout):
			close(c)
		}
	}()

	return c

}

// WaitForTxId uses the monitor to wait for a transaction by Id with a timeout
func (btm *blockHeaderTxMonitorMem) WaitForTxId(txId string, timeout time.Duration) <-chan *blocc.Tx {

	btm.Lock()

	c := make(chan *blocc.Tx, 1)

	// If shutdown
	if btm.byTxId == nil {
		btm.Unlock()
		close(c)
		return c
	}

	// Do we have this tx
	t, ok := btm.byTxId[txId]
	if !ok {

		// We don't have it, no wait requested, return
		if timeout == 0 {
			btm.Unlock()
			close(c)
			return c
		}

		// We, don't, create a new one
		t = &blockHeaderTxMonitorMemTx{
			tx:      nil,
			wait:    make(chan struct{}),
			expires: time.Now().Add(timeout + (5 * time.Second)), // Make it timeout a little after the actual timeout
		}
		btm.byTxId[txId] = t
	}

	// Return it if we have it or not
	if timeout == 0 {
		c <- t.tx
		close(c)
		btm.Unlock()
		return c
	}

	btm.Unlock()

	select {
	// We got the tx or it expired
	case <-t.wait:
		c <- t.tx
		close(c)
	// It timed out
	case <-time.After(timeout):
		c <- nil
	}

	return c

}

// ExpireBlockHeadersBelowBlockHeight removes any block headers from the monitor below a height
func (btm *blockHeaderTxMonitorMem) ExpireBlockHeadersBelowBlockHeight(blockHeight int64) {
	btm.Lock()
	defer btm.Unlock()
	for bh, b := range btm.byBlockHeight {
		if bh < blockHeight && bh != blocc.HeightUnknown {
			closeIfOpen(b.wait)
			delete(btm.byBlockHeight, bh)
			if b.bh != nil {
				delete(btm.byBlockId, b.bh.BlockId)
			}
		}
	}
}

// ExpireBlockHeadersBelowBlockHeight removes any block headers from the monitor above a height
func (btm *blockHeaderTxMonitorMem) ExpireBlockHeadersAboveBlockHeight(blockHeight int64) {
	btm.Lock()
	defer btm.Unlock()
	for bh, b := range btm.byBlockHeight {
		if bh > blockHeight {
			closeIfOpen(b.wait)
			delete(btm.byBlockHeight, bh)
			if b.bh != nil {
				delete(btm.byBlockId, b.bh.BlockId)
			}
		}
	}
}

// ExpireBlockHeadersHeightUnknown will remove any blocks with unknown height
func (btm *blockHeaderTxMonitorMem) ExpireBlockHeadersHeightUnknown() {
	btm.Lock()
	defer btm.Unlock()
	for blockId, b := range btm.byBlockId {
		if b.bh != nil && b.bh.Height == blocc.HeightUnknown {
			closeIfOpen(b.wait)
			delete(btm.byBlockId, blockId)
		}
	}
}

// LastBlockHeaderWithHeightTime returns the time the last block header was added
func (btm *blockHeaderTxMonitorMem) LastBlockHeaderWithHeightTime() time.Time {
	btm.Lock()
	defer btm.Unlock()
	return btm.lastBHWithHeightTime
}

// Shutdown shuts down the monitor
func (btm *blockHeaderTxMonitorMem) Shutdown() {
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
