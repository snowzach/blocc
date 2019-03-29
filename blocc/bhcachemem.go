package blocc

import (
	"sync"
	"time"
)

// This is a very simple, memory backed cache for storing and retrieveing BlockHeaders by BlockId
// It also supports expiring BlockHeaders based on time or block height
// While it does specify an expiration time, it's not actually used as it will be destroyed when the program stops

type bhrec struct {
	bh      *BlockHeader
	expires time.Time
}

type bhc struct {
	cache  map[string]*bhrec
	pcache map[string]*bhrec
	top    *BlockHeader
	sync.RWMutex
}

// This is the in memory block header cache
type BlockHeaderCacheMem struct {
	symbols map[string]*bhc
	sync.RWMutex
}

// This is a very simple, memory backed cache for storing and retrieveing BlockHeaders by BlockId
// It also supports expiring BlockHeaders based on time or block height
// While it does specify an expiration time, it's not actually used as it will be destroyed when the program stops
func NewBlockHeaderCacheMem() *BlockHeaderCacheMem {
	return &BlockHeaderCacheMem{
		symbols: make(map[string]*bhc),
	}
}

// Init sets up the cache
func (bhcm *BlockHeaderCacheMem) Init(symbol string) error {
	bhcm.Lock()
	bhcm.symbols[symbol] = &bhc{
		cache:  make(map[string]*bhrec),
		pcache: make(map[string]*bhrec),
		top:    nil,
	}
	bhcm.Unlock()
	return nil
}

// GetTopBlockHeader returns the top block header
func (bhcm *BlockHeaderCacheMem) GetTopBlockHeader(symbol string) (*BlockHeader, error) {
	bhcm.RLock()
	defer bhcm.RUnlock()
	if b, ok := bhcm.symbols[symbol]; ok {
		b.RLock()
		defer b.RUnlock()
		if b.top == nil {
			return nil, ErrNotFound
		}
		return b.top, nil
	}
	return nil, ErrUnknownSymbol
}

// InsertBlocHeader inserts a block and registers as the top if applicable
func (bhcm *BlockHeaderCacheMem) InsertBlockHeader(symbol string, bh *BlockHeader, expires time.Duration) error {
	bhcm.RLock()
	defer bhcm.RUnlock()
	if b, ok := bhcm.symbols[symbol]; ok {
		b.Lock()
		defer b.Unlock()
		rec := &bhrec{
			bh:      bh,
			expires: time.Now().Add(expires),
		}
		b.cache[bh.BlockId] = rec
		b.pcache[bh.PrevBlockId] = rec
		if b.top == nil || bh.Height >= b.top.Height {
			b.top = bh
		}
		return nil
	}
	return ErrUnknownSymbol
}

// GetBlockHeaderByBlockId fetches a block header by blockId
func (bhcm *BlockHeaderCacheMem) GetBlockHeaderByBlockId(symbol string, blockId string) (*BlockHeader, error) {
	bhcm.RLock()
	defer bhcm.RUnlock()
	if b, ok := bhcm.symbols[symbol]; ok {
		b.RLock()
		defer b.RUnlock()
		if bh, ok := b.cache[blockId]; ok {
			return bh.bh, nil
		} else {
			return nil, ErrNotFound
		}
	}
	return nil, ErrUnknownSymbol
}

// GetBlockHeaderByPrevBlockId fetches a block header by prevBlockId
func (bhcm *BlockHeaderCacheMem) GetBlockHeaderByPrevBlockId(symbol string, prevBlockId string) (*BlockHeader, error) {
	bhcm.RLock()
	defer bhcm.RUnlock()
	if b, ok := bhcm.symbols[symbol]; ok {
		b.RLock()
		defer b.RUnlock()
		if bh, ok := b.pcache[prevBlockId]; ok {
			return bh.bh, nil
		} else {
			return nil, ErrNotFound
		}
	}
	return nil, ErrUnknownSymbol
}

// ExpireBlockHeaderBelowHeight removes any block headers below a certain height
func (bhcm *BlockHeaderCacheMem) ExpireBlockHeaderBelowHeight(symbol string, height int64) error {
	bhcm.RLock()
	defer bhcm.RUnlock()
	if b, ok := bhcm.symbols[symbol]; ok {
		b.Lock()
		defer b.Unlock()

		for blockId, bh := range b.cache {
			if bh.bh.Height < height {
				delete(b.cache, blockId)
			}
		}
		for blockId, bh := range b.pcache {
			if bh.bh.Height < height {
				delete(b.pcache, blockId)
			}
		}
		return nil
	}
	return ErrUnknownSymbol
}
