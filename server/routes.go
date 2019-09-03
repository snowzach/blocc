package server

import (
	"net/http"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/go-chi/chi"

	"git.coinninja.net/backend/blocc/embed"
	"git.coinninja.net/backend/blocc/server/rpc"
	"git.coinninja.net/backend/blocc/blocc"
)

// SetupRoutes configures all the routes for this service
func (s *Server) SetupRoutes() {

	// Register our routes - you need at aleast one route
	s.router.Get("/none", func(w http.ResponseWriter, r *http.Request) {})

	// This is all of the legacy functions that are REST only and produce output compatiable with btc-api
	s.router.Route("/legacy", func(legacy chi.Router) {
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

	// Register RPC Services
	rpc.RegisterVersionRPCServer(s.grpcServer, s)
	s.gwReg(rpc.RegisterVersionRPCHandlerFromEndpoint)

	blocc.RegisterBloccRPCServer(s.grpcServer, s)
	s.gwReg(blocc.RegisterBloccRPCHandlerFromEndpoint)

	// Serve api-docs and swagger-ui
	fs := http.FileServer(&assetfs.AssetFS{Asset: embed.Asset, AssetDir: embed.AssetDir, AssetInfo: embed.AssetInfo, Prefix: "public"})
	s.router.Get("/api-docs/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
	s.router.Get("/swagger-ui/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))

}
