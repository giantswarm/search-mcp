package auth

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrTokenNotFound = errors.New("no valid authentication token found")
	ErrTokenExpired  = errors.New("authentication token expired")
	ErrRefreshFailed = errors.New("token refresh failed")
	ErrNotConfigured = errors.New("authentication not configured")
	ErrInvalidToken  = errors.New("invalid authentication token")
)

// AuthManager defines the interface for authentication management
type AuthManager interface {
	// GetToken returns a valid access token, refreshing if necessary
	GetToken(ctx context.Context) (string, error)

	// IsAuthenticated checks if user has valid credentials
	IsAuthenticated() bool

	// InitiateAuth starts OAuth flow, returns auth URL for user
	InitiateAuth(ctx context.Context) (authURL string, err error)

	// HandleCallback processes OAuth callback and stores tokens
	HandleCallback(ctx context.Context, code string, state string) error

	// ClearTokens removes stored tokens (logout)
	ClearTokens() error
}

// Config holds configuration for authentication
type Config struct {
	// OAuth configuration
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURI  string // Optional, will be auto-generated if empty
	Scopes       []string

	// Server configuration
	ServerAddr string // Used for generating auth URLs and redirect URIs
}

// TokenData represents stored OAuth tokens
type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	IDToken      string    `json:"id_token,omitempty"`
}

// IsValid checks if token is present and not expired
func (t *TokenData) IsValid() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	return time.Now().Before(t.Expiry)
}

// NeedsRefresh checks if token should be proactively refreshed
// Returns true if token expires in less than 20% of its lifetime
func (t *TokenData) NeedsRefresh() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}

	now := time.Now()
	timeUntilExpiry := t.Expiry.Sub(now)

	// Refresh if less than 12 minutes remaining (20% of assumed 1hr lifetime)
	refreshThreshold := 12 * time.Minute

	return timeUntilExpiry < refreshThreshold
}
