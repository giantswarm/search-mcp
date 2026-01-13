package search

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/mark3labs/mcp-go/server"
)

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	Transport    string
	HTTPAddr     string
	HTTPEndpoint string
	Debug        bool
}

// Server wraps the MCP server with our search functionality
type Server struct {
	mcpServer *server.MCPServer
	client    *Client
	authMgr   auth.AuthManager
	logger    *slog.Logger
	config    ServerConfig
}

// NewServer creates a new search MCP server
func NewServer(config ServerConfig) (*Server, error) {
	// Setup logger
	logLevel := slog.LevelInfo
	if config.Debug {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"giantswarm-search",
		"0.1.0",
		server.WithLogging(),
	)

	// Create search client
	searchClient := NewClient(logger)

	// Initialize auth manager (only for HTTP mode)
	var authMgr auth.AuthManager
	if config.Transport == "streamable-http" {
		authConfig := auth.Config{
			IssuerURL:    os.Getenv("OAUTH_ISSUER_URL"),
			ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
			Scopes:       []string{"openid", "offline_access"},
			ServerAddr:   config.HTTPAddr,
		}

		// Only create auth manager if configuration is provided
		if authConfig.IssuerURL != "" && authConfig.ClientID != "" && authConfig.ClientSecret != "" {
			var err error
			authMgr, err = auth.NewManager(authConfig, logger)
			if err != nil {
				logger.Warn("auth manager initialization failed", "error", err,
					"note", "Intranet tools will not be available")
			} else {
				logger.Info("authentication enabled", "issuer", authConfig.IssuerURL)
			}
		} else {
			logger.Info("authentication not configured",
				"note", "Set OAUTH_ISSUER_URL, OAUTH_CLIENT_ID, and OAUTH_CLIENT_SECRET to enable intranet access")
		}
	}

	// Register tools
	RegisterTools(mcpServer, searchClient, authMgr, config.Transport)

	logger.Info("MCP server initialized", "name", "giantswarm-search", "tools", 5)

	return &Server{
		mcpServer: mcpServer,
		client:    searchClient,
		authMgr:   authMgr,
		logger:    logger,
		config:    config,
	}, nil
}

// Start starts the MCP server with the configured transport
func (s *Server) Start(ctx context.Context) error {
	switch s.config.Transport {
	case "stdio":
		return s.startStdio(ctx)
	case "streamable-http":
		return s.startHTTP(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s (supported: stdio, streamable-http)", s.config.Transport)
	}
}

// startStdio starts the server in stdio mode
func (s *Server) startStdio(ctx context.Context) error {
	s.logger.Info("starting MCP server", "transport", "stdio")

	// ServeStdio handles signal handling internally
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("stdio server error: %w", err)
	}

	return nil
}

// startHTTP starts the server in HTTP mode
func (s *Server) startHTTP(ctx context.Context) error {
	s.logger.Info("starting MCP server", "transport", "streamable-http", "addr", s.config.HTTPAddr, "endpoint", s.config.HTTPEndpoint)

	// Create StreamableHTTPServer
	mcpHTTPServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath(s.config.HTTPEndpoint),
	)

	// Create custom mux that combines MCP endpoints and OAuth routes
	mux := http.NewServeMux()

	// Register MCP endpoint
	mux.Handle(s.config.HTTPEndpoint, mcpHTTPServer)

	// Add OAuth routes if auth is enabled
	if s.authMgr != nil {
		s.registerOAuthRoutes(mux)
	}

	// Create custom HTTP server with our mux
	httpServer := &http.Server{
		Addr:    s.config.HTTPAddr,
		Handler: mux,
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("HTTP server listening", "addr", s.config.HTTPAddr, "endpoint", s.config.HTTPEndpoint)
		if s.authMgr != nil {
			s.logger.Info("OAuth endpoints available", "login", "/oauth/login", "callback", "/callback")
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for signal, context cancellation, or error
	select {
	case <-ctx.Done():
		s.logger.Info("context cancelled, shutting down")
	case sig := <-sigChan:
		s.logger.Info("received signal, shutting down", "signal", sig)
	case err := <-errChan:
		return fmt.Errorf("HTTP server error: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server stopped gracefully")
	return nil
}

// registerOAuthRoutes registers OAuth authentication routes
func (s *Server) registerOAuthRoutes(mux *http.ServeMux) {
	// OAuth initiation endpoint
	mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		authURL, err := s.authMgr.InitiateAuth(r.Context())
		if err != nil {
			s.logger.Error("auth initiation failed", "error", err)
			http.Error(w, fmt.Sprintf("Authentication initiation failed: %v", err),
				http.StatusInternalServerError)
			return
		}

		s.logger.Info("redirecting to OAuth provider", "url", authURL)
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	// OAuth callback endpoint
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" {
			s.logger.Error("OAuth callback missing code")
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}

		err := s.authMgr.HandleCallback(r.Context(), code, state)
		if err != nil {
			s.logger.Error("auth callback failed", "error", err)
			http.Error(w, fmt.Sprintf("Authentication callback failed: %v", err),
				http.StatusInternalServerError)
			return
		}

		s.logger.Info("authentication successful")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Authentication Successful</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        h1 { color: #2ecc71; }
        p { line-height: 1.6; }
    </style>
</head>
<body>
    <h1>✓ Authentication Successful</h1>
    <p>You have successfully authenticated with Giant Swarm.</p>
    <p>You can now close this window and return to your AI assistant to access intranet resources.</p>
</body>
</html>
		`)
	})

	s.logger.Info("OAuth endpoints registered",
		"login", "/oauth/login",
		"callback", "/callback")
}
