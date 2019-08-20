package cmd

import (
	"time"

	cli "github.com/spf13/cobra"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/store/esearch"
)

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.PersistentFlags().StringVarP(&deleteCmdSymbol, "symbol", "s", "", "The symbol to delete from")
	deleteCmd.PersistentFlags().Int64VarP(&deleteCmdAbove, "above", "a", -1, "Delete blocks above this block height")
}

var (
	deleteCmdSymbol string
	deleteCmdAbove  int64

	deleteCmd = &cli.Command{
		Use:   "delete",
		Short: "Delete Blocks",
		Long:  `This will delete blocks from the block store`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			var bcs blocc.BlockChainStore
			var err error

			// Connect to the store
			bcs, err = esearch.NewBlockChainStore()
			if err != nil {
				logger.Fatalw("BlockStore Error", "error", err)
			}

			// Verify we know what we want
			if deleteCmdSymbol == "" {
				logger.Fatal("You must specify a symbol. (You probably want to use '--symbol btc')")
			}

			if deleteCmdAbove == -1 {
				logger.Fatal("You must specify an --above <n> value. If you want to truly wipe the database, start the miner with ELASTICSEARCH_WIPE_CONFIRM=true")
			}

			logger.Warnw("DELETING ALL BLOCKS ABOVE", "symbol", deleteCmdSymbol, "above", deleteCmdAbove)

			// Make it obvious we are destroying things
			logger.Warn("***WARNING*** This is going to remove data from the database, be sure you know what you are doing ***WARNING***")
			for x := 5; x > 0 && !conf.Stop.Bool(); x-- {
				logger.Warnf("%d...", x)
				time.Sleep(time.Second)
			}
			if conf.Stop.Bool() {
				logger.Fatal("Aborting...")
			}
			logger.Warnf("Deleting above block height %d...", deleteCmdAbove)

			err = bcs.DeleteAboveBlockHeight(deleteCmdSymbol, deleteCmdAbove)
			if err != nil {
				logger.Info("If this error indicates a timeout, try waiting for a time and retry until it succeeds")
				logger.Fatalf("Could not DeleteAboveBlockHeight: %v", err)
			}

			logger.Info("Done...")

		},
	}
)
