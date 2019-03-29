package cmd

import (
	cli "github.com/spf13/cobra"

	"git.coinninja.net/backend/blocc/store/esearch"
)

func init() {
	rootCmd.AddCommand(btcFixCmd)
}

var (
	btcFixCmd = &cli.Command{
		Use:   "btcfix",
		Short: "BTC FIX",
		Long:  `BTC FIX`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			es, err := esearch.New()
			if err != nil {
				logger.Fatalw("BlockStore Error", "error", err)
			}

			es.Fix()

		},
	}
)
