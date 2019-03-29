package server

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	emptypb "github.com/golang/protobuf/ptypes/empty"

	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server/rpc"
)

// Version returns the version
func (s *Server) Version(ctx context.Context, _ *emptypb.Empty) (*rpc.VersionResponse, error) {

	return &rpc.VersionResponse{
		Version: conf.GitVersion,
	}, nil

}

// GetVersion returns the version git version of the code
func (s *Server) GetVersion() http.HandlerFunc {

	// Simple version struct
	type version struct {
		Version string `json:"version"`
	}
	var v = &version{Version: conf.GitVersion}

	return func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, v)
	}
}
