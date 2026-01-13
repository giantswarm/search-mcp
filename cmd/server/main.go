package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/giantswarm/search-mcp/internal/search"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Define flags
	transport := flag.String("transport", "stdio", "Transport type: stdio or streamable-http")
	httpAddr := flag.String("http-addr", ":8080", "HTTP server address (for streamable-http transport)")
	httpEndpoint := flag.String("http-endpoint", "/mcp", "HTTP endpoint path (for streamable-http transport)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Show version information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Giant Swarm Search MCP Server\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nTransport Types:\n")
		fmt.Fprintf(os.Stderr, "  stdio            Standard I/O transport (default, for Cursor and local use)\n")
		fmt.Fprintf(os.Stderr, "  streamable-http  HTTP transport (for network deployment)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Start with stdio transport (default)\n")
		fmt.Fprintf(os.Stderr, "  %s\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Start with HTTP transport\n")
		fmt.Fprintf(os.Stderr, "  %s --transport=streamable-http --http-addr=:8080\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Enable debug logging\n")
		fmt.Fprintf(os.Stderr, "  %s --debug\n\n", os.Args[0])
	}

	flag.Parse()

	// Show version if requested
	if *showVersion {
		fmt.Printf("Giant Swarm Search MCP Server\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit:  %s\n", commit)
		fmt.Printf("Date:    %s\n", date)
		os.Exit(0)
	}

	// Create server config
	config := search.ServerConfig{
		Transport:    *transport,
		HTTPAddr:     *httpAddr,
		HTTPEndpoint: *httpEndpoint,
		Debug:        *debug,
	}

	// Create and start server
	srv, err := search.NewServer(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
