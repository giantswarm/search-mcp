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

	// Check if token is expired or needs refresh
	isExpired := !m.currentToken.IsValid()
	needsRefresh := m.currentToken.NeedsRefresh()

	if isExpired || needsRefresh {
		// Log appropriate message based on token state
		if isExpired {
			m.logger.Debug("token expired, attempting transparent refresh",
				"expires_at", m.currentToken.Expiry,
				"expired_since", time.Since(m.currentToken.Expiry))
		} else {
			m.logger.Debug("token nearing expiration, attempting refresh",
				"expires_at", m.currentToken.Expiry,
				"time_until_expiry", time.Until(m.currentToken.Expiry))
		}

		m.logger.Debug("current token details before refresh",
			"access_token", m.currentToken.AccessToken,
			"id_token", m.currentToken.IDToken,
			"has_refresh_token", m.currentToken.RefreshToken != "")

		// Attempt to refresh the token
		newTokens, err := m.refreshToken(ctx)
		if err != nil {
			m.logger.Debug("token refresh failed",
				"error", err,
				"old_expiry", m.currentToken.Expiry,
				"was_expired", isExpired)

			// If token was expired and refresh failed, return token expired error
			if isExpired {
				return "", fmt.Errorf("%w: %v", ErrTokenExpired, err)
			}
			// If token was just nearing expiration, return refresh failed error
			return "", fmt.Errorf("token refresh failed: %w", err)
		}

		m.logger.Debug("token refresh successful, received new tokens",
			"access_token", newTokens.AccessToken,
			"id_token", newTokens.IDToken,
			"new_expiry", newTokens.Expiry,
			"has_refresh_token", newTokens.RefreshToken != "")

		// Store refreshed tokens
		if err := m.storage.Store(ctx, newTokens); err != nil {
			m.logger.Error("failed to store refreshed tokens", "error", err)
			// Continue with new tokens even if storage fails
		} else {
			m.logger.Debug("refreshed tokens stored successfully")
		}

		m.currentToken = newTokens
		m.logger.Info("token refreshed successfully",
			"new_expiry", m.currentToken.Expiry,
			"was_expired", isExpired)
	}

	// Final validation check - token should be valid at this point
	if !m.currentToken.IsValid() {
		m.logger.Error("token still invalid after refresh attempt",
			"expiry", m.currentToken.Expiry,
			"is_expired", time.Now().After(m.currentToken.Expiry))
		return "", ErrTokenExpired
	}

	m.logger.Debug("returning valid access token",
		"access_token", m.currentToken.AccessToken,
		"id_token", m.currentToken.IDToken,
		"expires_at", m.currentToken.Expiry,
		"time_until_expiry", time.Until(m.currentToken.Expiry))

	return m.currentToken.AccessToken, nil
}

// StoreTokens stores new tokens (called after successful auth)
func (m *TokenManager) StoreTokens(ctx context.Context, tokens *TokenData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("storing new tokens",
		"access_token", tokens.AccessToken,
		"id_token", tokens.IDToken,
		"has_refresh_token", tokens.RefreshToken != "",
		"expiry", tokens.Expiry)

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
		m.logger.Debug("refresh token not available",
			"has_current_token", m.currentToken != nil)
		return nil, fmt.Errorf("no refresh token available")
	}

	m.logger.Debug("initiating token refresh",
		"old_access_token", m.currentToken.AccessToken,
		"refresh_token", m.currentToken.RefreshToken,
		"old_expiry", m.currentToken.Expiry)

	// Create token source from refresh token
	token := &oauth2.Token{
		AccessToken:  m.currentToken.AccessToken,
		RefreshToken: m.currentToken.RefreshToken,
		TokenType:    m.currentToken.TokenType,
		Expiry:       m.currentToken.Expiry,
	}

	tokenSource := m.oauthConfig.TokenSource(ctx, token)

	m.logger.Debug("calling token source to exchange refresh token")

	// Get new token (this automatically uses refresh token)
	newToken, err := tokenSource.Token()
	if err != nil {
		m.logger.Debug("token source returned error",
			"error", err,
			"error_type", fmt.Sprintf("%T", err))
		return nil, fmt.Errorf("%w: %v", ErrRefreshFailed, err)
	}

	m.logger.Debug("token source returned new token",
		"new_access_token", newToken.AccessToken,
		"new_refresh_token", newToken.RefreshToken,
		"new_expiry", newToken.Expiry,
		"token_type", newToken.TokenType)

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
		m.logger.Debug("extracted ID token from response",
			"id_token", idToken)
	} else {
		m.logger.Debug("no ID token in refresh response")
	}

	return tokenData, nil
}
