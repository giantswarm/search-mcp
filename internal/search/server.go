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

	"github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/pagekit"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/giantswarm/search-mcp/internal/metrics"
)

// Transport names supported by the server.
const (
	transportStdio          = "stdio"
	transportStreamableHTTP = "streamable-http"
)

// httpReadHeaderTimeout bounds how long the server waits for request headers,
// protecting against Slowloris-style attacks.
const httpReadHeaderTimeout = 10 * time.Second

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
	forwarded *auth.ForwardedTokenVerifier
	logger    *slog.Logger
	config    ServerConfig
	metrics   metrics.Collector
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
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"giantswarm-search",
		"0.1.0",
		server.WithLogging(),
	)

	// Create search client
	searchClient := NewClient(logger)

	// Initialize auth manager (for both HTTP and stdio mode)
	var authMgr auth.AuthManager
	var serverAddr string
	if config.Transport == transportStreamableHTTP {
		serverAddr = config.HTTPAddr
	} else {
		serverAddr = ":8080" // Default for device flow verification URL
	}

	authConfig := auth.Config{
		IssuerURL:    os.Getenv("OAUTH_ISSUER_URL"),
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
		Scopes:       []string{"openid", "offline_access"},
		ServerAddr:   serverAddr,
	}

	forwardedConfig := auth.ForwardedConfig{
		IssuerURL: os.Getenv("FORWARDED_TOKEN_ISSUER_URL"),
		Audience:  os.Getenv("FORWARDED_TOKEN_AUDIENCE"),
	}

	// Over HTTP behind an MCP gateway, each request carries its caller's
	// token; the process-wide login is not used.
	if forwardedConfig.Enabled() && config.Transport != transportStreamableHTTP {
		logger.Warn("forwarded tokens apply to streamable-http only; ignored", "transport", config.Transport)
	}
	var forwarded *auth.ForwardedTokenVerifier
	switch {
	case forwardedConfig.Enabled() && config.Transport == transportStreamableHTTP:
		var err error
		forwarded, err = auth.NewForwardedTokenVerifier(context.Background(), forwardedConfig)
		if err != nil {
			return nil, fmt.Errorf("forwarded tokens: %w", err)
		}
		logger.Info("forwarded tokens enabled", "issuer", forwardedConfig.IssuerURL, "audience", forwardedConfig.Audience)
	case authConfig.IssuerURL != "" && authConfig.ClientID != "":
		var err error
		authMgr, err = auth.NewManager(authConfig, logger)
		if err != nil {
			logger.Warn("auth manager initialization failed", "error", err,
				"note", "Intranet tools will not be available")
		} else {
			logger.Info("authentication enabled", "issuer", authConfig.IssuerURL, "transport", config.Transport)
		}
	default:
		logger.Info("authentication not configured",
			"note", "Set OAUTH_ISSUER_URL and OAUTH_CLIENT_ID (OAUTH_CLIENT_SECRET is optional) to enable intranet access")
	}

	// Initialize metrics collector based on transport
	var metricsCollector metrics.Collector
	if config.Transport == transportStreamableHTTP {
		metricsCollector = metrics.NewPrometheusCollector()
		logger.Info("metrics enabled", "endpoint", "/metrics")
	} else {
		metricsCollector = metrics.NewNoopCollector()
	}

	// Register tools
	RegisterTools(mcpServer, searchClient, authMgr, forwarded, config.Transport, logger, metricsCollector)

	logger.Info("MCP server initialized", "name", "giantswarm-search", "tools", 5)

	return &Server{
		mcpServer: mcpServer,
		client:    searchClient,
		authMgr:   authMgr,
		forwarded: forwarded,
		logger:    logger,
		config:    config,
		metrics:   metricsCollector,
	}, nil
}

// Start starts the MCP server with the configured transport
func (s *Server) Start(ctx context.Context) error {
	// If auth is enabled in stdio mode, start background HTTP server for device flow
	if s.config.Transport == transportStdio && s.authMgr != nil {
		go s.startBackgroundHTTPForDeviceFlow()
	}

	switch s.config.Transport {
	case transportStdio:
		return s.startStdio(ctx)
	case transportStreamableHTTP:
		return s.startHTTP(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s (supported: stdio, streamable-http)", s.config.Transport)
	}
}

// startBackgroundHTTPForDeviceFlow starts a minimal HTTP server for device flow in stdio mode
func (s *Server) startBackgroundHTTPForDeviceFlow() {
	mux := http.NewServeMux()
	s.registerOAuthRoutes(mux)

	// Use :8080 as default for device flow in stdio mode
	addr := ":8080"
	if s.config.HTTPAddr != "" {
		addr = s.config.HTTPAddr
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}

	s.logger.Info("starting background HTTP server for device flow", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("background HTTP server error", "error", err)
	}
}

// startStdio starts the server in stdio mode
func (s *Server) startStdio(ctx context.Context) error {
	s.logger.Info("starting MCP server", "transport", transportStdio)

	// ServeStdio handles signal handling internally
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("stdio server error: %w", err)
	}

	return nil
}

// startHTTP starts the server in HTTP mode
func (s *Server) startHTTP(ctx context.Context) error {
	s.logger.Info("starting MCP server", "transport", transportStreamableHTTP, "addr", s.config.HTTPAddr, "endpoint", s.config.HTTPEndpoint)

	// Create custom mux that combines MCP endpoints and OAuth routes
	mux := http.NewServeMux()

	// Register MCP endpoint with connection logging and metrics
	mux.Handle(s.config.HTTPEndpoint, s.mcpHandler())

	// Register metrics endpoint
	if handler := s.metrics.Handler(); handler != nil {
		mux.Handle("/metrics", handler)
	}

	// Register health endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`OK`))
	})

	// Add OAuth routes if auth is enabled
	if s.authMgr != nil {
		s.registerOAuthRoutes(mux)
	}

	// Create custom HTTP server with our mux
	httpServer := &http.Server{
		Addr:              s.config.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
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

// streamableHTTPServer serves MCP over streamable HTTP; with forwarded tokens,
// each request's bearer token reaches its tool calls.
func (s *Server) streamableHTTPServer() *server.StreamableHTTPServer {
	options := []server.StreamableHTTPOption{server.WithEndpointPath(s.config.HTTPEndpoint)}
	if s.forwarded != nil {
		options = append(options, server.WithHTTPContextFunc(withForwardedToken))
	}
	return server.NewStreamableHTTPServer(s.mcpServer, options...)
}

// maxMCPBodyBytes bounds an MCP request body. mcp-go reads a POST body whole
// before validating it (CVE-2026-108859); the bound keeps that read small.
const maxMCPBodyBytes = 1 << 20

// mcpHandler serves the MCP endpoint with connection logging and metrics,
// every request body bounded at maxMCPBodyBytes.
func (s *Server) mcpHandler() http.Handler {
	mcpHTTPServer := s.streamableHTTPServer()
	return http.MaxBytesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("new HTTP streamable connection", "method", r.Method, "remote", r.RemoteAddr)
		s.metrics.RecordSSEConnection()
		defer s.metrics.RecordSSEDisconnection()
		mcpHTTPServer.ServeHTTP(w, r)
	}), maxMCPBodyBytes)
}

// deviceUserCodeCookie carries a device flow's user code from /device to the
// callback.
const deviceUserCodeCookie = "device_user_code"

// registerOAuthRoutes registers OAuth authentication routes
func (s *Server) registerOAuthRoutes(mux *http.ServeMux) {
	// OAuth initiation endpoint
	mux.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
		authURL, err := s.authMgr.InitiateAuth(r.Context())
		if err != nil {
			s.logger.Error("auth initiation failed", "error", err)
			renderSignInError(w, http.StatusInternalServerError, "The sign-in could not be started", messageTryLater)
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
			renderSignInError(w, http.StatusBadRequest, "The sign-in response is incomplete", messageStartAgain)
			return
		}

		err := s.authMgr.HandleCallback(r.Context(), code, state)
		if err != nil {
			s.logger.Error("auth callback failed", "error", err)
			renderSignInError(w, http.StatusInternalServerError, "The sign-in could not be completed", messageStartAgain)
			return
		}

		// Check if this is a device flow authorization
		cookie, err := r.Cookie(deviceUserCodeCookie)
		if err == nil && cookie.Value != "" {
			// Device flow - need to link tokens to the device
			userCode := cookie.Value

			// Clear the cookie (attributes must match the ones used when setting it)
			http.SetCookie(w, &http.Cookie{
				Name:     deviceUserCodeCookie,
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})

			// The tokens were just stored in HandleCallback above
			// Since both device flow and HTTP mode share the same token storage,
			// the device can now use IsAuthenticated() to check for tokens
			// Note: We don't need to call AuthorizeDevice on the device store
			// because we're using a "retry" pattern instead of polling

			s.logger.Info("device flow authorization completed", "user_code", userCode)

			pagekit.Render(w, http.StatusOK, deviceSignedInPage(userCode))
			return
		}

		// Standard OAuth flow (not device flow)
		s.logger.Info("authentication successful")

		pagekit.Render(w, http.StatusOK, signedInPage)
	})

	// Device flow authorization page
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.serveDeviceAuthPage(w, r)
		case http.MethodPost:
			s.handleDeviceAuthorize(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.logger.Info("OAuth endpoints registered",
		"login", "/oauth/login",
		"callback", "/callback",
		"device", "/device")
}

// serveDeviceAuthPage serves the device authorization page
func (s *Server) serveDeviceAuthPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Device Authorization</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            margin-top: 0;
        }
        .code-input {
            width: 100%%;
            padding: 12px;
            font-size: 18px;
            text-transform: uppercase;
            letter-spacing: 2px;
            border: 2px solid #ddd;
            border-radius: 4px;
            text-align: center;
            font-family: monospace;
            margin: 20px 0;
        }
        .btn {
            width: 100%%;
            padding: 12px;
            background: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
        }
        .btn:hover {
            background: #0056b3;
        }
        .info {
            background: #e3f2fd;
            padding: 15px;
            border-radius: 4px;
            margin: 20px 0;
            color: #1976d2;
        }
        .error {
            background: #ffebee;
            padding: 15px;
            border-radius: 4px;
            margin: 20px 0;
            color: #c62828;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Device Authorization</h1>
        <div class="info">
            Enter the code displayed on your device to authorize access to Giant Swarm intranet.
        </div>
        <form method="POST" action="/device">
            <input
                type="text"
                name="user_code"
                class="code-input"
                placeholder="XXXX-XXXX"
                pattern="[A-Z0-9]{4}-[A-Z0-9]{4}"
                required
                maxlength="9"
                autocomplete="off"
            />
            <button type="submit" class="btn">Authorize Device</button>
        </form>
    </div>
</body>
</html>
	`)
}

// handleDeviceAuthorize handles device authorization
func (s *Server) handleDeviceAuthorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderSignInError(w, http.StatusBadRequest, "The code could not be read", messageEnterCode)
		return
	}

	userCode := r.FormValue("user_code")
	if userCode == "" {
		renderSignInError(w, http.StatusBadRequest, "No code was entered", messageEnterCode)
		return
	}

	// Get device data by user code
	mgr, ok := s.authMgr.(*auth.Manager)
	if !ok {
		s.logger.Error("auth manager is not a Manager type")
		renderSignInError(w, http.StatusInternalServerError, "The sign-in could not be started", messageTryLater)
		return
	}

	deviceData, err := mgr.GetDeviceByUserCode(userCode)
	if err != nil {
		s.logger.Warn("invalid user code", "code", userCode, "error", err)
		renderSignInError(w, http.StatusBadRequest, "The code is invalid or has expired", messageEnterCode)
		return
	}

	// Initiate OAuth flow for this device
	authURL, err := s.authMgr.InitiateAuth(r.Context())
	if err != nil {
		s.logger.Error("failed to initiate auth", "error", err)
		renderSignInError(w, http.StatusInternalServerError, "The sign-in could not be started", messageTryLater)
		return
	}

	// Store user code in session/cookie for callback
	http.SetCookie(w, &http.Cookie{
		Name:     deviceUserCodeCookie,
		Value:    userCode,
		Path:     "/",
		MaxAge:   600,  // 10 minutes
		Secure:   true, // browsers treat localhost as a secure context, so this also works for the local device flow
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	s.logger.Info("device authorization initiated", "user_code", userCode, "device_code", deviceData.DeviceCode)

	// Redirect to OAuth provider
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}
