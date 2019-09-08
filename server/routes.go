package server

import (
	"net/http"

	assetfs "github.com/elazarl/go-bindata-assetfs"

	"git.coinninja.net/backend/blocc/embed"
	"git.coinninja.net/backend/blocc/server/versionrpc"
	"git.coinninja.net/backend/blocc/server/versionrpc/versionrpcserver"
)

// SetupRoutes configures all the routes for this service
func (s *Server) SetupRoutes() {

	// Register our routes - you need at aleast one route
	s.router.Get("/none", func(w http.ResponseWriter, r *http.Request) {})

	// Register RPC Services
	versionrpc.RegisterVersionRPCServer(s.GRPCServer(), versionrpcserver.New())
	s.GwReg(versionrpc.RegisterVersionRPCHandlerFromEndpoint)

	// Serve api-docs and swagger-ui
	fs := http.FileServer(&assetfs.AssetFS{Asset: embed.Asset, AssetDir: embed.AssetDir, AssetInfo: embed.AssetInfo, Prefix: "public"})
	s.router.Get("/api-docs/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
	s.router.Get("/swagger-ui/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))

}
