package btc

import (
	"fmt"
	"math/big"
	"strconv"
	// "github.com/golang/protobuf/ptypes"

	// "github.com/btcsuite/btcd/chaincfg"
	// "github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/wire"
)

func (e *Extractor) handleBlock(in *wire.MsgBlock, size int) {

	// Translate from wire.MsgBlock to our proto block format
	out := new(Block)
	out.Hash = in.BlockHash().String()
	out.PrevHash = in.Header.PrevBlock.String()
	out.StrippedSize = int32(in.SerializeSizeStripped())
	out.Size = int32(size)
	out.Version = in.Header.Version
	out.VersionHex = fmt.Sprintf("%08x", in.Header.Version)
	out.MerkleRoot = in.Header.MerkleRoot.String()
	out.Nonce = in.Header.Nonce
	out.Timestamp = in.Header.Timestamp.UTC().Unix()
	out.Bits = strconv.FormatInt(int64(in.Header.Bits), 16)
	out.Weight = int32((in.SerializeSizeStripped() * (4 - 1)) + in.SerializeSize()) // WitnessScaleFactor = 4
	out.Difficulty = e.getDifficultyRatio(in.Header.Bits)
	out.Tx = make([]string, len(in.Transactions), len(in.Transactions))

	for x, tx := range in.Transactions {
		out.Tx[x] = tx.TxHash().String()

		// 	mtx := new(Tx)
		// 	mtx.Version = tx.Version
		// 	mtx.Hash = Hash(tx.TxHash())
		// 	mtx.TxIn = make([]*TxIn, len(tx.TxIn), len(tx.TxIn))
		// 	for y, txin := range tx.TxIn {
		// 		mtx.TxIn[y] = &TxIn{
		// 			PreviousOutPoint: OutPoint{
		// 				Hash:  Hash(txin.PreviousOutPoint.Hash),
		// 				Index: txin.PreviousOutPoint.Index,
		// 			},
		// 			// SignatureScript: txin.SignatureScript,
		// 			Witness:  TxWitness(txin.Witness),
		// 			Sequence: txin.Sequence,
		// 		}
		// 	}
		// 	mtx.TxOut = make([]*TxOut, len(tx.TxOut), len(tx.TxOut))
		// 	for z, txout := range tx.TxOut {
		// 		if txout == nil {
		// 			continue
		// 		}
		// 		addrtype, addrs, _, _ := txscript.ExtractPkScriptAddrs(txout.PkScript, &chaincfg.MainNetParams)
		// 		addrstring := make([]string, len(addrs), len(addrs))
		// 		for w, addr := range addrs {
		// 			addrstring[w] = addr.String()
		// 		}
		// 		mtx.TxOut[z] = &TxOut{
		// 			Value: txout.Value,
		// 			// PkScript:  txout.PkScript,
		// 			AddrType:  addrtype.String(),
		// 			Addresses: addrstring,
		// 		}
		// 		// fmt.Printf("A:%v B:%v E:%v ADD:%v\n", a, b, err, add)
		// 	}
		// 	mtx.LockTime = tx.LockTime

		// 	out.Transactions[x] = mtx

	}

	err := e.bs.InsertBlockBTC(out)
	if err != nil {
		e.logger.Errorw("Could not InsertBlockBTC", "error", err)
	}

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
