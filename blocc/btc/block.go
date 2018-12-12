package btc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
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
		Type:        blocc.TypeBlock,
		Symbol:      Symbol,
		BlockId:     wBlk.BlockHash().String(),
		PrevBlockId: wBlk.Header.PrevBlock.String(),
		Height:      -1,
		Time:        wBlk.Header.Timestamp.UTC().Unix(),
		Txids:       make([]string, len(wBlk.Transactions), len(wBlk.Transactions)),
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

	for x, wTx := range wBlk.Transactions {
		// Build list of transaction ids
		blk.Txids[x] = wTx.TxHash().String()

		// Handle a transaction
		e.handleTx(wBlk, int32(x), wTx)
	}

	e.Lock()
	// If we know of this previous block, record the height
	if blk.PrevBlockId == e.validBlockId {
		blk.Height = e.validBlockHeight + 1
		e.Unlock()
	} else {
		e.Unlock()
		// If we still don't know, wait for it
		prevBlk := <-e.bm.WaitForBlockId(blk.PrevBlockId, time.Now().Add(2*time.Minute))
		if prevBlk != nil && prevBlk.Height != -1 {
			blk.Height = prevBlk.Height + 1
		}
	}

	// Add the block to the block montior
	e.bm.AddBlock(blk, time.Now().Add(10*time.Minute))

	// if BlockChainStore is activated, store the block but keep processing
	if e.bcs != nil {
		err := e.bcs.InsertBlock(Symbol, blk)
		if err != nil {
			e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
		}
	}

	e.logger.Infow("Handled Block", "block_id", blk.BlockId, "height", blk.Height)

}

func (e *Extractor) handleTx(wBlk *wire.MsgBlock, height int32, wTx *wire.MsgTx) {

	// Build the blocc.Tx
	tx := &blocc.Tx{
		Type:      blocc.TypeTx,
		Symbol:    Symbol,
		TxId:      wTx.TxHash().String(),
		Addresses: make([]string, 0),
		Data:      make(map[string]string),
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

	var value int64

	// Append the destination addresses
	for _, vout := range wTx.TxOut {
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(vout.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "txId", tx.TxId, "error", err, "addresses", addresses, "script", hex.EncodeToString(vout.PkScript))
			continue
		}
		tx.Addresses = append(tx.Addresses, parseBTCAddresses(addresses)...)
		value += vout.Value
	}

	tx.Data["value"] = cast.ToString(value)

	// If this transaction came as part of a block, add block metadata
	if wBlk != nil {
		tx.Height = int64(height)
		tx.BlockId = wBlk.BlockHash().String()
		tx.Time = wBlk.Header.Timestamp.UTC().Unix()
		tx.BlockTime = wBlk.Header.Timestamp.UTC().Unix()

		// Insert it into the BlockChainStore
		if e.bcs != nil {
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
