package imgworker

import (
	"context"

	"github.com/docker/docker/api/types"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
)

func NewDockerAuthProvider(auth *types.AuthConfig) session.Attachable {
	return &authProvider{
		config: auth,
	}
}

type authProvider struct {
	config *types.AuthConfig
}

func (ap *authProvider) Register(server *grpc.Server) {
	// no-op
}

func (ap *authProvider) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	res := &auth.CredentialsResponse{}
	if ap.config.IdentityToken != "" {
		res.Secret = ap.config.IdentityToken
	} else {
		res.Username = ap.config.Username
		res.Secret = ap.config.Password
	}
	return res, nil
}
