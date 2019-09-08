package versionrpcserver

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	emptypb "github.com/golang/protobuf/ptypes/empty"

	"git.coinninja.net/backend/blocc/conf"
	"git.coinninja.net/backend/blocc/server/versionrpc"
)

type versionRPCServer struct{}

// New returns a new version server
func New() versionrpc.VersionRPCServer {
	return versionRPCServer{}
}

// Version returns the version
func (vs versionRPCServer) Version(ctx context.Context, _ *emptypb.Empty) (*versionrpc.VersionResponse, error) {

	return &versionrpc.VersionResponse{
		Version: conf.GitVersion,
	}, nil

}

// GetVersion returns the version git version of the code
func GetVersion() http.HandlerFunc {

	// Simple version struct
	type version struct {
		Version string `json:"version"`
	}
	var v = &version{Version: conf.GitVersion}

	return func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, v)
	}
}
