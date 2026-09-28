// Package authtest runs an in-process OIDC issuer for tests.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Issuer serves OIDC discovery and a JWKS, and signs tokens with its key.
type Issuer struct {
	URL    string
	signer jose.Signer
}

// NewIssuer starts an issuer that stops with the test.
func NewIssuer(t *testing.T) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader("kid", "test").WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/keys",
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"},
		}})
	})
	return &Issuer{URL: srv.URL, signer: signer}
}

// Token signs a token for subject and audience that expires after ttl
// (negative for an expired one), issued by issuer.
func (i *Issuer) Token(t *testing.T, issuer, audience, subject string, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	token, err := jwt.Signed(i.signer).Claims(jwt.Claims{
		Issuer:   issuer,
		Subject:  subject,
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:   jwt.NewNumericDate(now.Add(ttl)),
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}
