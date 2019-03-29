package blocc

import (
	"sync"
)

type validBlockStoreMem struct {
	store map[int64]*BlockHeader
	valid *BlockHeader
	sync.Mutex
}

func NewValidBlockStoreMem() *validBlockStoreMem {
	return &validBlockStoreMem{
		store: make(map[int64]*BlockHeader),
		valid: nil,
	}
}

// AddValidBlock considers the current header for height of the block chain
func (vbsm *validBlockStoreMem) AddValidBlock(bh *BlockHeader) {

	vbsm.Lock()
	defer vbsm.Unlock()

	// No current best
	if vbsm.valid == nil {
		vbsm.valid = bh
	}

	// Is the current valid block already the same or better than this new block?
	// This should only be possible during a chain re-org so we're not going to mess with the current best block
	// The block validator should detect this issue and reset the valid block to the winning block chain once it's time
	if vbsm.valid.Height >= bh.Height {
		return
	}

	// Follow all the blocks we have until we get to the best valid block
	for {
		// If our block is the new valid block, make it so
		if bh != nil && bh.Height == vbsm.valid.Height+1 {
			vbsm.valid = bh
			bh = nil // A signal it's been used, continue to look for better valid blocks
		}
		// Check to see if we have the next valid block
		if newvb, ok := vbsm.store[vbsm.valid.Height+1]; ok {
			vbsm.valid = newvb
			delete(vbsm.store, newvb.Height)
		} else {
			// We've traversed as far as we can, if we haven't used the block we passed in, store it for later use
			if bh != nil {
				vbsm.store[bh.Height] = bh
			}
			break
		}
	}

}

// SetValidBlock forces the current block chain height, it will also reset any current blocks
func (vbsm *validBlockStoreMem) SetValidBlock(bh *BlockHeader) {
	vbsm.Lock()
	defer vbsm.Unlock()
	vbsm.store = make(map[int64]*BlockHeader)
	vbsm.valid = bh
}

// GetValidBlock will get the current valid block
func (vbsm *validBlockStoreMem) GetValidBlock() *BlockHeader {
	vbsm.Lock()
	defer vbsm.Unlock()
	return vbsm.valid
}
