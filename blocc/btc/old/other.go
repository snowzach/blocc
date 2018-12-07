// +build +other

package btc

// USED TO BUILD BLOCK

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

// USED TO BUILD TRANSACTION
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
