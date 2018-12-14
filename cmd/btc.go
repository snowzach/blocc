package cmd

import (
	cli "github.com/spf13/cobra"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/btc"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server"
	"git.coinninja.net/backend/blocc/store"
	"git.coinninja.net/backend/blocc/store/esearch"
	"git.coinninja.net/backend/blocc/store/redis"
)

func init() {
	rootCmd.AddCommand(btcCmd)

	btcCmd.PersistentFlags().BoolVarP(&btcCmdServer, "server", "s", false, "Start the webserver")
	btcCmd.PersistentFlags().BoolVarP(&btcCmdBlocks, "blocks", "b", false, "Start the block extractor")
	btcCmd.PersistentFlags().BoolVarP(&btcCmdTxns, "transactions", "t", false, "Start the txn extractor")

	config.BindPFlag("extractor.btc.blocks", btcCmd.PersistentFlags().Lookup("blocks"))
	config.BindPFlag("extractor.btc.transactions", btcCmd.PersistentFlags().Lookup("transactions"))
}

var (
	btcCmdServer bool
	btcCmdBlocks bool
	btcCmdTxns   bool

	btcCmd = &cli.Command{
		Use:   "btc",
		Short: "BTC Extractor",
		Long:  `BTC Extractor`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			var err error

			// Setup the BlockStore
			var bcs blocc.BlockChainStore
			var txp blocc.TxPool
			var txb blocc.TxBus
			var ms blocc.MetricStore

			// Elastic will implement BlockStore
			if btcCmdBlocks {
				es, err := esearch.New()
				if err != nil {
					logger.Fatalw("BlockStore Error", "error", err)
				}
				// Elastic will implement BlockStore
				bcs = es
				// Elastic will implement MetricStore
				ms = es
			}

			// Redis will implement the TxPool/TxBus
			if btcCmdTxns || btcCmdServer {
				r, err := redis.New("mempool")
				if err != nil {
					logger.Fatalw("TxPool/TxBus Error", "error", err)
				}

				// Redis implents TxStore
				txp = r
				// Redis will also implement the message bus
				txb = r
			}

			// Redis will implement a dist cache if we are running the server
			var dc store.DistCache
			if btcCmdServer {
				dc, err = redis.New("distcache")
				if err != nil {
					logger.Fatalw("DistCache Error", "error", err)
				}
			}

			// Start the extractor
			if btcCmdBlocks || btcCmdTxns {
				_, err = btc.Extract(bcs, txp, txb, ms)
				if err != nil {
					logger.Fatalw("Could not create Extractor",
						"error", err,
					)
				}
			}

			//  Also start the web server
			if btcCmdServer {
				// Create the server
				s, err := server.New(dc, txp, txb)
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
			}

			<-conf.Stop.Chan() // Wait until StopChan
			conf.Stop.Wait()   // Wait until everyone cleans up
			zap.L().Sync()     // Flush the logger

		},
	}
)
