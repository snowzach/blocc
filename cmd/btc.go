package cmd

import (
	cli "github.com/spf13/cobra"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server"
	"git.coinninja.net/backend/blocc/store/esearch"
	"git.coinninja.net/backend/blocc/store/redis"
)

func init() {
	rootCmd.AddCommand(btcCmd)

	btcCmd.PersistentFlags().BoolVarP(&btcCmdBlocks, "block", "b", false, "Start the block extractor")
	btcCmd.PersistentFlags().BoolVarP(&btcCmdTxns, "transaction", "t", false, "Start the txn extractor")
	btcCmd.PersistentFlags().BoolVarP(&btcCmdHealth, "health", "", false, "Start the health server")

	// Bind these to environment variables
	config.BindPFlag("extractor.btc.block", btcCmd.PersistentFlags().Lookup("block"))
	config.BindPFlag("extractor.btc.transaction", btcCmd.PersistentFlags().Lookup("transaction"))
	config.BindPFlag("extractor.btc.health", btcCmd.PersistentFlags().Lookup("health"))
}

var (
	btcCmdBlocks bool
	btcCmdTxns   bool
	btcCmdHealth bool

	btcCmd = &cli.Command{
		Use:   "btc",
		Short: "BTC Extractor",
		Long:  `BTC Extractor`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			var err error

			// Setup the BlockStore
			var blockChainStore blocc.BlockChainStore
			var txBus blocc.TxBus

			// Everything uses redis
			r, err := redis.New()
			if err != nil {
				logger.Fatalw("TxPool/TxBus Error", "error", err)
			}

			// Elastic will implement BlockStore
			blockChainStore, err = esearch.New()
			if err != nil {
				logger.Fatalw("BlockStore Error", "error", err)
			}

			// Redis will implement the TxPool/TxBus
			if btcCmdTxns {
				// Redis will also implement the message bus
				txBus = r.Prefix("mbus")
			}

			// Start the extractor
			_, err = btc.Extract(blockChainStore, txBus)
			if err != nil {
				logger.Fatalw("Could not create Extractor",
					"error", err,
				)
			}

			// If we requested a health server, also start it
			if btcCmdHealth {

				s, err := server.NewHealthServer()
				if err != nil {
					logger.Fatalw("Could not create health server",
						"error", err,
					)
				}
				err = s.ListenAndServe()
				if err != nil {
					logger.Fatalw("Could not start health server",
						"error", err,
					)
				}

			}

			<-conf.Stop.Chan() // Wait until StopChan
			conf.Stop.Wait()   // Wait until everyone cleans up
			zap.L().Sync()     // Flush the logger

		},
	}
)
