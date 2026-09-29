package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// memoryTokenStorage is a minimal in-memory TokenStorage for tests.
type memoryTokenStorage struct {
	data *TokenData
}

func (s *memoryTokenStorage) Store(_ context.Context, tokens *TokenData) error {
	s.data = tokens
	return nil
}

func (s *memoryTokenStorage) Load(_ context.Context) (*TokenData, error) {
	if s.data == nil {
		return nil, ErrTokenNotFound
	}
	return s.data, nil
}

func (s *memoryTokenStorage) Delete() error {
	s.data = nil
	return nil
}

func (s *memoryTokenStorage) Exists() bool {
	return s.data != nil
}

// assertNoSecretsLogged fails the test if any of the given secret values
// appear anywhere in the log output.
func assertNoSecretsLogged(t *testing.T, logged string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(logged, secret) {
			t.Errorf("log output contains a raw token value %q:\n%s", secret, logged)
		}
	}
}

func TestTokenManager_StoreAndGetToken_NeverLogsTokenValues(t *testing.T) {
	const (
		accessToken  = "super-secret-access-token-value"  //nolint:gosec // G101: fake test value, not a real credential
		refreshToken = "super-secret-refresh-token-value" //nolint:gosec // G101: fake test value, not a real credential
		idToken      = "super-secret-id-token-value"      //nolint:gosec // G101: fake test value, not a real credential
	)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	manager := NewTokenManager(&memoryTokenStorage{}, &oauth2.Config{}, logger)

	if err := manager.StoreTokens(context.Background(), &TokenData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		IDToken:      idToken,
		TokenType:    testBearerTokenType,
		Expiry:       time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("StoreTokens: %v", err)
	}

	if _, err := manager.GetToken(context.Background()); err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	assertNoSecretsLogged(t, buf.String(), accessToken, refreshToken, idToken)
}

func TestTokenManager_Refresh_NeverLogsTokenValues(t *testing.T) {
	const (
		oldAccessToken  = "old-secret-access-token-value"  //nolint:gosec // G101: fake test value, not a real credential
		oldRefreshToken = "old-secret-refresh-token-value" //nolint:gosec // G101: fake test value, not a real credential
		newAccessToken  = "new-secret-access-token-value"  //nolint:gosec // G101: fake test value, not a real credential
		newRefreshToken = "new-secret-refresh-token-value" //nolint:gosec // G101: fake test value, not a real credential
		newIDToken      = "new-secret-id-token-value"      //nolint:gosec // G101: fake test value, not a real credential
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccessToken,
			"refresh_token": newRefreshToken,
			"token_type":    testBearerTokenType,
			"expires_in":    3600,
			"id_token":      newIDToken,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oauthConfig := &oauth2.Config{
		Endpoint: oauth2.Endpoint{TokenURL: srv.URL + "/token"},
	}
	storage := &memoryTokenStorage{}
	manager := NewTokenManager(storage, oauthConfig, logger)
	manager.currentToken = &TokenData{
		AccessToken:  oldAccessToken,
		RefreshToken: oldRefreshToken,
		TokenType:    testBearerTokenType,
		// Within the 12-minute proactive refresh threshold, but not expired.
		Expiry: time.Now().Add(5 * time.Minute),
	}

	if _, err := manager.GetToken(context.Background()); err != nil {
		t.Fatalf("GetToken: %v", err)
	}

	assertNoSecretsLogged(t, buf.String(),
		oldAccessToken, oldRefreshToken, newAccessToken, newRefreshToken, newIDToken)
}
