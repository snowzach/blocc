package bloccserver

import (
	"time"

	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

type Server struct {
	logger *zap.SugaredLogger

	defaultSymbol string
	defaultCount  int

	distCache    store.DistCache
	cacheTimeout time.Duration

	blockChainStore blocc.BlockChainStore
	txBus           blocc.TxBus
}

func New(blockChainStore blocc.BlockChainStore, txBus blocc.TxBus, distCache store.DistCache) (*Server, error) {

	return &Server{
		logger: zap.S().With("package", "bloccserver"),

		defaultSymbol: config.GetString("server.default_symbol"),
		defaultCount:  config.GetInt("server.default_count"),

		distCache:    distCache,
		cacheTimeout: config.GetDuration("server.cache_duration"),

		blockChainStore: blockChainStore,
		txBus:           txBus,
	}, nil

}
