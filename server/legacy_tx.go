package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
)

// LegacyGetTx fetches a transaction by Id
func (s *Server) LegacyGetTx() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")
		tx, err := s.blockChainStore.GetTxByTxId(btc.Symbol, id, blocc.TxIncludeAll)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		// Fix any issues with the inputs if there are some
		s.txCheckInputs(tx)

		// Return the block
		render.JSON(w, r, TxToLegacyTx(tx))

	}

}

// LegacyGetTxConfirmations finds a block and and returns the transactions
func (s *Server) LegacyGetTxConfirmations() http.HandlerFunc {

	type txConfirmations struct {
		TxID          string `json:"txid"`
		Confirmations int64  `json:"confirmations"`
	}

	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")
		tx, err := s.blockChainStore.GetTxByTxId(btc.Symbol, id, blocc.TxIncludeHeader)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		height := tx.BlockHeight

		if height != blocc.HeightUnknown {
			// Get the height
			blkTipHeader, err := s.blockChainStore.GetBlockHeaderTopByStatuses(btc.Symbol, nil)
			if err != nil {
				render.Render(w, r, s.ErrInternalLog(err))
				return
			}
			height = blkTipHeader.Height - height
		}

		// Return the block
		render.JSON(w, r, &txConfirmations{
			TxID:          id,
			Confirmations: height,
		})

	}

}

// LegacyGetTxStats gets statistics from a Tx
func (s *Server) LegacyGetTxStats() http.HandlerFunc {

	type txStats struct {
		TxID          string  `json:"txid"`
		Coinbase      bool    `json:"coinbase"`
		Fees          int64   `json:"fees"`
		FeesPerByte   float32 `json:"fees_per_byte"`
		FeesPerWeight float32 `json:"fees_per_weight"`
		Miner         string  `json:"miner"`
		Size          int64   `json:"size"`
		VSize         int64   `json:"vsize"`
		VInValue      int64   `json:"vin_value"`
		VOutValue     int64   `json:"vout_value"`
		Weight        int64   `json:"weight"`
	}

	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")
		// tx, err := s.blockChainStore.GetTxByTxId(btc.Symbol, id, blocc.TxIncludeHeader|blocc.TxIncludeData)
		// Need all but raw to repair tx with missing inputs
		tx, err := s.blockChainStore.GetTxByTxId(btc.Symbol, id, blocc.TxIncludeAllButRaw)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		// Fix any issues with the inputs if there are some
		s.txCheckInputs(tx)

		ret := &txStats{
			TxID:      id,
			Coinbase:  cast.ToBool(tx.DataValue("coinbase")),
			Fees:      cast.ToInt64(tx.DataValue("fee")),
			Size:      tx.TxSize,
			VSize:     cast.ToInt64(tx.DataValue("vsize")),
			VInValue:  cast.ToInt64(tx.DataValue("in_value")),
			VOutValue: cast.ToInt64(tx.DataValue("out_value")),
			Weight:    cast.ToInt64(tx.DataValue("weight")),
		}
		ret.FeesPerByte = float32(ret.Fees) / float32(ret.Size)
		ret.FeesPerWeight = float32(ret.Fees) / float32(ret.Weight)

		// Return the tx
		render.JSON(w, r, ret)

	}

}

// LegacyFindTxIds finds multiple transactions
func (s *Server) LegacyFindTxIds() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var postData struct {
			Query struct {
				Terms struct {
					TxID []string `json:"txid"`
				} `json:"terms"`
				Missing bool `json:"missing"`
			} `json:"query"`
		}
		if err := render.DecodeJSON(r.Body, &postData); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
		if len(postData.Query.Terms.TxID) == 0 {
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("You need to provide at least one txid")))
		}

		// We only care about finding a list of the missing IDs (transactions expired or removed)
		if postData.Query.Missing {

			// Create the map of txids we are searching for
			txids := make(map[string]struct{})
			for _, txid := range postData.Query.Terms.TxID {
				txids[txid] = struct{}{}
			}

			// Get txs
			txs, err := s.blockChainStore.GetTxsByTxIds(btc.Symbol, postData.Query.Terms.TxID, blocc.TxIncludeHeader)
			if err != nil && err != blocc.ErrNotFound {
				render.Render(w, r, ErrInvalidRequest(err))
				return
			}

			// Remove it from the list
			for _, tx := range txs {
				delete(txids, tx.TxId)
			}

			// Make the list of missing txids
			ret := make([]string, 0, len(txids))
			for txid := range txids {
				ret = append(ret, txid)
			}

			render.JSON(w, r, ret)
			return

		}

		// Get txs
		txs, err := s.blockChainStore.GetTxsByTxIds(btc.Symbol, postData.Query.Terms.TxID, blocc.TxIncludeAll)
		if err != nil && err != blocc.ErrNotFound {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		ret := make([]*LegacyTx, len(txs), len(txs))

		for i, tx := range txs {

			// Fix any issues with the inputs if there are some
			s.txCheckInputs(tx)

			ret[i] = TxToLegacyTx(tx)
		}

		render.JSON(w, r, ret)
	}

}

// This will check a transaction for missing inputs
func (s *Server) txCheckInputs(tx *blocc.Tx) {

	var inValue int64
	var outValue int64
	var hadMissing bool
	var stillMissing bool

	for _, txIn := range tx.In {
		if txIn.Out == nil {
			hadMissing = true
			s.logger.Warnw("Handling Missing", "tx_id", txIn.TxId, "height", txIn.Height)
			txIn.Out = s.txGetOutput(tx.Symbol, txIn.TxId, txIn.Height)
		}
		if txIn.Out == nil {
			stillMissing = true
		} else {
			inValue += txIn.Out.Value
		}
	}
	if hadMissing && !stillMissing {
		for _, txOut := range tx.Out {
			outValue += txOut.Value
		}
		s.logger.Warnw("Repaired", "tx", tx)

		tx.Data["in_value"] = cast.ToString(inValue)
		tx.Data["out_value"] = cast.ToString(inValue)
		tx.Data["fee"] = cast.ToString(inValue - outValue)
		tx.Data["fee_vsize"] = cast.ToString(float64(inValue-outValue) / cast.ToFloat64(tx.DataValue("vsize")))

	}

}

// This will fetch an output or return nil
func (s *Server) txGetOutput(symbol string, txId string, height int64) *blocc.TxOut {
	tx, err := s.blockChainStore.GetTxByTxId(symbol, txId, blocc.TxIncludeOut)
	if err != nil {
		return nil
	}
	if int64(len(tx.Out)) > height {
		return tx.Out[height]
	}
	return nil
}
