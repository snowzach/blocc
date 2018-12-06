package btc

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"

	"git.coinninja.net/backend/blocc/store"
)

func (e *Extractor) handleBlock(in *wire.MsgBlock, size int) {

	e.logger.Debugf("Handling Block: %s", in.BlockHash().String())

	b := &store.Block{
		Type:        "block",
		Symbol:      Symbol,
		BlockId:     in.BlockHash().String(),
		PrevBlockId: in.Header.PrevBlock.String(),
		Time:        in.Header.Timestamp.UTC().Unix(),
		TxIds:       make([]string, len(in.Transactions), len(in.Transactions)),
		Raw:         new(store.Raw),
	}
	in.Serialize(b.Raw)

	// // Translate from wire.MsgBlock to our proto block format
	// out := new(Block)
	// out.Type = "blk"
	// out.Hash = in.BlockHash().String()
	// out.PrevHash = in.Header.PrevBlock.String()
	// out.StrippedSize = int32(in.SerializeSizeStripped())
	// out.Size = int32(size)
	// out.Version = in.Header.Version
	// out.VersionHex = fmt.Sprintf("%08x", in.Header.Version)
	// out.MerkleRoot = in.Header.MerkleRoot.String()
	// out.Nonce = in.Header.Nonce
	// out.Time = in.Header.Timestamp.UTC().Unix()
	// out.Bits = strconv.FormatInt(int64(in.Header.Bits), 16)
	// out.Weight = int32((in.SerializeSizeStripped() * (4 - 1)) + in.SerializeSize()) // WitnessScaleFactor = 4
	// out.Difficulty = e.getDifficultyRatio(in.Header.Bits)
	// out.Tx = make([]string, len(in.Transactions), len(in.Transactions))

	// out.Height =
	// out.NextHash =
	// out.Confirmations =

	for x, inTx := range in.Transactions {
		b.TxIds[x] = inTx.TxHash().String()

		// This transaction is part of a block, remove it from the mempool
		err := e.ts.DeleteTransaction(Symbol, inTx.TxHash().String())
		if err != nil {
			e.logger.Errorw("Could not BlockCache DeleteBTCTransaction", "error", err)
		}

		e.handleTx(in, int32(x), inTx)
	}

	err := e.bs.InsertBlock(Symbol, b)
	if err != nil {
		e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
	}

}

func (e *Extractor) handleTx(blk *wire.MsgBlock, height int32, in *wire.MsgTx) {

	t := &store.Tx{
		Type:      "tx",
		Symbol:    Symbol,
		TxId:      in.TxHash().String(),
		Raw:       new(store.Raw),
		Addresses: make([]string, 0),
	}
	in.Serialize(t.Raw)

	// var raw = bytes.NewBuffer()
	// blk.Serialize(raw)

	// outTx := new(Transaction)
	// outTx.Type = "tx"
	// outTx.Version = in.Version
	// outTx.LockTime = int64(in.LockTime)
	// outTx.Txid = in.TxHash().String()
	// outTx.Hash = in.WitnessHash().String()
	// outTx.Size = int32(in.SerializeSize())
	// outTx.Vsize = int32(in.SerializeSizeStripped())
	// outTx.Time = time.Now().UTC().Unix()
	// outTx.ReceivedTime = time.Now().UTC().Unix()
	// outTx.Weight = (outTx.Vsize * (4 - 1)) + outTx.Size // WitnessScaleFactor = 4

	// // Handle TxIn
	// outTx.Vin = make([]*TxIn, len(in.TxIn), len(in.TxIn))
	// for x, tx := range in.TxIn {
	// 	newTx := &TxIn{
	// 		Txid: tx.PreviousOutPoint.Hash.String(),
	// 		Vout: tx.PreviousOutPoint.Index,
	// 		SigScript: &SigScript{
	// 			Hex: hex.EncodeToString(tx.SignatureScript),
	// 		},
	// 		Txwitness: parseWitness(tx.Witness),
	// 		Sequence:  tx.Sequence,
	// 	}
	// 	// Need to resolve previous out point
	// 	// newTx.Value
	// 	outTx.Vin[x] = newTx
	// }

	// // Handle TxOut
	// outTx.Vout = make([]*TxOut, len(in.TxOut), len(in.TxOut))
	// for x, tx := range in.TxOut {

	// 	scriptClass, addresses, reqSigs, err := txscript.ExtractPkScriptAddrs(tx.PkScript, e.chainParams)
	// 	if err != nil {
	// 		e.logger.Warnw("Could not decode PkScript", "error", err)
	// 	}

	// 	newTx := &TxOut{
	// 		N:     int32(x),
	// 		Value: tx.Value,
	// 		PkScript: &PkScript{
	// 			Hex:       hex.EncodeToString(tx.PkScript),
	// 			Type:      scriptClass.String(),
	// 			Addresses: parseBTCAddresses(addresses),
	// 			ReqSigs:   int32(reqSigs),
	// 		},
	// 	}
	// 	outTx.Vout[x] = newTx
	// }

	// // outTx.CoinBase
	// // outTx.Fee - calculate
	// outTx.BlockHash = blk.BlockHash().String()
	// outTx.BlockTime = blk.Header.Timestamp.UTC().Unix()
	// outTx.Height = pos
	// // outTx.BlockHeight

	// Append the destination addresses
	for _, tx := range in.TxOut {
		_, addresses, _, err := txscript.ExtractPkScriptAddrs(tx.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "error", err)
		}
		t.Addresses = append(t.Addresses, parseBTCAddresses(addresses)...)
	}

	if blk != nil {

		t.Height = int64(height)
		t.BlockId = blk.BlockHash().String()
		t.Time = blk.Header.Timestamp.UTC().Unix()
		t.BlockTime = blk.Header.Timestamp.UTC().Unix()

		err := e.bs.InsertTransaction(Symbol, t)
		if err != nil {
			e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
		}

	} else {
		// Store it in the transaction store for 2 weeks
		err := e.ts.InsertTransaction(Symbol, t, 14*24*time.Hour)
		if err != nil {
			e.logger.Errorw("Could not BlockCache InsertBlockBTC", "error", err)
		}
	}

}

// func (tx *Transaction) MarshalBinary() ([]byte, error) {
// 	return json.Marshal(tx)
// }

// func (tx *Block) MarshalBinary() ([]byte, error) {
// 	return json.Marshal(tx)
// }

func parseWitness(in [][]byte) []string {
	ret := make([]string, len(in), len(in))
	for x, y := range in {
		ret[x] = hex.EncodeToString(y)
	}
	return ret
}

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
