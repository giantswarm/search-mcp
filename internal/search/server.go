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

	// Register tools
	RegisterTools(mcpServer, searchClient)

	logger.Info("MCP server initialized", "name", "giantswarm-search", "tools", 5)

	return &Server{
		mcpServer: mcpServer,
		client:    searchClient,
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
	httpServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath(s.config.HTTPEndpoint),
	)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("HTTP server listening", "addr", s.config.HTTPAddr, "endpoint", s.config.HTTPEndpoint)
		if err := httpServer.Start(s.config.HTTPAddr); err != nil && err != http.ErrServerClosed {
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
