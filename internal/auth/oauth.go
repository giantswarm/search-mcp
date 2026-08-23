package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// Manager implements AuthManager interface using OAuth 2.1
type Manager struct {
	config       Config
	oauthConfig  *oauth2.Config
	tokenManager *TokenManager
	logger       *slog.Logger
	stateStore   map[string]bool // Simple in-memory state validation
	deviceStore  *DeviceFlowStore
}

// NewManager creates a new OAuth authentication manager
func NewManager(config Config, logger *slog.Logger) (AuthManager, error) {
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Auto-generate redirect URI if not provided
	if config.RedirectURI == "" {
		config.RedirectURI = generateRedirectURI(config.ServerAddr)
		logger.Debug("auto-generated redirect URI", "uri", config.RedirectURI)
	}

	// Create OAuth config
	// Trim trailing slash from issuer URL to avoid double slashes in endpoints
	issuerURL := strings.TrimSuffix(config.IssuerURL, "/")
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Scopes:       config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   issuerURL + "/auth",
			TokenURL:  issuerURL + "/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}

	// Create token storage
	storage, err := NewFileTokenStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to create token storage: %w", err)
	}

	// Create token manager
	tokenManager := NewTokenManager(storage, oauthConfig, logger)

	// Create device flow store
	deviceStore := NewDeviceFlowStore()

	return &Manager{
		config:       config,
		oauthConfig:  oauthConfig,
		tokenManager: tokenManager,
		logger:       logger,
		stateStore:   make(map[string]bool),
		deviceStore:  deviceStore,
	}, nil
}

// GetToken returns a valid access token, refreshing if necessary
func (m *Manager) GetToken(ctx context.Context) (string, error) {
	m.logger.Debug("GetToken called")
	token, err := m.tokenManager.GetToken(ctx)
	if err != nil {
		m.logger.Debug("GetToken failed", "error", err)
		// Provide detailed error message based on error type
		return "", m.formatError(err)
	}

	m.logger.Debug("GetToken returning token", "token", token)
	return token, nil
}

// IsAuthenticated checks if user has valid credentials
func (m *Manager) IsAuthenticated() bool {
	return m.tokenManager.HasValidToken(context.Background())
}

// InitiateAuth starts OAuth flow and returns authorization URL
func (m *Manager) InitiateAuth(ctx context.Context) (string, error) {
	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}

	// Store state for validation (in production, use secure session store)
	m.stateStore[state] = true

	// Generate authorization URL with PKCE
	authURL := m.oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline, // Request refresh token
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	m.logger.Info("OAuth flow initiated",
		"auth_url", authURL,
		"state", state[:8]+"...") // Log only prefix of state

	return authURL, nil
}

// HandleCallback processes OAuth callback and exchanges code for tokens
func (m *Manager) HandleCallback(ctx context.Context, code string, state string) error {
	// Validate state
	if !m.stateStore[state] {
		return fmt.Errorf("invalid state parameter (possible CSRF attack)")
	}

	// Remove used state
	delete(m.stateStore, state)

	m.logger.Debug("exchanging authorization code for tokens",
		"code", code[:10]+"...")

	// Exchange code for tokens
	token, err := m.oauthConfig.Exchange(ctx, code)
	if err != nil {
		m.logger.Debug("token exchange failed", "error", err)
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}

	m.logger.Debug("received tokens from OAuth provider",
		"access_token", token.AccessToken,
		"refresh_token", token.RefreshToken,
		"token_type", token.TokenType,
		"expiry", token.Expiry)

	// Convert to TokenData
	tokenData := &TokenData{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}

	// Extract ID token if present
	if idToken, ok := token.Extra("id_token").(string); ok {
		tokenData.IDToken = idToken
		m.logger.Debug("extracted ID token from OAuth response",
			"id_token", idToken)
	} else {
		m.logger.Debug("no ID token in OAuth response")
	}

	// Store tokens
	if err := m.tokenManager.StoreTokens(ctx, tokenData); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	m.logger.Info("OAuth callback processed successfully",
		"token_type", tokenData.TokenType,
		"expires_at", tokenData.Expiry)

	return nil
}

// ClearTokens removes stored tokens (logout)
func (m *Manager) ClearTokens() error {
	return m.tokenManager.ClearTokens()
}

// InitiateDeviceFlow starts a device authorization flow
func (m *Manager) InitiateDeviceFlow(ctx context.Context) (*DeviceCodeData, error) {
	verificationURI := fmt.Sprintf("http://%s/device", m.config.ServerAddr)
	if m.config.ServerAddr[0] == ':' {
		verificationURI = fmt.Sprintf("http://localhost%s/device", m.config.ServerAddr)
	}

	return m.deviceStore.CreateDeviceCode(verificationURI)
}

// PollForToken polls for authorization and returns tokens when available
func (m *Manager) PollForToken(ctx context.Context, deviceCode string) (*TokenData, error) {
	data, err := m.deviceStore.GetByDeviceCode(deviceCode)
	if err != nil {
		return nil, err
	}

	if !data.Authorized {
		return nil, fmt.Errorf("authorization_pending")
	}

	// Authorization complete, clean up and return tokens
	tokens := data.Tokens
	m.deviceStore.Delete(deviceCode)

	return tokens, nil
}

// GetDeviceByUserCode retrieves device data for user authorization
func (m *Manager) GetDeviceByUserCode(userCode string) (*DeviceCodeData, error) {
	return m.deviceStore.GetByUserCode(userCode)
}

// AuthorizeDeviceWithUserCode completes device authorization
func (m *Manager) AuthorizeDeviceWithUserCode(userCode string, tokens *TokenData) error {
	return m.deviceStore.AuthorizeDevice(userCode, tokens)
}

// formatError provides user-friendly error messages with guidance
func (m *Manager) formatError(err error) error {
	switch err {
	case ErrTokenNotFound:
		return fmt.Errorf(
			"not authenticated: no valid session found\n"+
				"Please authenticate by visiting: %s/oauth/login\n"+
				"Or ensure environment variables are set: OAUTH_ISSUER_URL, OAUTH_CLIENT_ID (and optionally OAUTH_CLIENT_SECRET)",
			m.config.ServerAddr,
		)

	case ErrTokenExpired:
		return fmt.Errorf(
			"authentication expired: your session has timed out\n"+
				"Please re-authenticate by visiting: %s/oauth/login",
			m.config.ServerAddr,
		)

	case ErrRefreshFailed:
		return fmt.Errorf(
			"token refresh failed: %w\n"+
				"This may indicate revoked access or network issues.\n"+
				"Try re-authenticating at: %s/oauth/login",
			err, m.config.ServerAddr,
		)

	default:
		return fmt.Errorf("authentication error: %w", err)
	}
}

// validateConfig checks if the configuration is valid
func validateConfig(config Config) error {
	if config.IssuerURL == "" {
		return ErrNotConfigured
	}

	if config.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}

	if len(config.Scopes) == 0 {
		return fmt.Errorf("at least one scope is required")
	}

	// Validate issuer URL format
	if _, err := url.Parse(config.IssuerURL); err != nil {
		return fmt.Errorf("invalid issuer URL: %w", err)
	}

	return nil
}

// generateRedirectURI creates a redirect URI from server address
func generateRedirectURI(serverAddr string) string {
	// Parse server address
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	// Handle different address formats
	host := serverAddr
	if host[0] == ':' {
		host = "localhost" + host
	}

	return fmt.Sprintf("http://%s/callback", host)
}

// generateRandomState generates a cryptographically secure random state
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
