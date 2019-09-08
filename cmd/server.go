package cmd

import (
	cli "github.com/spf13/cobra"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/blocc/bloccserver"
	"git.coinninja.net/backend/blocc/blocc/legacyserver"
	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server"
	"git.coinninja.net/backend/blocc/store/esearch"
	"git.coinninja.net/backend/blocc/store/redis"
)

func init() {
	rootCmd.AddCommand(serverCmd)
}

var (
	serverCmd = &cli.Command{
		Use:   "server",
		Short: "Blocc Server",
		Long:  `Blocc Server`,
		Run: func(cmd *cli.Command, args []string) { // Initialize the databse

			// Setup the BlockStore
			var blockChainStore blocc.BlockChainStore
			var txBus blocc.TxBus

			// Everything uses redis
			r, err := redis.New()
			if err != nil {
				logger.Fatalw("TxPool/TxBus Error", "error", err)
			}

			// Elastic will implement BlockStore
			blockChainStore, err = esearch.NewBlockChainStore()
			if err != nil {
				logger.Fatalw("BlockStore Error", "error", err)
			}

			// Redis will also implement the message bus
			txBus = r.Prefix("mbus")

			// Create the blocc GRPC Server
			bs, err := bloccserver.New(blockChainStore, txBus, r.Prefix("scache"))
			if err != nil {
				logger.Fatalw("Could not create bloccserver", "error", err)
			}

			// Create the server
			s, err := server.New()
			if err != nil {
				logger.Fatalw("Could not create server",
					"error", err,
				)
			}

			blocc.RegisterBloccRPCServer(s.GRPCServer(), bs)
			s.GwReg(blocc.RegisterBloccRPCHandlerFromEndpoint)

			_, err = legacyserver.New(s.Router(), blockChainStore)
			if err != nil {
				logger.Fatalw("Could not create legacy server", "error", err)
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
