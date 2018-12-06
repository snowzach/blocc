package btc

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"git.coinninja.net/backend/blocc/store"
)

func (e *Extractor) handleBlock(wBlk *wire.MsgBlock, size int) {

	e.logger.Infow("Handling Block", "block_id", wBlk.BlockHash().String())

	// Build the store.Block
	blk := &store.Block{
		Type:        store.TypeBlock,
		Symbol:      Symbol,
		BlockId:     wBlk.BlockHash().String(),
		PrevBlockId: wBlk.Header.PrevBlock.String(),
		Time:        wBlk.Header.Timestamp.UTC().Unix(),
		TxIds:       make([]string, len(wBlk.Transactions), len(wBlk.Transactions)),
		Raw:         new(store.Raw),
	}
	wBlk.Serialize(blk.Raw)

	for x, wTx := range wBlk.Transactions {
		// Build list of transaction ids
		blk.TxIds[x] = wTx.TxHash().String()

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

	// Build the store.Tx
	tx := &store.Tx{
		Type:      store.TypeTx,
		Symbol:    Symbol,
		TxId:      wTx.TxHash().String(),
		Raw:       new(store.Raw),
		Addresses: make([]string, 0),
	}
	wTx.Serialize(tx.Raw)

	// TODO: Fetch the source addesses from the blockstore

	// Append the destination addresses
	for _, vout := range wTx.TxOut {
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(vout.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "error", err)
		}
		tx.Addresses = append(tx.Addresses, parseBTCAddresses(addresses)...)
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
