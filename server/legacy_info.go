package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/spf13/cast"
	config "github.com/spf13/viper"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
)

const (
	blockChainInfoPathInfo = iota
	blockChainInfoPathFees
)

// LegacyGetBlockChainInfo returns the block height and fees (in satoshis per byte)
func (s *Server) LegacyGetBlockChainInfo(path int) http.HandlerFunc {

	type blockFees struct {
		Avg float64 `json:"avg"`
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	}

	type blockChainInfo struct {
		Blocks int64     `json:"blocks"`
		Fees   blockFees `json:"fees"`
	}

	return func(w http.ResponseWriter, r *http.Request) {

		// Get the top 3 blocks
		blks, err := s.blockChainStore.FindBlocksByBlockIdsAndTime(btc.Symbol, nil, nil, nil, blocc.BlockIncludeHeader|blocc.BlockIncludeData, 0, 3)
		if err == blocc.ErrNotFound {
			render.Render(w, r, ErrNotFound)
			return
		} else if err != nil && !blocc.IsValidationError(err) {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		} else if len(blks) != 3 {
			render.Render(w, r, s.ErrInternalLog(fmt.Errorf("Did not get top 3 blocks when collecting fees")))
			return
		}

		var count float64
		var txTotalSize float64
		var txTotalCount float64
		var info blockChainInfo

		for _, blk := range blks {

			// Get the block height. It will be the first block but we'll do this to make it simple and reliable
			if blk.Height > info.Blocks {
				info.Blocks = blk.Height + 1 // This endpoint was expected to return the count of blocks (genesis block = block 0 so add 1)
			}

			// Make sure this is a block that actually has fees in it
			if cast.ToInt64(blk.DataValue("fee")) == 0 {
				continue
			}

			info.Fees.Avg += cast.ToFloat64(blk.DataValue("fee_avg"))
			info.Fees.Min += cast.ToFloat64(blk.DataValue("fee_min"))
			info.Fees.Max += cast.ToFloat64(blk.DataValue("fee_max"))

			count += 1.0
			txTotalSize += cast.ToFloat64(blk.BlockSize - 88)
			txTotalCount += cast.ToFloat64(blk.TxCount)
		}

		if count == 0.0 {
			render.Render(w, r, s.ErrInternalLog(fmt.Errorf("Did not get any fee blocks when collecting fees")))
			return
		}

		avgTxSize := txTotalSize / txTotalCount

		info.Fees.Avg = (info.Fees.Avg / count) / avgTxSize
		info.Fees.Min = (info.Fees.Min / count) / avgTxSize
		info.Fees.Max = (info.Fees.Max / count) / avgTxSize

		// Cap the fees
		if minMaxFee := config.GetFloat64("server.legacy.btc_min_fee_max"); minMaxFee > 0 && info.Fees.Min > minMaxFee {
			info.Fees.Min = minMaxFee
		}
		if avgMaxFee := config.GetFloat64("server.legacy.btc_avg_fee_max"); avgMaxFee > 0 && info.Fees.Avg > avgMaxFee {
			info.Fees.Avg = avgMaxFee
		}
		if maxMaxFee := config.GetFloat64("server.legacy.btc_max_fee_max"); maxMaxFee > 0 && info.Fees.Max > maxMaxFee {
			info.Fees.Max = maxMaxFee
		}

		// Quick and dirty hack to provide the average fee as the minimum one since this is what the drop bit app uses
		if config.GetBool("server.legacy.btc_avg_fee_as_min") {
			info.Fees.Min = info.Fees.Avg
		}

		if path == blockChainInfoPathFees {
			// Return just the fees portion
			fmt.Println(info.Fees)
			render.JSON(w, r, info.Fees)
		} else {
			// Return the full info object
			render.JSON(w, r, info)

		}
	}

}
