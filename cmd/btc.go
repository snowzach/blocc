package cmd

import (
	cli "github.com/spf13/cobra"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
	"git.coinninja.net/backend/blocc/cache/redis"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/store/esearch"
)

func init() {
	rootCmd.AddCommand(btcCmd)
}

var (
	btcCmd = &cli.Command{
		Use:   "btc",
		Short: "BTC Extractor",
		Long:  `BTC Extractor`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			var err error

			// Setup the BlockStore
			var bs blocc.BlockStore
			var ms blocc.MetricStore
			var ts blocc.TxStore

			// Elastic will implement BlockStore
			bs, err = esearch.New()
			if err != nil {
				logger.Fatalw("BlockStore Error", "error", err)
			}

			// Redis will implement the TxStore/MemPool
			ts, err = redis.New(btc.Symbol + ":mempool:")
			if err != nil {
				logger.Fatalw("BlockCache Error", "error", err)
			}

			// Elastic will also implement MetricStore
			ms = bs

			// Start the extractor
			_, err = btc.Extract(bs, ts, ms)
			if err != nil {
				logger.Fatalw("Could not create Extractor",
					"error", err,
				)
			}

			<-conf.Stop.Chan() // Wait until StopChan
			conf.Stop.Wait()   // Wait until everyone cleans up
			zap.L().Sync()     // Flush the logger

		},
	}
)
