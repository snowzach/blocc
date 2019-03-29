package server

import (
	"bytes"
	"encoding/hex"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
)

// LegacyTx is the legacy block format
type LegacyTx struct {
	TxID         string        `json:"txid"`
	Hash         string        `json:"hash"`
	BlockHash    string        `json:"blockhash"`
	Height       int64         `json:"height"`
	Version      int64         `json:"version"`
	Size         int64         `json:"size"`
	VSize        int64         `json:"vsize"`
	Weight       int64         `json:"weight"`
	Time         int64         `json:"time"`
	BlockTime    int64         `json:"blocktime"`
	LockTime     int64         `json:"locktime"`
	VIn          []*LegacyVIn  `json:"vin"`
	VOut         []*LegacyVOut `json:"vout"`
	Coinbase     bool          `json:"coinbase"`
	BlockHeight  int64         `json:"blockheight"`
	Blocks       []string      `json:"blocks"`
	ReceivedTime int64         `json:"received_time"`
}

// LegacyVIn is the legacy VIn format
type LegacyVIn struct {
	TxID      string `json:"txid"`
	VOut      int64  `json:"vout"`
	Coinbase  string `json:"coinbase"` // I think this field was an error but I want to match exactly
	ScriptSig struct {
		Asm string `json:"asm"`
		Hex string `json:"hex"`
	} `json:"scriptSig"`
	TxInWitness    []string    `json:"txinwitness"`
	Sequence       int64       `json:"sequence"`
	PreviousOutput *LegacyVOut `json:"previousoutput"`
}

// LegacyVOut is the legacy VOut format
type LegacyVOut struct {
	Value        int64 `json:"value"`
	N            int   `json:"n"`
	ScriptPubKey struct {
		Asm       string   `json:"asm"`
		Hex       string   `json:"hex"`
		ReqSigs   int      `json:"reqsigs"`
		Type      string   `json:"type"`
		Addresses []string `json:"addresses"`
	} `json:"scriptPubKey"`
}

// TxToLegacyTx converts our blocc.Tx to LegacyTx
func TxToLegacyTx(tx *blocc.Tx) *LegacyTx {

	ltx := &LegacyTx{
		TxID:         tx.TxId,
		Hash:         tx.DataValue("hash"),
		BlockHash:    tx.BlockId,
		Height:       tx.Height,
		Version:      cast.ToInt64(tx.DataValue("version")),
		Size:         tx.TxSize,
		VSize:        cast.ToInt64(tx.DataValue("vsize")),
		Weight:       cast.ToInt64(tx.DataValue("weight")),
		Time:         tx.Time,
		BlockTime:    tx.BlockTime,
		LockTime:     cast.ToInt64(tx.DataValue("lock_time")),
		VIn:          make([]*LegacyVIn, len(tx.In), len(tx.In)),
		VOut:         make([]*LegacyVOut, len(tx.Out), len(tx.Out)),
		Coinbase:     cast.ToBool(tx.DataValue("coinbase")),
		BlockHeight:  tx.BlockHeight,
		Blocks:       []string{tx.BlockId},
		ReceivedTime: cast.ToInt64(tx.DataValue("received_time")),
	}

	// If this tx is part of the mempool, return an empty hash and 0 for the height
	if ltx.BlockHash == blocc.BlockIdMempool {
		ltx.BlockHeight = 0
		ltx.BlockHash = ""
	}

	// If received time isn't set, use the tx.Time = Block Time
	if ltx.ReceivedTime == 0 {
		ltx.ReceivedTime = tx.Time
	}

	// Parse the transaction raw bytes
	var wTx = new(wire.MsgTx)
	err := wTx.Deserialize(bytes.NewReader(tx.Raw))
	if err != nil {
		return ltx
	}

	// Get the witness hash
	ltx.Hash = wTx.WitnessHash().String()

	for i, txin := range tx.In {
		vin := &LegacyVIn{
			TxID:           txin.TxId,
			VOut:           txin.Height,
			Sequence:       cast.ToInt64(txin.DataValue("sequence")),
			TxInWitness:    make([]string, 0),
			PreviousOutput: TxOutToLegacyVOut(txin.Out, int(txin.Height)),
		}
		// Other fields from the raw transaction

		if len(wTx.TxIn) > i {
			vin.ScriptSig.Asm, _ = txscript.DisasmString(wTx.TxIn[i].SignatureScript)
			vin.ScriptSig.Hex = hex.EncodeToString(wTx.TxIn[i].SignatureScript)
			for _, witness := range wTx.TxIn[i].Witness {
				vin.TxInWitness = append(vin.TxInWitness, hex.EncodeToString(witness))
			}
		}
		ltx.VIn[i] = vin
	}

	for i, out := range tx.Out {
		ltx.VOut[i] = TxOutToLegacyVOut(out, i)
	}

	return ltx

}

// TxOutToLegacyVOut converts our blocc.TxOut to LegacyVOut
func TxOutToLegacyVOut(txout *blocc.TxOut, n int) *LegacyVOut {
	if txout == nil {
		return new(LegacyVOut)
	}
	vout := &LegacyVOut{
		Value: txout.Value,
		N:     n,
	}

	vout.ScriptPubKey.Asm, _ = txscript.DisasmString(bytes.TrimRight(txout.Raw, "\000"))
	vout.ScriptPubKey.Hex = hex.EncodeToString(bytes.TrimRight(txout.Raw, "\000"))
	vout.ScriptPubKey.ReqSigs = cast.ToInt(txout.DataValue("req_sigs"))
	vout.ScriptPubKey.Type = txout.Type
	vout.ScriptPubKey.Addresses = txout.Addresses

	return vout
}
