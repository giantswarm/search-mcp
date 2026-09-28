package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/giantswarm/search-mcp/internal/auth/authtest"
)

const audience = "searchmcp"

func TestForwardedTokenVerifier_Verify(t *testing.T) {
	issuer := authtest.NewIssuer(t)
	other := authtest.NewIssuer(t)
	verifier, err := auth.NewForwardedTokenVerifier(context.Background(),
		auth.ForwardedConfig{IssuerURL: issuer.URL, Audience: audience})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{"valid", issuer.Token(t, issuer.URL, audience, "alice", time.Hour), nil},
		{"empty", "", auth.ErrTokenNotFound},
		{"wrong audience", issuer.Token(t, issuer.URL, "other-client", "alice", time.Hour), auth.ErrInvalidToken},
		{"wrong issuer", issuer.Token(t, "https://elsewhere.example", audience, "alice", time.Hour), auth.ErrInvalidToken},
		{"expired", issuer.Token(t, issuer.URL, audience, "alice", -time.Minute), auth.ErrInvalidToken},
		{"foreign signature", other.Token(t, issuer.URL, audience, "alice", time.Hour), auth.ErrInvalidToken},
		{"not a token", "not-a-jwt", auth.ErrInvalidToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifier.Verify(context.Background(), tt.token)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Verify() = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Verify() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewForwardedTokenVerifier_RequiresIssuerAndAudience(t *testing.T) {
	issuer := authtest.NewIssuer(t)
	for _, config := range []auth.ForwardedConfig{
		{IssuerURL: issuer.URL},
		{Audience: audience},
	} {
		if _, err := auth.NewForwardedTokenVerifier(context.Background(), config); err == nil {
			t.Errorf("NewForwardedTokenVerifier(%+v) succeeded, want an error", config)
		}
	}
}
