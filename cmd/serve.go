package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/giantswarm/search-mcp/internal/search"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long:  "Start the MCP server with the specified transport (stdio or streamable-http)",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Use the same flags from root command
	serveCmd.Flags().StringVar(&transport, "transport", "stdio", "Transport type: stdio or streamable-http")
	serveCmd.Flags().StringVar(&httpAddr, "http-addr", ":8080", "HTTP server address (for streamable-http transport)")
	serveCmd.Flags().StringVar(&httpEndpoint, "http-endpoint", "/mcp", "HTTP endpoint path (for streamable-http transport)")
	serveCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Create server config
	config := search.ServerConfig{
		Transport:    transport,
		HTTPAddr:     httpAddr,
		HTTPEndpoint: httpEndpoint,
		Debug:        debug,
	}

	// Create and start server
	srv, err := search.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
