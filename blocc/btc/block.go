package btc

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

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
		Time:        wBlk.Header.Timestamp.UTC().Unix(),
		Txids:       make([]string, len(wBlk.Transactions), len(wBlk.Transactions)),
		Metric:      make(map[string]float64),
		Tag:         make(map[string]string),
	}

	// Write the raw block
	if e.storeRawBlocks {
		var r = new(bytes.Buffer)
		wBlk.Serialize(r)
		blk.Raw = r.Bytes()
	}

	// Metrics
	blk.Metric["size"] = float64(wBlk.SerializeSize())

	for x, wTx := range wBlk.Transactions {
		// Build list of transaction ids
		blk.Txids[x] = wTx.TxHash().String()

		// Handle a transaction
		e.handleTx(wBlk, int32(x), wTx)
	}

	// if blockstore is activated, store the block
	if e.bs != nil {
		err := e.bs.InsertBlock(Symbol, blk)
		if err != nil {
			e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
		}
	}

}

func (e *Extractor) handleTx(wBlk *wire.MsgBlock, height int32, wTx *wire.MsgTx) {

	// Build the blocc.Tx
	tx := &blocc.Tx{
		Type:      blocc.TypeTx,
		Symbol:    Symbol,
		TxId:      wTx.TxHash().String(),
		Addresses: make([]string, 0),
		Metric:    make(map[string]float64),
		Tag:       make(map[string]string),
	}

	// Write the raw transaction
	if e.storeRawTransactions {
		var r = new(bytes.Buffer)
		wTx.Serialize(r)
		tx.Raw = r.Bytes()
	}

	// Metrics
	tx.Metric["vin_count"] = float64(len(wTx.TxIn))
	tx.Metric["vout_count"] = float64(len(wTx.TxOut))
	tx.Metric["size"] = float64(wTx.SerializeSize())
	tx.Metric["value"] = 0

	// TODO: Fetch the source addesses from the blockstore

	// Append the destination addresses
	for _, vout := range wTx.TxOut {
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(vout.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "error", err)
		}
		tx.Addresses = append(tx.Addresses, parseBTCAddresses(addresses)...)
		tx.Metric["value"] += float64(vout.Value)
	}

	// If this transaction came as part of a block, add block metadata
	if wBlk != nil {
		tx.Height = int64(height)
		tx.BlockId = wBlk.BlockHash().String()
		tx.Time = wBlk.Header.Timestamp.UTC().Unix()
		tx.BlockTime = wBlk.Header.Timestamp.UTC().Unix()

		// Insert it into the blockstore
		if e.bs != nil {
			err := e.bs.InsertTransaction(Symbol, tx)
			if err != nil {
				e.logger.Errorw("Could not BlockStore InsertTransaction", "error", err)
			}
		}

		// If we have a txStore, remove this transaction if it exists
		if e.ts != nil {
			err := e.ts.DeleteTransaction(Symbol, tx.TxId)
			if err != nil {
				e.logger.Errorw("Could not TxStore DeleteTransaction", "error", err)
			}
		}

	} else {
		// Store it in the transaction store
		if e.ts != nil {
			err := e.ts.InsertTransaction(Symbol, tx, e.txLifetime)
			if err != nil {
				e.logger.Errorw("Could not TxStore InsertTransaction", "error", err)
			}
		}

		// Send it on the message bus
		if e.mb != nil {
			err := e.mb.Publish(Symbol, "stream", tx)
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
func (e *Extractor) getDifficultyRatio(bits uint32) float64 {
	// The minimum difficulty is the max possible proof-of-work limit bits
	// converted back to a number.  Note this is not the same as the proof of
	// work limit directly because the block difficulty is encoded in a block
	// with the compact form which loses precision.
	max := blockchain.CompactToBig(e.chainParams.PowLimitBits)
	target := blockchain.CompactToBig(bits)

	difficulty := new(big.Rat).SetFrac(max, target)
	outString := difficulty.FloatString(8)
	diff, err := strconv.ParseFloat(outString, 64)
	if err != nil {
		return 0
	}
	return diff
}
