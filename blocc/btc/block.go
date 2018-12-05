package btc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const Symbol = "btc"

type BlockStore interface {
	InitBTC() error
	InsertBTCBlock(*Block) error
	UpsertBTCBlock(*Block) error
	FindBTCBlocks() ([]*Block, error)
}

type TxMemPool interface {
	Init() error
	Set(string, interface{}, time.Duration) error
	Delete(string) error
	Get(string, interface{}) error
}

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
	out.Time = in.Header.Timestamp.UTC().Unix()
	out.Bits = strconv.FormatInt(int64(in.Header.Bits), 16)
	out.Weight = int32((in.SerializeSizeStripped() * (4 - 1)) + in.SerializeSize()) // WitnessScaleFactor = 4
	out.Difficulty = e.getDifficultyRatio(in.Header.Bits)
	out.Tx = make([]string, len(in.Transactions), len(in.Transactions))

	// out.Height =
	// out.NextHash =
	// out.Confirmations =

	for x, inTx := range in.Transactions {
		out.Tx[x] = inTx.TxHash().String()

		// This transaction is part of a block, remove it from the mempool
		err := e.mp.Delete(inTx.TxHash().String())
		if err != nil {
			e.logger.Errorw("Could not BlockCache DeleteBTCTransaction", "error", err)
		}
	}

	// err := e.bs.InsertBTCBlock(out)
	// if err != nil {
	// 	e.logger.Errorw("Could not BlockStore InsertBlockBTC", "error", err)
	// }

}

func (e *Extractor) handleTx(in *wire.MsgTx) {

	outTx := new(Transaction)
	outTx.Version = in.Version
	outTx.LockTime = int64(in.LockTime)
	outTx.Txid = in.TxHash().String()
	outTx.Hash = in.WitnessHash().String()
	outTx.Size = int32(in.SerializeSize())
	outTx.Vsize = int32(in.SerializeSizeStripped())
	outTx.Time = time.Now().UTC().Unix()
	outTx.ReceivedTime = time.Now().UTC().Unix()
	outTx.Weight = (outTx.Vsize * (4 - 1)) + outTx.Size // WitnessScaleFactor = 4

	// Handle TxIn
	outTx.Vin = make([]*TxIn, len(in.TxIn), len(in.TxIn))
	for x, tx := range in.TxIn {
		newTx := &TxIn{
			Txid: tx.PreviousOutPoint.Hash.String(),
			Vout: tx.PreviousOutPoint.Index,
			SigScript: &SigScript{
				Hex: hex.EncodeToString(tx.SignatureScript),
			},
			Txwitness: parseWitness(tx.Witness),
			Sequence:  tx.Sequence,
		}
		// Need to resolve previous out point
		// newTx.Value
		outTx.Vin[x] = newTx
	}

	// Handle TxOut
	outTx.Vout = make([]*TxOut, len(in.TxOut), len(in.TxOut))
	for x, tx := range in.TxOut {

		scriptClass, addresses, reqSigs, err := txscript.ExtractPkScriptAddrs(tx.PkScript, e.chainParams)
		if err != nil {
			e.logger.Warnw("Could not decode PkScript", "error", err)
		}

		newTx := &TxOut{
			N:     int32(x),
			Value: tx.Value,
			PkScript: &PkScript{
				Hex:       hex.EncodeToString(tx.PkScript),
				Type:      scriptClass.String(),
				Addresses: parseBTCAddresses(addresses),
				ReqSigs:   int32(reqSigs),
			},
		}
		outTx.Vout[x] = newTx
	}

	// outTx.BlockHash
	// outTx.BlockHeight
	// outTx.BlockTime
	// outTx.Height = position in block
	// outTx.CoinBase
	// outTx.Fee - calculate

	err := e.mp.Set(outTx.Hash, outTx, 14*24*time.Hour)
	if err != nil {
		e.logger.Errorw("Could not BlockCache InsertBlockBTC", "error", err)
	}

}

func (tx *Transaction) MarshalBinary() ([]byte, error) {
	return json.Marshal(tx)
}

func (tx *Block) MarshalBinary() ([]byte, error) {
	return json.Marshal(tx)
}

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
