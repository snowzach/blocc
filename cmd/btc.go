package cmd

import (
	cli "github.com/spf13/cobra"
	"go.uber.org/zap"

	config "github.com/spf13/viper"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server"
	"git.coinninja.net/backend/blocc/store/esearch"
	"git.coinninja.net/backend/blocc/store/redis"
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
			if config.GetString("elasticsearch.host") != "" {
				bs, err = esearch.New()
				if err != nil {
					logger.Fatalw("BlockStore Error", "error", err)
				}

				// Elastic will also implement MetricStore
				ms = bs
			}

			// Redis will implement the TxStore/MemPool
			if config.GetString("redis.host") != "" {
				ts, err = redis.New(btc.Symbol, "mempool")
				if err != nil {
					logger.Fatalw("BlockCache Error", "error", err)
				}
			}

			// Start the extractor
			_, err = btc.Extract(bs, ts, ms)
			if err != nil {
				logger.Fatalw("Could not create Extractor",
					"error", err,
				)
			}

			// Create the server
			s, err := server.New(ts)
			if err != nil {
				logger.Fatalw("Could not create server",
					"error", err,
				)
			}
			err = s.ListenAndServe()
			if err != nil {
				logger.Fatalw("Could not start server",
					"error", err,
				)
			}

			<-conf.Stop.Chan() // Wait until StopChan
			conf.Stop.Wait()   // Wait until everyone cleans up
			zap.L().Sync()     // Flush the logger

		},
	}
)
