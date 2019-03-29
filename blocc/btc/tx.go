package btc

import (
	"bytes"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
)

type txStat struct {
	Coinbase     bool
	InputMissing bool
	InputValue   int64
	OutputValue  int64
	Fee          int64
}

// handleTx is called to handle transaction both when sent from the peer as part of the mempool or when parsing block
func (e *Extractor) handleTx(blk *blocc.Block, txHeight int64, wTx *wire.MsgTx, prevOutPoints map[string]*blocc.Tx) *txStat {

	// Build the blocc.Tx
	tx := &blocc.Tx{
		Symbol: Symbol,
		TxId:   wTx.TxHash().String(),
		Height: txHeight,
		TxSize: int64(wTx.SerializeSize()),
		Data:   make(map[string]string),
		Metric: make(map[string]float64),
		In:     make([]*blocc.TxIn, len(wTx.TxIn)),
		Out:    make([]*blocc.TxOut, len(wTx.TxOut)),
	}

	// Write the raw transaction
	if e.txStoreRaw {
		var r = new(bytes.Buffer)
		wTx.Serialize(r)
		tx.Raw = r.Bytes()
	}

	// Metrics
	weight := int64((wTx.SerializeSizeStripped() * (4 - 1)) + wTx.SerializeSize()) // WitnessScaleFactor = 4
	tx.Data["vin_count"] = cast.ToString(len(wTx.TxIn))
	tx.Data["vout_count"] = cast.ToString(len(wTx.TxOut))
	tx.Data["weight"] = cast.ToString(weight)
	tx.Data["vsize"] = cast.ToString((weight + 3) / 4)
	tx.Data["version"] = cast.ToString(wTx.Version)
	tx.Data["lock_time"] = cast.ToString(wTx.LockTime)

	// This will be returned
	txs := &txStat{
		Coinbase: blockchain.IsCoinBaseTx(wTx),
	}
	tx.Data["coinbase"] = cast.ToString(txs.Coinbase)

	// Parse all of the outputs
	for height, vout := range wTx.TxOut {

		txOut := &blocc.TxOut{
			Value: vout.Value,
			Raw:   vout.PkScript,
		}
		txs.OutputValue += vout.Value

		// Attempt to parse simple addresses out of the script
		scriptType, addresses, reqSigs, err := txscript.ExtractPkScriptAddrs(vout.PkScript, e.chainParams)
		// Could not decode
		if err != nil {
			txOut.Type = txscript.NonStandardTy.String()
		} else {
			txOut.Type = scriptType.String()
			txOut.Addresses = parseBTCAddresses(addresses)
			txOut.Data = map[string]string{
				"req_sigs": cast.ToString(reqSigs),
			}
			txOut.Metric = make(map[string]float64)
		}

		tx.Out[height] = txOut
	}
	tx.Data["out_value"] = cast.ToString(txs.OutputValue)

	// At this point all transaction outputs are final and safe to read.
	// The only things accessed from the blockHeaderTxMon are the outputs so it's safe to put the transaction in the blockHeaderTxMon
	e.blockHeaderTxMon.AddTx(tx, e.blockHeaderTxMonTxLifetime)

	// Parse all of the inputs
	for height, vin := range wTx.TxIn {
		txIn := &blocc.TxIn{
			TxId:   vin.PreviousOutPoint.Hash.String(),
			Height: int64(vin.PreviousOutPoint.Index),
			Data: map[string]string{
				"sequence": cast.ToString(vin.Sequence),
			},
			Metric: make(map[string]float64),
		}
		tx.In[height] = txIn

		// Resolve the previous transaction
		if e.txResolvePrevious && !txs.Coinbase {

			// If the transaction id is not included in prevOutPoints it means it's included in this block
			prevTx, found := prevOutPoints[txIn.TxId]
			if !found {
				// The prevOutPoint was removed from the list of prevOutPoints which means that it is part of this block
				// we will wait unti it's processed
				prevTx = <-e.blockHeaderTxMon.WaitForTxId(txIn.TxId, time.Minute)
				if prevTx == nil {
					e.logger.Warnw("PreviousOutPoint missing while waiting in current block",
						"tx_id", txIn.TxId,
						"height", txIn.Height,
						"blkheight", blk.GetHeightSafe(),
					)
					txs.InputMissing = true
				}
			}

			// We never found the previous transaction, this is a problem
			if prevTx == nil {
				e.logger.Warnw("PreviousOutPoint missing",
					"tx_id", txIn.TxId,
					"height", txIn.Height,
					"blkheight", blk.GetHeightSafe(),
					"found", found,
				)
				txs.InputMissing = true
			} else {
				if int64(len(prevTx.Out)) > txIn.Height {
					txIn.Out = prevTx.Out[txIn.Height]
					txs.InputValue += txIn.Out.Value
				} else {
					e.logger.Warnw("PreviousOutPoint missing transaction",
						"tx_id", txIn.TxId,
						"height", txIn.Height,
						"blkheight", blk.GetHeightSafe(),
						"found", found,
					)
					txs.InputMissing = true
				}
			}
		}
	}

	// Final TX stats
	tx.Data["in_value"] = cast.ToString(txs.InputValue)
	if txs.InputMissing {
		tx.Data["input_missing"] = cast.ToString(txs.InputMissing)
	}
	// If it's a coinbase or we couldn't find an input, mark the fee as zero otherwise it's some negative number messing everything up
	if txs.Coinbase || txs.InputMissing {
		txs.Fee = 0
	} else {
		txs.Fee = txs.InputValue - txs.OutputValue
	}
	tx.Data["fee"] = cast.ToString(txs.Fee)

	// If this transaction came as part of a block, add block metadata
	if blk != nil {

		tx.BlockId = blk.BlockId
		tx.BlockHeight = blk.Height
		tx.Time = blk.Time
		tx.BlockTime = blk.Time

	} else {

		tx.BlockId = blocc.BlockIdMempool
		tx.BlockHeight = blocc.HeightUnknown
		tx.Time = time.Now().UTC().Unix()
		tx.Data["received_time"] = cast.ToString(time.Now().UTC().Unix())

		// Send it on the TxBus
		if e.txBus != nil {
			err := e.txBus.Publish(Symbol, "stream", tx)
			if err != nil {
				e.logger.Errorw("Could not TxMsgBus Public", "error", err)
			}
		}

	}

	// Add it to the blockChainStore
	err := e.blockChainStore.UpsertTransaction(Symbol, tx)
	if err != nil {
		e.logger.Errorw("Could not BlockStore UpsertTransaction", "error", err)
	}

	// At this point in time, the only references to the transaction will be for using the outputs
	// We can clean the TX of unnessesary stuff to free up memory
	tx.Data = nil
	tx.In = nil
	tx.Raw = nil

	// Return tx stats to block handler for aggregating block stats
	return txs

}

// This populates the map of prevOutPoints with any transactions that are still missing
func (e *Extractor) getPrevOutPoints(prevOutPoints map[string]*blocc.Tx, chainCompleteToThisBlock <-chan struct{}, txIdsInThisBlock map[string]struct{}) error {

	// It is assumed at this point that the cache has already been checked while building the map and that we need to either
	// check the block store or wait until the chain is complete and check the cache once more and then the block store one last time

	// Build the list of txIds still missing
	txIds := make([]string, 0)
	missingTxIds := make(map[string]struct{})
	for hash, tx := range prevOutPoints {
		// Remove any prevOutPoints that are in this block as a signal they are in this block
		if _, found := txIdsInThisBlock[hash]; found {
			delete(prevOutPoints, hash)
		} else if tx == nil {
			txIds = append(txIds, hash)
			missingTxIds[hash] = struct{}{}
		}
	}

	// Just sanity check that there's something to do
	if len(txIds) == 0 {
		return nil
	}

	// Fetch all the previous outpoints in one go to the block-store
	txs, err := e.blockChainStore.GetTxsByTxIds(Symbol, txIds, blocc.TxIncludeOut)
	if err != nil {
		return fmt.Errorf("Could not blockChainStore.GetTxsByTxIds: %v", err)
	}

	// Get all the transactions
	for _, tx := range txs {
		prevOutPoints[tx.TxId] = tx
		delete(missingTxIds, tx.TxId)
	}

	// If none are still missing, we're done
	if len(missingTxIds) == 0 {
		return nil
	}

	// Oterwise, we were not able to retrieve them all, we will need to wait until the block chain is complete
	if chainCompleteToThisBlock == nil {
		return nil
	}

	// Wait until chain complete
	<-chainCompleteToThisBlock

	// Make a new list of still missing txIds
	txIds = make([]string, 0)

	// Check the cache once again now that the block chain is complete
	for hash, _ := range missingTxIds {
		tx := <-e.blockHeaderTxMon.WaitForTxId(hash, 0)
		if tx == nil {
			txIds = append(txIds, hash)
		} else {
			prevOutPoints[hash] = tx
			delete(missingTxIds, hash)
		}
	}

	// Now we're good
	if len(missingTxIds) == 0 {
		return nil
	}

	// We may have missed the cache, give the block chain store one more try
	txs, err = e.blockChainStore.GetTxsByTxIds(Symbol, txIds, blocc.TxIncludeOut)
	if err != nil {
		return fmt.Errorf("Could not blockChainStore.GetTxsByTxIds: %v", err)
	}

	// Get all the transactions
	for _, tx := range txs {
		prevOutPoints[tx.TxId] = tx
		delete(missingTxIds, tx.TxId)
	}

	if len(missingTxIds) == 0 {
		return nil
	}

	return fmt.Errorf("Could not find all transactions")

}

// This coverts []btcutil.Address to []string
func parseBTCAddresses(in []btcutil.Address) []string {
	ret := make([]string, len(in), len(in))
	for x, y := range in {
		ret[x] = y.EncodeAddress()
	}
	return ret
}
