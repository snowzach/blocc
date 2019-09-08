package legacyserver

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/server"
)

type Server struct {
	logger *zap.SugaredLogger

	defaultCount int

	blockChainStore blocc.BlockChainStore
}

func New(router chi.Router, blockChainStore blocc.BlockChainStore) (*Server, error) {

	s := &Server{
		logger: zap.S().With("package", "legacyserver"),

		defaultCount: config.GetInt("server.default_count"),

		blockChainStore: blockChainStore,
	}

	// This is all of the legacy functions that are REST only and produce output compatiable with btc-api
	router.Route("/legacy", func(legacy chi.Router) {
		legacy.Get("/addresses/{address}", s.LegacyFindAddressTransactions(http.MethodGet))
		legacy.Post("/addresses/query", s.LegacyFindAddressTransactions(http.MethodPost))
		legacy.Get("/addresses/{address}/stats", s.LegacyGetAddressStats())

		legacy.Get("/blocks", s.LegacyFindBlocks(http.MethodGet))
		legacy.Get("/blocks/", s.LegacyFindBlocks(http.MethodGet))
		legacy.Post("/blocks/query", s.LegacyFindBlocks(http.MethodPost))
		legacy.Get("/blocks/{id}", s.LegacyGetBlock())
		legacy.Get("/blocks/info", s.LegacyGetBlockChainInfo(blockChainInfoPathInfo))
		legacy.Get("/blocks/{id}/confirmations", s.LegacyGetBlockConfirmations())
		legacy.Get("/blocks/{id}/stats", s.LegacyGetBlockStats())

		legacy.Get("/transactions/{id}", s.LegacyGetTx())
		legacy.Post("/transactions/query", s.LegacyFindTxIds())
		legacy.Get("/transactions/{id}/confirmations", s.LegacyGetTxConfirmations())
		legacy.Get("/transactions/{id}/stats", s.LegacyGetTxStats())

		legacy.Get("/fees", s.LegacyGetBlockChainInfo(blockChainInfoPathFees))
	})

	return s, nil

}

// ErrInternalLog will log an error and return a generic server error to the user
func (s *Server) ErrInternalLog(err error) render.Renderer {
	s.logger.Errorw("Server Error", "error", err)
	return server.ErrInternal(err)
}
