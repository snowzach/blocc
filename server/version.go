package server

import (
	"context"

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
