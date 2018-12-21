package btc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
)

func (e *Extractor) handleBlock(wBlk *wire.MsgBlock, size int) {

	e.logger.Infow("Handling Block", "block_id", wBlk.BlockHash().String())

	// Build the blocc.Block
	blk := &blocc.Block{
		BlockId:     wBlk.BlockHash().String(),
		PrevBlockId: wBlk.Header.PrevBlock.String(),
		Height:      blocc.HeightUnknown,
		Time:        wBlk.Header.Timestamp.UTC().Unix(),
		TxIds:       make([]string, len(wBlk.Transactions), len(wBlk.Transactions)),
		Data:        make(map[string]string),
	}

	// Write the raw block
	if e.storeRawBlocks {
		var r = new(bytes.Buffer)
		wBlk.Serialize(r)
		blk.Raw = r.Bytes()
	}

	// Metrics
	blk.Data["size"] = cast.ToString(wBlk.SerializeSize())
	blk.Data["stripped_size"] = cast.ToString(wBlk.SerializeSizeStripped())
	blk.Data["weight"] = cast.ToString((wBlk.SerializeSizeStripped() * (4 - 1)) + wBlk.SerializeSize()) // WitnessScaleFactor = 4
	blk.Data["bits"] = cast.ToString(wBlk.Header.Bits)
	blk.Data["difficulty"] = cast.ToString(e.getDifficultyRatio(wBlk.Header.Bits))
	blk.Data["version"] = cast.ToString(wBlk.Header.Version)
	blk.Data["version_hex"] = fmt.Sprintf("%08x", wBlk.Header.Version)
	blk.Data["merkle_root"] = wBlk.Header.MerkleRoot.String()
	blk.Data["nonce"] = cast.ToString(wBlk.Header.Nonce)

	// Build list of transaction ids
	for x, wTx := range wBlk.Transactions {
		blk.TxIds[x] = wTx.TxHash().String()
	}

	// if BlockChainStore is activated, determine the height if possible and store the block
	if e.bcs != nil {
		e.Lock()
		// If we know of this previous block, record the height
		if blk.PrevBlockId == e.validBlockId {
			blk.Height = e.validBlockHeight + 1
			e.Unlock()
		} else {
			e.Unlock()
			// If we still don't know, wait for it
			prevBlk := <-e.bm.WaitForBlockId(blk.PrevBlockId, time.Now().Add(10*time.Minute))
			if prevBlk != nil && prevBlk.Height != blocc.HeightUnknown {
				blk.Height = prevBlk.Height + 1
			}
		}

		// If we figure out the height, store the block
		if blk.Height != blocc.HeightUnknown {
			err := e.bcs.InsertBlock(Symbol, blk)
			if err != nil {
				e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
			}
		}

		e.bm.AddBlock(blk, time.Now().Add(5*time.Minute))
	}

	// Handle transactions in parallel
	var wg sync.WaitGroup
	for x, wTx := range wBlk.Transactions {
		wg.Add(1)
		go func(txHeight int64, t *wire.MsgTx) {
			e.handleTx(blk, txHeight, t)
			wg.Done()
		}(int64(x), wTx)
	}
	wg.Wait()

	e.logger.Infow("Handled Block", "block_id", blk.BlockId, "height", blk.Height)

}

func (e *Extractor) handleTx(blk *blocc.Block, txHeight int64, wTx *wire.MsgTx) {

	// Build the blocc.Tx
	tx := &blocc.Tx{
		TxId:   wTx.TxHash().String(),
		Height: txHeight,
		Data:   make(map[string]string),
		Out:    make([]*blocc.Out, len(wTx.TxOut)),
	}

	// Write the raw transaction
	if e.storeRawTransactions {
		var r = new(bytes.Buffer)
		wTx.Serialize(r)
		tx.Raw = r.Bytes()
	}

	// Metrics
	tx.Data["vin_count"] = cast.ToString(len(wTx.TxIn))
	tx.Data["vout_count"] = cast.ToString(len(wTx.TxOut))
	tx.Data["size"] = cast.ToString(wTx.SerializeSize())
	tx.Data["vsize"] = cast.ToString(wTx.SerializeSizeStripped())
	tx.Data["weight"] = cast.ToString((wTx.SerializeSizeStripped() * (4 - 1)) + wTx.SerializeSize()) // WitnessScaleFactor = 4
	tx.Data["version"] = cast.ToString(wTx.Version)
	tx.Data["lock_time"] = cast.ToString(wTx.LockTime)
	tx.Data["hash"] = wTx.TxHash().String()

	// TODO: Fetch the source addesses from the blockstore

	var txValue int64

	// Parse all of the outputs
	for height, vout := range wTx.TxOut {
		scriptType, addresses, _, err := txscript.ExtractPkScriptAddrs(vout.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "txId", tx.TxId, "error", err, "addresses", addresses, "script", hex.EncodeToString(vout.PkScript))
			continue
		}

		tx.Out[height] = &blocc.Out{
			Type:      scriptType.String(),
			Addresses: parseBTCAddresses(addresses),
			Value:     vout.Value,
		}
		txValue += vout.Value
	}

	tx.Data["value"] = cast.ToString(txValue)

	// If this transaction came as part of a block, add block metadata
	if blk != nil {
		tx.BlockId = blk.BlockId
		tx.BlockHeight = blk.Height
		tx.Time = blk.Time
		tx.BlockTime = blk.Time

		// Insert it into the BlockChainStore but only if we know of it as part of the chain
		if e.bcs != nil && blk.Height != blocc.HeightUnknown {
			err := e.bcs.InsertTransaction(Symbol, tx)
			if err != nil {
				e.logger.Errorw("Could not BlockStore InsertTransaction", "error", err)
			}
		}

		// If we have a TxPool, remove this transaction if it exists
		if e.txp != nil {
			err := e.txp.DeleteTransaction(Symbol, tx.TxId)
			if err != nil {
				e.logger.Errorw("Could not TxStore DeleteTransaction", "error", err)
			}
		}

	} else {

		tx.Data["received_time"] = cast.ToString(time.Now().UTC().Unix())

		// Store it in the TxPool
		if e.txp != nil {
			err := e.txp.InsertTransaction(Symbol, tx, e.txLifetime)
			if err != nil {
				e.logger.Errorw("Could not TxStore InsertTransaction", "error", err)
			}
		}

		// Send it on the TxBus
		if e.txb != nil {
			err := e.txb.Publish(Symbol, "stream", tx)
			if err != nil {
				e.logger.Errorw("Could not TxMsgBus Public", "error", err)
			}
		}
	}

}

// This converts [][]byte (witnesses) to []string
func parseWitness(in [][]byte) []string {
	ret := make([]string, len(in), len(in))
	for x, y := range in {
		ret[x] = hex.EncodeToString(y)
	}
	return ret
}

// This coverts []btcutil.Address to []string
func parseBTCAddresses(in []btcutil.Address) []string {
	ret := make([]string, len(in), len(in))
	for x, y := range in {
		ret[x] = y.String()
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
