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
		tx, err := s.blockChainStore.GetTxByTxId(btc.Symbol, id, blocc.TxIncludeHeader|blocc.TxIncludeData)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

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
			} `json:"query"`
		}
		if err := render.DecodeJSON(r.Body, &postData); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}
		if len(postData.Query.Terms.TxID) == 0 {
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("You need to provide at least one txid")))
		}

		// Get txs
		txs, err := s.blockChainStore.GetTxsByTxIds(btc.Symbol, postData.Query.Terms.TxID, blocc.TxIncludeAll)
		if err != nil && err != blocc.ErrNotFound {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		ret := make([]*LegacyTx, len(txs), len(txs))

		for i, tx := range txs {
			ret[i] = TxToLegacyTx(tx)
		}

		render.JSON(w, r, ret)
	}

}
