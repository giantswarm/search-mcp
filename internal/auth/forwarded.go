package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// ForwardedConfig configures the bearer tokens an MCP gateway forwards per
// request: the issuer that signs them and the audience they must carry.
type ForwardedConfig struct {
	IssuerURL string
	Audience  string
}

// Enabled reports whether any forwarded-token setting is present.
func (c ForwardedConfig) Enabled() bool {
	return c.IssuerURL != "" || c.Audience != ""
}

// ForwardedTokenVerifier validates a caller's forwarded bearer token. It keeps
// no token: each request's token is checked and used for that request only.
type ForwardedTokenVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewForwardedTokenVerifier discovers the issuer's signing keys. It fails when
// the issuer or the audience is missing or the issuer cannot be discovered.
func NewForwardedTokenVerifier(ctx context.Context, config ForwardedConfig) (*ForwardedTokenVerifier, error) {
	if config.IssuerURL == "" || config.Audience == "" {
		return nil, errors.New("forwarded tokens need both an issuer URL and an audience")
	}
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discovering issuer %s: %w", config.IssuerURL, err)
	}
	return &ForwardedTokenVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: config.Audience})}, nil
}

// Verify checks the token's signature, issuer, audience and expiry.
func (v *ForwardedTokenVerifier) Verify(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrTokenNotFound
	}
	if _, err := v.verifier.Verify(ctx, rawToken); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return nil
}
