package btc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
	"github.com/montanaflynn/stats"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
)

type blockStat struct {
	InputMissing bool
	InputValue   int64
	OutputValue  int64
	TxCount      int64

	HasFee bool
	Fee    int64
	MinFee int64
	MaxFee int64

	sync.Mutex
}

// handleBlock is called as the peers send in blocks
func (e *Extractor) handleBlock(wBlk *wire.MsgBlock) {

	// Block shutdown until fully processed to prevent partial blocks
	e.Add(1)
	defer e.Done()

	e.logger.Debugw("Handling Block", "block_id", wBlk.BlockHash().String())

	// Build the blocc.Block
	blk := &blocc.Block{
		Symbol:      Symbol,
		BlockId:     wBlk.BlockHash().String(),
		PrevBlockId: wBlk.Header.PrevBlock.String(),
		Height:      blocc.HeightUnknown,
		Time:        wBlk.Header.Timestamp.UTC().Unix(),
		TxCount:     int64(len(wBlk.Transactions)),
		TxIds:       make([]string, len(wBlk.Transactions), len(wBlk.Transactions)),
		BlockSize:   int64(wBlk.SerializeSize()),
		Status:      blocc.StatusNew,
		Data:        make(map[string]string),
		Metric:      make(map[string]float64),
	}

	// Write the raw block
	if e.blockStoreRaw {
		var r = new(bytes.Buffer)
		wBlk.Serialize(r)
		blk.Raw = r.Bytes()
	}

	// Metrics
	blk.Data["stripped_size"] = cast.ToString(wBlk.SerializeSizeStripped())
	blk.Data["weight"] = cast.ToString((wBlk.SerializeSizeStripped() * (4 - 1)) + wBlk.SerializeSize()) // WitnessScaleFactor = 4
	blk.Data["bits"] = cast.ToString(wBlk.Header.Bits)
	blk.Data["difficulty"] = cast.ToString(e.getDifficultyRatio(wBlk.Header.Bits))
	blk.Data["version"] = cast.ToString(wBlk.Header.Version)
	blk.Data["version_hex"] = fmt.Sprintf("%08x", wBlk.Header.Version)
	blk.Data["merkle_root"] = wBlk.Header.MerkleRoot.String()
	blk.Data["nonce"] = cast.ToString(wBlk.Header.Nonce)

	// WaitGroup while we are parsing transactions in parallel
	var parsingTransactions sync.WaitGroup

	// Get the block height from the block header cache which should remain ahead of fetching the block chain
	if bh, err := e.blockHeaderCache.GetBlockHeaderByBlockId(Symbol, blk.BlockId); bh != nil && err == nil {
		blk.Height = bh.Height
	} else if err == blocc.ErrNotFound {
		// See if we have the previous block header, we can then determine the height from that
		if pbh, err := e.blockHeaderCache.GetBlockHeaderByBlockId(Symbol, blk.PrevBlockId); pbh != nil && err == nil {
			// We determined the height, put it in the cache in case it's not already there (we are current on the block chain)
			blk.Height = pbh.Height + 1
			if err = e.blockHeaderCache.InsertBlockHeader(Symbol, blk.BlockHeader(), e.blockHeaderCacheLifetime); err != nil {
				e.logger.Fatalf("Could not blockHeaderCache.InsertBlockHeader", "error", err)
			}
		}
	} else if err != nil {
		e.logger.Errorw("Could not blockHeaderCache.GetBlockHeaderByBlockId", "error", err)
	}

	// If we can fetch the NextBlockId from the header cache, do so
	if bh, err := e.blockHeaderCache.GetBlockHeaderByPrevBlockId(Symbol, blk.BlockId); err == nil {
		blk.NextBlockId = bh.BlockId
	}

	// If the height of this block is greater than the peer height, update it
	if blk.Height > int64(e.peer.LastBlock()) {
		e.peer.UpdateLastBlockHeight(int32(blk.Height))
	}

	// If we are following the block chain and we know the height, only handle e.blockConcurrent at a time
	// as it can be very memory intensive
	if blk.Height != blocc.HeightUnknown {
		valid := e.validBlockStore.GetValidBlock()
		if valid == nil {
			e.logger.Fatalw("Could not validBlockStore.GetValidBlock", "validNil", valid == nil)
		}
		// Wait until the block chain is caught up to at least this height - e.blockConncurrent
		if blk.Height-e.blockConcurrent > valid.Height {
			select {
			case <-e.blockHeaderTxMon.WaitForBlockHeight(blk.Height-e.blockConcurrent, 2*time.Hour):
			}
		}
	}

	// Build and populate a
	// - Slice of TxIds for the field in the block
	// - A map of TxIds to that the transaction resolve can use to determine if a transaction output->input is in this block
	// - A map of PrevOutPoints for populating the txids from the tx monitor
	prevOutPoints := make(map[string]*blocc.Tx)
	txIdsInThisBlock := make(map[string]struct{})
	for x, wTx := range wBlk.Transactions {
		txId := wTx.TxHash().String()

		// Save the txId in the block as well as a lookup table
		blk.TxIds[x] = txId
		txIdsInThisBlock[txId] = struct{}{}

		// Attempt to fetch the prevOutPoints by txId from the monitor without any wait if we're tracking blocks
		if !blockchain.IsCoinBaseTx(wTx) {
			for _, vin := range wTx.TxIn {
				// Get the prevOutPoint hash
				hash := vin.PreviousOutPoint.Hash.String()
				// If the cache already has it, populate it. If it's missing it will be nil
				prevOutPoints[hash] = <-e.blockHeaderTxMon.WaitForTxId(hash, 0)
			}
		}
	}

	// If we are in the process of parsing the blockchain, this is a signal that the chain is complete below this block
	// This will allow transaction parsing to pause until the chain is caught up if transactions are missing from the cache
	chainCompleteToThisBlock := make(chan struct{})

	// if we have a block we know to be part of our blockchain
	if blk.Height != blocc.HeightUnknown {

		// Monitor the block chain and signal via chainCompleteToThisBlock that we have complete block chain
		if blk.Height == 0 {
			// Genesis block is always complete
			close(chainCompleteToThisBlock)
		} else {
			// Send a signal when this portion of the blockchain is complete
			go func() {
				for {
					select {
					case <-e.blockHeaderTxMon.WaitForBlockHeight(blk.Height-1, e.blockHeaderTxMonBlockWaitTimeout):
						close(chainCompleteToThisBlock)
						return
					case <-time.After(2 * time.Minute):
						e.logger.Infof("Still waiting for block height: %d", blk.Height-1)
					}
				}
			}()
		}

		// Complete the map of previous outputs needed to populate the transactions in this block
		err := e.getPrevOutPoints(prevOutPoints, chainCompleteToThisBlock, txIdsInThisBlock)
		if err != nil {
			// Print a notification if any of them are missing
			for txId, tx := range prevOutPoints {
				if tx == nil {
					e.logger.Warnw("PreviousOutPoint missing",
						"tx_id", txId,
						"blkheight", blk.Height,
						"error", err,
					)
				}
			}
		}

	} else {
		// We are not parsing the blockchain or we could not resolve the height
		close(chainCompleteToThisBlock)
	}

	var blks blockStat

	var txFeeList []int64
	blks.MinFee = int64((^uint64(0)) >> 1) // Max int64 = 9223372036854775807

	// Iterate through and process transactions
	for x, wTx := range wBlk.Transactions {
		// Capture block stats
		blks.TxCount++

		// Parse the transaction
		parsingTransactions.Add(1)
		go func(txHeight int64, t *wire.MsgTx) {
			txs := e.handleTx(blk, txHeight, t, prevOutPoints)

			// Handle block stats from this transaction
			blks.Lock()
			blks.InputValue += txs.InputValue
			blks.OutputValue += txs.OutputValue
			if !txs.Coinbase {
				blks.HasFee = true
				blks.Fee += txs.Fee

				txFeeList = append(txFeeList, txs.Fee)

				if txs.Fee < blks.MinFee {
					blks.MinFee = txs.Fee
				}
				if txs.Fee > blks.MaxFee {
					blks.MaxFee = txs.Fee
				}
			} else {
				blk.Data["coinbase_value"] = cast.ToString(txs.OutputValue)
			}
			if txs.InputMissing {
				blks.InputMissing = true
			}
			blks.Unlock()
			parsingTransactions.Done()
		}(int64(x), wTx)
	}

	// Wait for transactions to complete resolving
	parsingTransactions.Wait()

	// Wait until the block chain is complete up to this block
	<-chainCompleteToThisBlock

	// Store block/tx stats
	blk.Data["input_value"] = cast.ToString(blks.InputValue)
	blk.Data["output_value"] = cast.ToString(blks.OutputValue)
	if blks.InputMissing {
		blk.Data["input_missing"] = cast.ToString(blks.InputMissing)
		blk.Status = blocc.StatusInvalid
	}
	if blks.HasFee {
		blk.Data["fee"] = cast.ToString(blks.Fee)
		quartileFees, err := stats.Quartile(stats.LoadRawData(txFeeList))
		if err != nil {
			blk.Data["fee_min"] = cast.ToString(float32(blks.Fee) / float32(blks.TxCount-1))
		} else {
			blk.Data["fee_min"] = cast.ToString(float32(quartileFees.Q1))
		}
		blk.Data["fee_max"] = cast.ToString(blks.MaxFee)
		if blks.TxCount < 2 {
			// We don't count the coinbase in the fees calculation because it does have a fee associated
			blk.Data["fee_avg"] = cast.ToString(blks.Fee)
		} else {
			//Get median fee. If this fails for some reason, fall back to mean calculation.
			medianFee, err := stats.Median(stats.LoadRawData(txFeeList))
			if err != nil {
				blk.Data["fee_avg"] = cast.ToString(float32(blks.Fee) / float32(blks.TxCount-1))
			} else {
				blk.Data["fee_avg"] = cast.ToString(float32(medianFee))
			}
		}
	} else {
		// No fees, OVER THE LINE! MARK IT ZERO DUDE
		blk.Data["fee"] = "0"
		blk.Data["fee_min"] = "0"
		blk.Data["fee_max"] = "0"
		blk.Data["fee_avg"] = "0"
	}

	e.logger.Infow("Handled Block", "block_id", blk.BlockId, "height", blk.Height)

	// The block is complete, add to the block store and the block monitor
	bh := blk.BlockHeader()
	// This will also update the block store height that it's complete (and transactions are there to match)
	if blk.Height != blocc.HeightUnknown {
		err := e.blockChainStore.InsertBlock(Symbol, blk)
		if err != nil {
			e.logger.Errorw("Could not blockChainStore.InsertBlock", "error", err)
		}

		// Only mark block valid if there are no missing inputs
		if !blks.InputMissing || e.txIgnoreMissingPrevious {
			e.validBlockStore.AddValidBlock(bh)
		}

		// If this is the top block, ensure everything is immediately flushed to disk
		if blk.Height >= int64(e.peer.LastBlock()) {

			// Update the NextBlockId of the previous block to point to this block
			err = e.blockChainStore.UpdateBlock(Symbol, blk.PrevBlockId, "", blk.BlockId, nil, nil)
			if err != nil {
				e.logger.Errorw("Could not update PrevBlock.NextBlockId", "error", err)
			}

			e.logger.Debugw("Flushing blockChainStore blocks and transactions", "block_height", blk.Height, "peer_height", e.peer.LastBlock())
			e.blockChainStore.FlushBlocks(Symbol)
			e.blockChainStore.FlushTransactions(Symbol)
		}
	}
	e.blockHeaderTxMon.AddBlockHeader(bh, e.blockHeaderTxMonBHLifetime)

}

// This converts [][]byte (witnesses) to []string
func parseWitness(in [][]byte) []string {
	ret := make([]string, len(in), len(in))
	for x, y := range in {
		ret[x] = hex.EncodeToString(y)
	}
	return ret
}

// getDifficultyRatio returns the proof-of-work difficulty as a multiple of the
// minimum difficulty using the passed bits field from the header of a block.
func (e *Extractor) getDifficultyRatio(bits uint32) string {
	// The minimum difficulty is the max possible proof-of-work limit bits
	// converted back to a number.  Note this is not the same as the proof of
	// work limit directly because the block difficulty is encoded in a block
	// with the compact form which loses precision.
	max := blockchain.CompactToBig(e.chainParams.PowLimitBits)
	target := blockchain.CompactToBig(bits)
	difficulty := new(big.Rat).SetFrac(max, target)
	return difficulty.FloatString(8)
}
