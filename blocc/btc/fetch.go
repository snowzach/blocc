package btc

import (
	"fmt"
	"time"

	config "github.com/spf13/viper"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/conf"
)

// fetchBlockChain will start fetching blocks until it has the entire block chain
func (e *Extractor) fetchBlockChain() {

	// Print starting message
	valid := e.validBlockStore.GetValidBlock()
	if valid == nil {
		e.logger.Fatalw("Could not validBlockStore.GetValidBlock", "validNil", valid == nil)
	}
	e.logger.Infow("Starting block extraction", "block_start_id", valid.BlockId, "block_start_height", valid.Height)

	var lastValidBlockRequested string
	var lastValidateBlockChain = time.Now()

	for !conf.Stop.Bool() {
		loopStartTime := time.Now()

		// If we're disconnected, attempt to reconnect - this resets the state
		if !e.peer.Connected() {
			// Ensure we're disconnected
			e.peer.Disconnect()
			// Reconnect
			e.logger.Warn("Attempting peer reconnect")
			err := e.Connect()
			if err != nil {
				e.logger.Errorw("Error reconnecting to peer. Sleeping/Retry", "valid", valid, "error", err)
				time.Sleep(time.Minute)
				continue
			}
			lastValidBlockRequested = ""
		}

		// Get the current valid height of the block chain
		valid := e.validBlockStore.GetValidBlock()
		if valid == nil {
			e.logger.Fatalw("Could not validBlockStore.GetValidBlock", "validNil", valid == nil)
		}
		// Ensure the valid header is in the blockHeaderTxMonitor
		e.blockHeaderTxMon.AddBlockHeader(valid, e.blockHeaderTxMonBHLifetime)

		// How tall is our peer block height
		peerBlockHeight := int64(e.peer.LastBlock())

		// Validate the block chain up to this point
		if loopStartTime.Sub(lastValidateBlockChain) > e.blockValidationInterval {

			e.logger.Info("Validating/Flushing BlockChainStore")
			lastValidateBlockChain = loopStartTime

			// If we are caught up on the block chain, but the last processed block was invalid, there is a state issue
			// between the bitcoind node and blocc. Disconnect and reset the state, and re-download all the recent blocks
			e.RLock()
			if valid.Height >= peerBlockHeight-1 && e.lastBlockHeightUnknown {
				e.logger.Info("Unknown height blocks detected on caught up BlockChain. Resetting Connection.")
				e.RUnlock()
				e.peer.Disconnect()
				time.Sleep(time.Second)
				continue
			}
			e.RUnlock()

			// Check for the highest valid block
			var newValid *blocc.BlockHeader
			var err error

			if valid.Height < peerBlockHeight-e.blockValidationHeightDelta {
				// If we are over 100 blocks behind, we can assume the block chain is accurate without forks
				// Flushing the blocks and transactions would have produced an error if there was a problem
				// Mark anything not marked as invalid as valid
				newValid, err = e.validateBlocksSimple(valid.Height)
			} else {
				// Validate the more recent block chain completely up to peerBlockheight - blockValidationHeightHoldOff
				newValid, err = e.ValidateBlockChain(Symbol, peerBlockHeight-e.blockValidationHeightHoldOff)
			}

			if err == blocc.ErrInvalidBlock || err == blocc.ErrMissingBlock || err == blocc.ErrMissingInput {
				e.logger.Errorf("Block chain error detected", "error", err, "newValid", newValid)
				// Attempt to reset the fetching of the block chain to re-fetch either missing blocks/transactions
				// or to resolve forks in the block chain
				e.validBlockStore.SetValidBlock(newValid)
				// We can remove any headers above the valid block in case we are retrying blocks
				e.blockHeaderTxMon.ExpireBlockHeadersAboveBlockHeight(newValid.Height)
				// Restart the loop with a new valid block
				continue
			} else if err == blocc.ErrBestChainUnknown {
				// There's nothing we can do but wait for additional chain data
				e.logger.Warn("Fork in blockchain detected with equal heights, could not determine best chain")
			} else if err != nil {
				e.logger.Fatalw("Blockchain validation error", "error", err)
			}
		}

		// If we're less than one block behind, we just sit and wait for blocks to come in
		if valid.Height >= peerBlockHeight-1 {
			time.Sleep(time.Second)
			continue
		}

		// Get the current height of our header cache
		topHeader, err := e.blockHeaderCache.GetTopBlockHeader(Symbol)
		if err != nil || topHeader == nil {
			e.logger.Fatalw("Could not blockHeaderCache.GetTopBlockHeader", "error", err, "validNil", valid == nil)
		}

		// If our header cache height is less than our peer block height - 1
		// Make sure we keep our header cache well ahead of our block chain follower unless we're caught up with the block chain
		if topHeader.Height < peerBlockHeight-1 && topHeader.Height-valid.Height < config.GetInt64("extractor.btc.block_headers_request_count") {
			e.logger.Infow("Fetching headers", "block", valid, "header", topHeader)
			gotHeaders, err := e.RequestHeaders(topHeader.BlockId, "0")
			if err != nil {
				e.logger.Errorw("Could not request headers", "block", valid, "header", topHeader, "error", err)
				time.Sleep(1)
				continue
			}

			// We don't have enough headers to complete this pass, fetch headers and wait until they arrive
			if topHeader.Height-valid.Height < config.GetInt64("extractor.btc.block_request_count") {
				// Allow for timeout
				select {
				case <-gotHeaders:
				case <-time.After(5 * time.Minute):
					e.logger.Infow("Timeout waiting for headers", "block", valid, "header", topHeader)
				}
				continue
			}
		}

		// Expire other blocks and headers below current peer height - blockValidationHeightDelta or the valid height whichever is less
		// as this is the range the valid hight could be rolled back to if there is a problem and we want to ensure we have all the necessary headers
		if peerBlockHeight-e.blockValidationHeightDelta <= valid.Height {
			e.blockHeaderCache.ExpireBlockHeaderBelowHeight(Symbol, peerBlockHeight-e.blockValidationHeightDelta-1)
			e.blockHeaderTxMon.ExpireBlockHeadersBelowBlockHeight(peerBlockHeight - e.blockValidationHeightDelta - 1)
		} else {
			e.blockHeaderCache.ExpireBlockHeaderBelowHeight(Symbol, valid.Height)
			e.blockHeaderTxMon.ExpireBlockHeadersBelowBlockHeight(valid.Height)
		}

		// This will fetch blocks, the first block will be the one after this one and will return extractor.btc.block_request_count (500) blocks
		expectedBlockHeight := valid.Height + config.GetInt64("extractor.btc.block_request_count")
		// If we are caught up, we can only catch up to the peer
		if expectedBlockHeight > peerBlockHeight {
			expectedBlockHeight = peerBlockHeight
			// We need to re-handle the top blocks most-likely. Ensure they are no longer in the blockHeaderTxMonitor so they will be re-handled
			e.blockHeaderTxMon.ExpireBlockHeadersAboveBlockHeight(valid.Height)
			e.blockHeaderTxMon.ExpireBlockHeadersHeightUnknown()
		}
		e.logger.Infow("Requesting blocks", "from", valid, "peer_block_height", peerBlockHeight, "expected_height", expectedBlockHeight)

		// If it gets to the point where we have re-requested the same block, the peer isn't going to send it as it views it as a duplicate request
		// We need to disconnect and re-connect to reset the peers connection state
		if valid.BlockId == lastValidBlockRequested {
			e.logger.Warnw("Attempted to re-request the same blocks - restting peer connection", "from", valid)
			// We had to re-request a chunk of blocks, clear everything out of the blockTxMonitor
			e.blockHeaderTxMon.ExpireBlockHeadersAboveBlockHeight(valid.Height)
			// This might be because something didn't make it to redis/the valid block store
			// Clear and reset the valid block store to the last valid block
			e.validBlockStore.SetValidBlock(valid)
			// Disconnect the peer, wait and reconnect above
			e.Disconnect()
			time.Sleep(time.Minute)
			continue
		}

		// Make the block request
		e.RequestBlocks(valid.BlockId, "0")
		lastValidBlockRequested = valid.BlockId

		// If we have no block for extractor.btc.block_timeout, assume it stalled
		blockTimeout := make(chan struct{})
		go func() {
			for {
				select {
				case <-blockTimeout:
					return
				case <-time.After(time.Minute):
					// Still working
				}
				// Every minute, check if we have received a block in the blockHeaderMonitor in the last extractor.btc.block_timeout
				// If not, trigger a blockTimeout/re-request
				if time.Now().Sub(e.blockHeaderTxMon.LastBlockHeaderWithHeightTime()) > config.GetDuration("extractor.btc.block_timeout") {
					close(blockTimeout)
					return
				}
			}
		}()

		select {
		// Otherwise, wait for the the expectedBlockHeight in the stream of blocks
		case blk := <-e.blockHeaderTxMon.WaitForBlockHeight(expectedBlockHeight, config.GetDuration("extractor.btc.block_request_timeout")):
			close(blockTimeout) // Cancel the block timeout monitor, we made it to the next group of blocks
			if blk == nil {
				valid = e.validBlockStore.GetValidBlock()
				e.logger.Errorw("Timeout following blockchain", "expected", expectedBlockHeight, "got", valid)
				continue
			} else {
				e.logger.Infow("Block Chain Stats",
					"height", blk.Height,
					"rate(/h)", 500.0/(time.Now().Sub(loopStartTime).Hours()),
					"rate(/m)", 500.0/time.Now().Sub(loopStartTime).Minutes(),
					"rate(/s)", 500.0/time.Now().Sub(loopStartTime).Seconds(),
					"eta", (time.Duration(float64(peerBlockHeight-expectedBlockHeight)/(config.GetFloat64("extractor.btc.block_request_count")/time.Now().Sub(loopStartTime).Seconds())) * time.Second).String(),
				)
			}
		// No block for extractor.btc.block_timeout
		case <-blockTimeout:
			valid := e.validBlockStore.GetValidBlock()
			if valid == nil {
				e.logger.Fatalw("Could not validBlockStore.GetValidBlock", "validNil", valid == nil)
			}
			e.logger.Errorw("Block timeout", "valid", valid, "expected_height", expectedBlockHeight)

		// We're exiting
		case <-conf.Stop.Chan():
		}

	}

}

// After a successful Blocks and Transactions flush without error, we will mark any blocks valid
func (e *Extractor) validateBlocksSimple(height int64) (*blocc.BlockHeader, error) {

	// Ensure the blockChainStore is operating properly and flush everything without error
	if err := e.blockChainStore.FlushBlocks(Symbol); err != nil {
		// Make sure we flush as much as we can before we die
		e.blockChainStore.FlushTransactions(Symbol)
		return nil, fmt.Errorf("blockChainStore.FlushBlocks error:%v", err)
	}
	if err := e.blockChainStore.FlushTransactions(Symbol); err != nil {
		return nil, fmt.Errorf("blockChainStore.FlushTransactions error:%v", err)
	}

	// If there are invalid blocks up until height, there is nothing to do, this needs to be fixed, exit out returning the highest valid block height we have
	blks, err := e.blockChainStore.FindBlocksByStatusAndHeight(Symbol, []string{blocc.StatusInvalid}, blocc.HeightUnknown, height, blocc.BlockIncludeHeader, 0, 1)
	if err != nil && err != blocc.ErrNotFound {
		return nil, fmt.Errorf("Could not blockChainStore.FindBlocksByStatusAndHeight: %v", err)
	} else if err == nil {
		// We found an invalid block, the last non-valid block should be one before this one
		lastValidBlockHeader, err := e.blockChainStore.GetBlockHeaderTopByStatuses(Symbol, []string{blocc.StatusValid})
		if err != nil {
			return nil, fmt.Errorf("Could not fetch last valid block from invalid %v blockChainStore.GetBlockHeaderTopByStatuses: %v", blks[0], err)
		}
		return lastValidBlockHeader, blocc.ErrInvalidBlock
	}

	// Update the status through the given height to valid as long as it's marked as new
	err = e.blockChainStore.UpdateBlockStatusByStatusesAndHeight(Symbol, []string{blocc.StatusNew}, blocc.HeightUnknown, height, blocc.StatusValid)
	if err != nil {
		return nil, fmt.Errorf("Could not blockChainStore.UpdateBlockStatusByHeight: %v", err)
	}

	// Successful, don't need to return block header
	return nil, nil
}
