package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// TokenManager handles token lifecycle including refresh
type TokenManager struct {
	storage      TokenStorage
	oauthConfig  *oauth2.Config
	logger       *slog.Logger
	mu           sync.Mutex // Protects token refresh operations
	currentToken *TokenData
}

// NewTokenManager creates a new token manager
func NewTokenManager(storage TokenStorage, oauthConfig *oauth2.Config, logger *slog.Logger) *TokenManager {
	return &TokenManager{
		storage:     storage,
		oauthConfig: oauthConfig,
		logger:      logger,
	}
}

// GetToken returns a valid access token, refreshing if necessary
func (m *TokenManager) GetToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load current tokens if not in memory
	if m.currentToken == nil {
		tokens, err := m.storage.Load(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to load tokens: %w", err)
		}
		m.currentToken = tokens
	}

	// Check if token needs refresh
	if m.currentToken.NeedsRefresh() {
		m.logger.Debug("token nearing expiration, attempting refresh",
			"expires_at", m.currentToken.Expiry,
			"time_until_expiry", time.Until(m.currentToken.Expiry))

		newTokens, err := m.refreshToken(ctx)
		if err != nil {
			return "", fmt.Errorf("token refresh failed: %w", err)
		}

		// Store refreshed tokens
		if err := m.storage.Store(ctx, newTokens); err != nil {
			m.logger.Error("failed to store refreshed tokens", "error", err)
			// Continue with new tokens even if storage fails
		}

		m.currentToken = newTokens
		m.logger.Info("token refreshed successfully",
			"new_expiry", m.currentToken.Expiry)
	}

	// Check if token is valid
	if !m.currentToken.IsValid() {
		return "", ErrTokenExpired
	}

	return m.currentToken.AccessToken, nil
}

// StoreTokens stores new tokens (called after successful auth)
func (m *TokenManager) StoreTokens(ctx context.Context, tokens *TokenData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.storage.Store(ctx, tokens); err != nil {
		return fmt.Errorf("failed to store tokens: %w", err)
	}

	m.currentToken = tokens
	m.logger.Info("tokens stored successfully", "expiry", tokens.Expiry)

	return nil
}

// HasValidToken checks if valid tokens are available
func (m *TokenManager) HasValidToken(ctx context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to load tokens if not in memory
	if m.currentToken == nil {
		tokens, err := m.storage.Load(ctx)
		if err != nil {
			return false
		}
		m.currentToken = tokens
	}

	return m.currentToken != nil && m.currentToken.IsValid()
}

// ClearTokens removes stored tokens
func (m *TokenManager) ClearTokens() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.storage.Delete(); err != nil {
		return fmt.Errorf("failed to delete tokens: %w", err)
	}

	m.currentToken = nil
	m.logger.Info("tokens cleared successfully")

	return nil
}

// refreshToken uses the refresh token to obtain new access tokens
func (m *TokenManager) refreshToken(ctx context.Context) (*TokenData, error) {
	if m.currentToken == nil || m.currentToken.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// Create token source from refresh token
	token := &oauth2.Token{
		AccessToken:  m.currentToken.AccessToken,
		RefreshToken: m.currentToken.RefreshToken,
		TokenType:    m.currentToken.TokenType,
		Expiry:       m.currentToken.Expiry,
	}

	tokenSource := m.oauthConfig.TokenSource(ctx, token)

	// Get new token (this automatically uses refresh token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRefreshFailed, err)
	}

	// Convert oauth2.Token to TokenData
	tokenData := &TokenData{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		TokenType:    newToken.TokenType,
		Expiry:       newToken.Expiry,
	}

	// Extract ID token if present
	if idToken, ok := newToken.Extra("id_token").(string); ok {
		tokenData.IDToken = idToken
	}

	return tokenData, nil
}
