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

		Fast float64 `json:"fast"`
		Med  float64 `json:"med"`
		Slow float64 `json:"slow"`
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

			// Use the median or the average for the fee, whichever is lower
			median := cast.ToFloat64(blk.DataValue("fee_median"))
			if median > 0 && median < cast.ToFloat64(blk.DataValue("fee_avg")) {
				info.Fees.Avg += median
			} else {
				info.Fees.Avg += cast.ToFloat64(blk.DataValue("fee_avg"))
			}

			info.Fees.Min += cast.ToFloat64(blk.DataValue("fee_min"))
			info.Fees.Max += cast.ToFloat64(blk.DataValue("fee_max"))

			count += 1.0
			txTotalSize += cast.ToFloat64(blk.BlockSize - 88)
			txTotalCount += cast.ToFloat64(blk.TxCount)
		}

		// If test mode, return hard coded fees
		if config.GetBool("server.legacy.btc_fee_testing") {
			info.Fees.Min = 1
			info.Fees.Avg = 2
			info.Fees.Max = 3
			info.Fees.Slow = 1.1
			info.Fees.Med = 2.2
			info.Fees.Fast = 3.1

			if path == blockChainInfoPathFees {
				// Return just the fees portion
				render.JSON(w, r, info.Fees)
			} else {
				// Return the full info object
				render.JSON(w, r, info)
			}
			return
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

		// The fast fee will be the average of the 10th percentile of the last 3 blocks
		info.Fees.Fast, err = s.blockChainStore.AverageBlockDataFieldByHeight(btc.Symbol, "data.fee_vsize_p10", true, info.Blocks-4, blocc.HeightUnknown)
		if err == blocc.ErrNotFound {
			info.Fees.Fast = info.Fees.Avg
		} else if err != nil {
			render.Render(w, r, s.ErrInternalLog(fmt.Errorf("Error AverageBlockDataFieldByHeight: %v", err)))
			return
		}

		// The slow fee will be the 10th percentile of the 10th percentile of the last 144 blocks
		info.Fees.Slow, err = s.blockChainStore.PercentileBlockDataFieldByHeight(btc.Symbol, "data.fee_vsize_p10", 10.0, true, info.Blocks-145, blocc.HeightUnknown)
		if err == blocc.ErrNotFound {
			info.Fees.Slow = info.Fees.Avg
		} else if err != nil {
			render.Render(w, r, s.ErrInternalLog(fmt.Errorf("Error PercentileBlockDataFieldByHeight: %v", err)))
			return
		} else {
			// If the fast fee is somehow better than this fee, use it instead, and then increase the fast fee by a couple sats
			// This should basically never happen
			if info.Fees.Fast < info.Fees.Slow {
				info.Fees.Slow = info.Fees.Fast
			}
		}

		// Now calculate the medium fee as 2/3 the way to the fast fee from the slow fee
		info.Fees.Med = (((info.Fees.Fast - info.Fees.Slow) / 3) * 2) + info.Fees.Slow

		// Make extra sure the fees didn't end up the same or in the wrong order if there are rounding errors
		if info.Fees.Med <= info.Fees.Slow {
			info.Fees.Med += (info.Fees.Slow - info.Fees.Med) + 1
		}
		if info.Fees.Fast <= info.Fees.Med {
			info.Fees.Fast += (info.Fees.Med - info.Fees.Fast) + 1
		}

		// Quick and dirty hack to provide the average fee as the minimum one since this is what the drop bit app uses
		if config.GetBool("server.legacy.btc_avg_fee_as_min") {
			info.Fees.Min = info.Fees.Avg
		}

		// This will substitute the p10 fees for the average and min fees
		if config.GetBool("server.legacy.btc_use_p10_fee") {
			info.Fees.Min = info.Fees.Fast
			info.Fees.Avg = info.Fees.Fast
		}

		if path == blockChainInfoPathFees {
			// Return just the fees portion
			render.JSON(w, r, info.Fees)
		} else {
			// Return the full info object
			render.JSON(w, r, info)

		}
	}

}
