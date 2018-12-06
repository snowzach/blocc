package server

import (
	"net/http"

	"git.coinninja.net/backend/blocc/server/rpc"
)

// SetupRoutes configures all the routes for this service
func (s *Server) SetupRoutes() {

	// Register our routes - you need at aleast one route
	s.router.Get("/none", func(w http.ResponseWriter, r *http.Request) {})

	// Register RPC Services
	rpc.RegisterVersionRPCServer(s.grpcServer, s)
	s.gwReg(rpc.RegisterVersionRPCHandlerFromEndpoint)
	rpc.RegisterMemPoolRPCServer(s.grpcServer, s)
	s.gwReg(rpc.RegisterMemPoolRPCHandlerFromEndpoint)

}
