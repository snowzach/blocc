package server

import (
	"fmt"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
	"git.coinninja.net/backend/blocc/store"
)

var (
	blockIDRegex     = regexp.MustCompile(`^[a-zA-Z0-9]{64}$`)
	blockHeightRegex = regexp.MustCompile(`^[0-9]*$`)
)

// LegacyBlock mimics the JSON RPC response from bitcoind
type LegacyBlock struct {
	Hash              string   `json:"hash"`
	Height            int64    `json:"height"`
	Size              int64    `json:"size"`
	StrippedSize      int64    `json:"strippedsize"`
	Weight            int64    `json:"weight"`
	Version           int64    `json:"version"`
	MerkleRoot        string   `json:"merkleroot"`
	Time              int64    `json:"time"`
	Nonce             int64    `json:"nonce"`
	Bits              int64    `json:"bits"`
	Difficulty        float64  `json:"difficulty"`
	PreviousBlockHash string   `json:"previousblockhash"`
	NextBlockHash     string   `json:"nextblockhash"`
	Connected         bool     `json:"connected"`
	ReceivedTime      int64    `json:"received_time"`
	Status            string   `json:"status"`
	VersionHex        string   `json:"versionHex"`
	Tx                []string `json:"tx"`
}

// LegacyFindBlocks returns paginated set of blocks matching the query
func (s *Server) LegacyFindBlocks(method string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var page int
		var perPage int
		var start *time.Time
		var end *time.Time
		var blockIds []string

		switch method {
		case http.MethodGet:
			// Handle paging
			page = cast.ToInt(r.URL.Query().Get("page"))
			if page <= 0 {
				page = 1
			}
			perPage = cast.ToInt(r.URL.Query().Get("perPage"))
			if perPage <= 0 {
				perPage = s.defaultCount
			}
			// Handle time
			start = blocc.ParseUnixTime(cast.ToInt64(r.URL.Query().Get("start")))
			end = blocc.ParseUnixTime(cast.ToInt64(r.URL.Query().Get("end")))

			blockIds = []string{}

		case http.MethodPost:

			var postData struct {
				Query struct {
					Terms struct {
						Hash []string `json:"hash"`
					} `json:"terms"`
				} `json:"query"`
			}
			if err := render.DecodeJSON(r.Body, &postData); err != nil {
				render.Render(w, r, ErrInvalidRequest(err))
				return
			}
			if len(postData.Query.Terms.Hash) == 0 {
				render.Render(w, r, ErrInvalidRequest(fmt.Errorf("You need to provide at least one hash")))
			}

			page = 1
			perPage = store.CountMax
			blockIds = postData.Query.Terms.Hash
		}

		// Get blocks
		blks, err := s.blockChainStore.FindBlocksByBlockIdsAndTime(btc.Symbol, blockIds, start, end, blocc.BlockIncludeHeader|blocc.BlockIncludeData|blocc.BlockIncludeTxIds, (page-1)*perPage, perPage)
		if err != nil && err != blocc.ErrNotFound {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		ret := make([]*LegacyBlock, len(blks), len(blks))

		for i, blk := range blks {
			ret[i] = BlockToLegacyBlock(blk)
		}

		render.JSON(w, r, ret)
	}

}

// LegacyGetBlock returns a block for the provided id
func (s *Server) LegacyGetBlock() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		var blk *blocc.Block
		var err error

		// Get the id
		id := chi.URLParam(r, "id")
		if blockHeightRegex.MatchString(id) {
			var blks []*blocc.Block
			blks, err = s.blockChainStore.FindBlocksByHeight(btc.Symbol, cast.ToInt64(id), blocc.BlockIncludeHeader|blocc.BlockIncludeData|blocc.BlockIncludeTxIds)
			if err == nil {
				if len(blks) == 0 {
					err = blocc.ErrNotFound
				} else {
					// Return the first one we find
					blk = blks[0]
				}
			}
		} else if blockIDRegex.MatchString(id) {
			blk, err = s.blockChainStore.GetBlockByBlockId(btc.Symbol, id, blocc.BlockIncludeHeader|blocc.BlockIncludeData|blocc.BlockIncludeTxIds)
		} else if id == "tip" {
			blk, err = s.blockChainStore.GetBlockTopByStatuses(btc.Symbol, nil, blocc.BlockIncludeHeader|blocc.BlockIncludeData|blocc.BlockIncludeTxIds)
		} else {
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("Unable to determine block hash or block height")))
			return
		}
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil && !blocc.IsValidationError(err) {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		// Return the block
		render.JSON(w, r, BlockToLegacyBlock(blk))
	}

}

// LegacyGetBlockConfirmations returns the block confirmations based on id
func (s *Server) LegacyGetBlockConfirmations() http.HandlerFunc {

	type blockConfirmations struct {
		Hash          string `json:"hash"`
		Confirmations int64  `json:"confirmations"`
	}

	return func(w http.ResponseWriter, r *http.Request) {

		// Get the height
		blkTipHeader, err := s.blockChainStore.GetBlockHeaderTopByStatuses(btc.Symbol, nil)
		if err != nil {
			render.Render(w, r, s.ErrInternalLog(err))
			return
		}

		id := chi.URLParam(r, "id")

		// Get the block header
		blk, err := s.blockChainStore.GetBlockByBlockId(btc.Symbol, id, blocc.BlockIncludeHeader)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil && !blocc.IsValidationError(err) {
			render.Render(w, r, s.ErrInternalLog(err))
			return
		}

		render.JSON(w, r, &blockConfirmations{
			Hash:          blk.BlockId,
			Confirmations: blkTipHeader.Height - blk.Height,
		})
	}
}

// BlockToLegacyBlock converts our blocc.Block format to the LegacyBlock format
func BlockToLegacyBlock(blk *blocc.Block) *LegacyBlock {

	return &LegacyBlock{
		Hash:              blk.BlockId,
		Height:            blk.Height,
		Size:              blk.BlockSize,
		StrippedSize:      cast.ToInt64(blk.DataValue("stripped_size")),
		Weight:            cast.ToInt64(blk.DataValue("weight")),
		Version:           cast.ToInt64(blk.DataValue("version")),
		MerkleRoot:        blk.DataValue("merkle_root"),
		Time:              blk.Time,
		Nonce:             cast.ToInt64(blk.DataValue("nonce")),
		Bits:              cast.ToInt64(blk.DataValue("bits")),
		Difficulty:        cast.ToFloat64(blk.DataValue("difficulty")),
		PreviousBlockHash: blk.PrevBlockId,
		NextBlockHash:     blk.NextBlockId,
		Connected:         true,
		ReceivedTime:      blk.Time,
		VersionHex:        blk.DataValue("version_hex"),
		Tx:                blk.TxIds,
	}

}

// LegacyGetBlockStats returns the block stats based on id
func (s *Server) LegacyGetBlockStats() http.HandlerFunc {

	type legacyTxStats struct {
		Fees          int64   `json:"fees"`
		FeesPerByte   float32 `json:"fees_per_byte"`
		FeesPerWeight float32 `json:"fees_per_weight"`
		Size          int64   `json:"size"`
		VSize         int64   `json:"vsize"`
		Weight        int64   `json:"weight"`
		VinValue      int64   `json:"vin_value"`
		VoutValue     int64   `json:"vout_value"`
	}

	type legacyBlockStats struct {
		Hash          string                    `json:"hash"`
		Size          int64                     `json:"size"`
		StrippedSize  int64                     `json:"strippedsize"`
		Weight        int64                     `json:"weight"`
		CoinbaseValue int64                     `json:"coinbase_value"`
		Fees          int64                     `json:"fees"`
		FeesPerByte   float32                   `json:"fees_per_byte"`
		FeesPerWeight float32                   `json:"fees_per_weight"`
		Miner         string                    `json:"miner"`
		Reward        int64                     `json:"reward"`
		VinValue      int64                     `json:"vin_value"`
		VoutValue     int64                     `json:"vout_value"`
		Transactions  map[string]*legacyTxStats `json:"transactions"`
	}

	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")

		// Get the block
		blk, err := s.blockChainStore.GetBlockByBlockId(btc.Symbol, id, blocc.BlockIncludeAllButRaw)
		if err != nil {
			render.Render(w, r, s.ErrInternalLog(err))
			return
		}

		// Get the transactions
		txs, err := s.blockChainStore.GetTxsByBlockId(btc.Symbol, id, blocc.TxIncludeHeader|blocc.TxIncludeData)
		if err != nil {
			render.Render(w, r, s.ErrInternalLog(err))
			return
		}

		blkFees := cast.ToInt64(blk.DataValue("fee"))
		blkWeight := cast.ToInt64(blk.DataValue("weight"))

		ret := &legacyBlockStats{
			Hash:          blk.BlockId,
			Size:          blk.BlockSize,
			StrippedSize:  cast.ToInt64(blk.DataValue("stripped_size")),
			Weight:        blkWeight,
			Fees:          blkFees,
			FeesPerByte:   float32(blkFees) / cast.ToFloat32(blk.DataValue("size")),
			FeesPerWeight: float32(blkFees) / cast.ToFloat32(blk.DataValue("weight")),
			CoinbaseValue: cast.ToInt64(blk.DataValue("coinbase_value")),
			Reward:        cast.ToInt64(blk.DataValue("coinbase_value")) - blkFees,
			VinValue:      cast.ToInt64(blk.DataValue("input_value")),
			VoutValue:     cast.ToInt64(blk.DataValue("output_value")),
			Transactions:  make(map[string]*legacyTxStats),
		}

		if blk.BlockSize != 0 {
			ret.FeesPerByte = float32(blkFees) / float32(blk.BlockSize)
		}
		if blkWeight != 0 {
			ret.FeesPerWeight = float32(blkFees) / float32(blkWeight)
		}

		max := &legacyTxStats{
			Size:          0,
			VSize:         0,
			Fees:          0,
			FeesPerByte:   0,
			FeesPerWeight: 0.0,
			Weight:        0,
			VinValue:      0,
			VoutValue:     0,
		}
		min := &legacyTxStats{
			Size:          9223372036854775807,
			VSize:         9223372036854775807,
			Fees:          9223372036854775807,
			FeesPerByte:   math.MaxFloat32,
			FeesPerWeight: math.MaxFloat32,
			Weight:        9223372036854775807,
			VinValue:      9223372036854775807,
			VoutValue:     9223372036854775807,
		}
		ret.Transactions["max"] = max
		ret.Transactions["min"] = min

		if len(txs) > 1 {

			var avgSize = new(big.Int)
			var avgVSize = new(big.Int)
			var avgFees = new(big.Int)
			var avgFeesPerByte = new(big.Float)
			var avgFeesPerWeight = new(big.Float)
			var avgWeight = new(big.Int)
			var avgInValue = new(big.Int)
			var avgOutValue = new(big.Int)

			for _, tx := range txs {

				// Skip the coinbase transaction
				if cast.ToBool(tx.DataValue("coinbase")) {
					continue
				}

				vsize := cast.ToInt64(tx.DataValue("vsize"))
				fee := cast.ToInt64(tx.DataValue("fee"))
				weight := cast.ToInt64(tx.DataValue("weight"))
				feePerByte := float32(fee) / float32(tx.TxSize)
				feePerWeight := float32(fee) / float32(weight)
				inValue := cast.ToInt64(tx.DataValue("in_value"))
				outValue := cast.ToInt64(tx.DataValue("out_value"))

				// MAX
				if tx.TxSize > max.Size {
					max.Size = tx.TxSize
				}
				if vsize > max.VSize {
					max.VSize = vsize
				}
				if fee > max.Fees {
					max.Fees = fee
				}
				if feePerByte > max.FeesPerByte {
					max.FeesPerByte = feePerByte
				}
				if feePerWeight > max.FeesPerWeight {
					max.FeesPerWeight = feePerWeight
				}
				if weight > max.Weight {
					max.Weight = weight
				}
				if inValue > max.VinValue {
					max.VinValue = inValue
				}
				if outValue > max.VoutValue {
					max.VoutValue = outValue
				}

				// MIN
				if tx.TxSize < min.Size {
					min.Size = tx.TxSize
				}
				if vsize < min.VSize {
					min.VSize = vsize
				}
				if fee < min.Fees {
					min.Fees = fee
				}
				if feePerByte < min.FeesPerByte {
					min.FeesPerByte = feePerByte
				}
				if feePerWeight < min.FeesPerWeight {
					min.FeesPerWeight = feePerWeight
				}
				if weight < min.Weight {
					min.Weight = weight
				}
				if inValue < min.VinValue {
					min.VinValue = inValue
				}
				if outValue < min.VoutValue {
					min.VoutValue = outValue
				}

				// AVG
				avgSize.Add(avgSize, big.NewInt(tx.TxSize))
				avgVSize.Add(avgVSize, big.NewInt(vsize))
				avgFees.Add(avgFees, big.NewInt(fee))
				avgFeesPerByte.Add(avgFeesPerByte, big.NewFloat(float64(feePerByte)))
				avgFeesPerWeight.Add(avgFeesPerWeight, big.NewFloat(float64(feePerWeight)))
				avgWeight.Add(avgWeight, big.NewInt(weight))
				avgInValue.Add(avgInValue, big.NewInt(inValue))
				avgOutValue.Add(avgOutValue, big.NewInt(outValue))
			}

			count := big.NewInt(int64(len(txs) - 1))
			countFloat := big.NewFloat(float64(len(txs) - 1))
			afpb, _ := avgFeesPerByte.Quo(avgFeesPerByte, countFloat).Float32()
			afpw, _ := avgFeesPerWeight.Quo(avgFeesPerWeight, countFloat).Float32()
			ret.Transactions["avg"] = &legacyTxStats{
				Size:          avgSize.Div(avgSize, count).Int64(),
				VSize:         avgVSize.Div(avgVSize, count).Int64(),
				Fees:          avgFees.Div(avgFees, count).Int64(),
				FeesPerByte:   afpb,
				FeesPerWeight: afpw,
				Weight:        avgWeight.Div(avgWeight, count).Int64(),
				VinValue:      avgInValue.Div(avgInValue, count).Int64(),
				VoutValue:     avgOutValue.Div(avgOutValue, count).Int64(),
			}

		} else {
			// All Zeros
			ret.Transactions["min"] = max
			ret.Transactions["avg"] = max
		}

		render.JSON(w, r, ret)
	}
}
