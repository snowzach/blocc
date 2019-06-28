package btc

import (
	"fmt"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

func (e *Extractor) ValidateBlockChain(symbol string, stopAfter int64) (*blocc.BlockHeader, error) {

	// Make sure we have the lastValidBlockHeader
	lastValidBlockHeader, lastError := e.blockChainStore.GetBlockHeaderTopByStatuses(symbol, []string{blocc.StatusValid})
	if lastError != nil {
		return nil, fmt.Errorf("Could not blockChainStore.GetBlockHeaderTopByStatuses: %v", lastError)
	}

	// If there are invalid blocks, there is nothing to do, this needs to be fixed, exit out returning the highest valid block height we have
	blks, err := e.blockChainStore.FindBlocksByStatusAndHeight(symbol, []string{blocc.StatusInvalid}, blocc.HeightUnknown, blocc.HeightUnknown, blocc.BlockIncludeHeader, 0, 1)
	if err != nil && err != blocc.ErrNotFound {
		return nil, fmt.Errorf("Could not blockChainStore.FindBlocksByStatusAndHeight: %v", err)
	} else if err == nil {
		// We found an invalid block, we already have the last valid one, set the error
		lastError = blocc.ErrInvalidBlock
	}

	// Main Validation Loop
	for lastError == nil {

		// Get a chunk of unverified/new blocks
		startHeight := lastValidBlockHeader.GetHeightSafe()
		if startHeight != blocc.HeightUnknown {
			startHeight++ // Start searching after the lastValidBlockHeader
		}
		blks, err = e.blockChainStore.FindBlocksByStatusAndHeight(symbol, []string{blocc.StatusNew}, startHeight, stopAfter, blocc.BlockIncludeHeader, 0, store.CountMax)
		if err != nil && err != blocc.ErrNotFound {
			lastError = fmt.Errorf("Could not blockChainStore.FindBlocksByStatusAndHeight: %v", err)
			break
		} else if err == blocc.ErrNotFound || len(blks) == 0 {
			// All done
			break
		}

		// Loop through the blocks
		for index, blk := range blks {

			// If we're processing more than one block
			if len(blks) > 1 {
				// If this is the last block in a group of blocks, skip it and fetch another chunk of blocks so we can make sure to catch 2 of the same height in a row (block fork)
				if index == len(blks)-1 {
					break // Go around and fetch another group of blocks
				}

				// If this block and the next block have the same height, it's a fork,
				if blk.Height == blks[index+1].Height {
					e.logger.Warnw("Block fork detected in validator", "height", blk.Height)
					// Determine how many blocks to follow in the competing block chain
					forkStopAfter := stopAfter
					// Don't parse more than 10 blocks
					if forkStopAfter-blk.Height > 10 {
						forkStopAfter = blk.Height + 10
					}
					// Validate the winning block chain
					err = e.handleFork(symbol, blk.Height, forkStopAfter)
					// We couldn't determine the best block chain, just give up for now
					if err == blocc.ErrBestChainUnknown {
						e.logger.Warnw("Best block chain unknown in validator", "height", blk.Height)
						lastError = blocc.ErrBestChainUnknown
						break
					} else if err != nil { // There's some other fatal issue
						return nil, err
					}
					// We have marked competing block chains as orphans
					// Start fetching blocks again from this height to handle completed chain
					break
				}
			}

			// Check for missing block height
			if lastValidBlockHeader != nil && blk.Height != lastValidBlockHeader.Height+1 {
				e.logger.Warnw("Block Missing", "height", blk.Height, "expected", lastValidBlockHeader.Height+1)
				lastError = blocc.ErrMissingBlock
				break
			}

			// For the block, check that all of the transactions exist. Just compare TxCount with the number of stored transactions
			txCount, err := e.blockChainStore.GetTxCountByBlockId(symbol, blk.BlockId, false)
			if err != nil {
				return nil, fmt.Errorf("Could not blockChainStore.GetTxCountByBlockId:%v", err)
			}

			// Do the TxCounts match up with the database
			if txCount != blk.TxCount {
				// There are missing transactions
				err = e.blockChainStore.UpdateBlock(symbol, blk.BlockId, blocc.StatusInvalid, "", nil, nil)
				if err != nil {
					return nil, fmt.Errorf("Could not blockChainStore.UpdateBlock:%v", err)
				}
				e.logger.Warnw("TxCount Mistmatch", "block", blk, "expected", blk.TxCount, "found", txCount)
				lastError = blocc.ErrInvalidBlock
				break
			}

			// Since it's possible there was a re-org, update the lastValidBlock.NextBlockId with the new blocks BlockId
			if lastValidBlockHeader != nil && lastValidBlockHeader.Height+1 == blk.Height {
				err = e.blockChainStore.UpdateBlock(symbol, lastValidBlockHeader.BlockId, "", blk.BlockId, nil, nil)
				if err != nil {
					return nil, fmt.Errorf("Could not blockChainStore.UpdateBlock PrevBlock.NextBlockId:%v", err)
				}
			}

			// Set the next valid block
			lastValidBlockHeader = blk.BlockHeader()
			err = e.blockChainStore.UpdateBlock(symbol, blk.BlockId, blocc.StatusValid, "", nil, nil)
			if err != nil {
				return nil, fmt.Errorf("Could not blockChainStore.UpdateBlock:%v", err)
			}
		}
	}

	// Flush changes to disk
	err = e.blockChainStore.FlushBlocks(symbol)
	if err != nil {
		return lastValidBlockHeader, fmt.Errorf("Could not blockChainStore.FlushBlocks:%v", err)
	}

	// Delete any orphaned blocks and transactions
	if lastValidBlockHeader.GetHeightSafe() != blocc.HeightUnknown {
		orphans, err := e.blockChainStore.FindBlocksByStatusAndHeight(symbol, []string{blocc.StatusOrphaned}, blocc.HeightUnknown, lastValidBlockHeader.GetHeightSafe(), blocc.BlockIncludeHeader, 0, store.CountMax)
		if err != nil && err != blocc.ErrNotFound {
			return lastValidBlockHeader, fmt.Errorf("Could not blockChainStore.FindBlocksByStatusAndHeight: %v", err)
		} else if err == nil && len(orphans) > 0 {
			for _, blk := range orphans {
				err = e.blockChainStore.DeleteBlockByBlockId(symbol, blk.BlockId)
				if err != nil {
					return lastValidBlockHeader, fmt.Errorf("Could not blockChainStore.DeleteBlockById:%v", err)
				}
				err = e.blockChainStore.DeleteTransactionsByBlockIdAndTime(symbol, blk.BlockId, nil, nil)
				if err != nil {
					return lastValidBlockHeader, fmt.Errorf("Could not blockChainStore.DeleteTransactionsByBlockIdAndTime:%v", err)
				}
			}
		}

		// Delete any old blocks still in the mempool/blockChainStore longer than txPoolLifetime
		err = e.blockChainStore.DeleteTransactionsByBlockIdAndTime(symbol, blocc.BlockIdMempool, nil, blocc.ParseUnixTime(-int64(e.txPoolLifetime.Seconds())))
		if err != nil {
			return lastValidBlockHeader, fmt.Errorf("Could not blockChainStore.DeleteTransactionsByBlockIdAndTime:%v", err)
		}
	}

	return lastValidBlockHeader, lastError
}

// This will follow chains and orhpan any losing block chains
func (e *Extractor) handleFork(symbol string, height int64, stopAfter int64) error {

	e.logger.Infof("Handling %s fork at block_height: %d SA:%d", symbol, height, stopAfter)

	// The root of the block chain
	root := make(map[string]*blocc.Block)
	// The blocks indexed by prevBlockId
	blksByPrevBlockId := make(map[string]*blocc.Block)

	// A -- B -- C      = 3
	// D -- E -- F -- G = 4
	var calcLength func(blockId string) int64
	calcLength = func(blockId string) int64 {
		if b, ok := blksByPrevBlockId[blockId]; ok {
			return calcLength(b.BlockId) + 1
		}
		return 1
	}

	// Fetch all the blocks at this height and above
	blocks, err := e.blockChainStore.FindBlocksByStatusAndHeight(symbol, []string{blocc.StatusNew}, height, stopAfter, blocc.BlockIncludeHeader, 0, store.CountMax)
	if err != nil {
		return fmt.Errorf("Could not blockChainStore.FindBlocksByStatusAndHeight:%v", err)
	}

	// Put the blocks into the root and blksByPrevBlockId lookup table
	for _, blk := range blocks {
		if blk.Height == height {
			root[blk.BlockId] = blk
		}
		blksByPrevBlockId[blk.PrevBlockId] = blk
	}

	// Calculate the best chain
	var bestBlockId string
	var bestChainLength int64
	var bestChainLengthCount int
	for blockId, _ := range root {

		chainLength := calcLength(blockId)
		e.logger.Warnf("Checking block_id:%s length:%d", blockId, chainLength)

		if chainLength == bestChainLength {
			// We somehow have a duplicate best chain length
			bestChainLengthCount++
		} else if chainLength > bestChainLength {
			bestChainLength = chainLength
			bestChainLengthCount = 1
			bestBlockId = blockId
		}
	}

	// We have two competing chains of equal length
	if bestChainLengthCount > 1 {
		return blocc.ErrBestChainUnknown
	}

	// Mark any losing blocks as orphaned
	for blockId, _ := range root {
		e.logger.Warnf("Checking root: %s", blockId)

		if blockId != bestBlockId {
			blk, found := root[blockId]
			for found {
				e.logger.Warnf("Marking block orphaned: %s", blk.BlockId)
				err = e.blockChainStore.UpdateBlock(symbol, blk.BlockId, blocc.StatusOrphaned, "", nil, nil)
				if err != nil {
					return fmt.Errorf("Could not blockChainStore.UpdateBlock:%v", err)
				}
				// Get the next block in the chain or break when done
				blk, found = blksByPrevBlockId[blk.BlockId]
			}
		}
	}

	// Ensure changes are flushed to disk
	err = e.blockChainStore.FlushBlocks(symbol)
	if err != nil {
		return fmt.Errorf("Could not blockChainStore.FlushBlocks:%v", err)
	}

	return nil

}

// ResolveTxInputs will handle automatically resolving any missing inputs from transactions and updating them in the database
func (e *Extractor) ResolveTxInputs(symbol string, blockId string) error {

	// Find all transaction with missing inputs
	txs, err := e.blockChainStore.FindTxs(symbol, nil, blockId, nil, blocc.TxFilterIncompleteTrue, nil, nil, blocc.TxIncludeIn, 0, store.CountMax)
	if err != nil {
		return fmt.Errorf("Could not blockChainStore.FindTx:%v", err)
	}

	for _, tx := range txs {

		e.logger.Infow("Resolving Tx Inputs", "tx_id", tx.TxId)

		prevOutPoints := make(map[string]*blocc.Tx)
		for _, in := range tx.In {
			// It's missing
			if in.Out == nil {
				// If the cache has it now, populate it. If it's missing it will be nil with zero wait
				prevOutPoints[in.TxId] = <-e.blockHeaderTxMon.WaitForTxId(in.TxId, 0)
			}
		}

		// Find all those previous outputs that we have
		err = e.getPrevOutPoints(prevOutPoints, nil, map[string]struct{}{})
		if err != nil {
			return fmt.Errorf("Could not getPrevOutPoints:%v", err)
		}

		// Populate the missing transactions
		tx.Incomplete = false
		for _, in := range tx.In {
			// It's missing
			if in.Out == nil {
				// If we found it, store it in out
				if prevTx := prevOutPoints[in.TxId]; prevTx != nil && int64(len(prevTx.Out)) > in.Height {
					in.Out = prevTx.Out[in.Height]
				} else {
					// It's still missing
					tx.Incomplete = true
				}
			}
		}

		// We resolved this transaction fully
		if !tx.Incomplete {
			// This will Upsert the transaction and ONLY update the inputs without modifying the rest of the transaction in case it has hit a block
			err = e.blockChainStore.UpsertTransaction(symbol, &blocc.Tx{
				TxId:       tx.TxId,
				In:         tx.In,
				Incomplete: tx.Incomplete,
			})
		} else {
			e.logger.Warnw("Could not resolve all Tx inputs", "tx_id", tx.TxId)
		}
	}
	return nil
}
