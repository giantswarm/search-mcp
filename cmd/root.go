package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/giantswarm/search-mcp/cmd/version"
)

var rootCmd = &cobra.Command{
	Use:   "search-mcp",
	Short: "Giant Swarm Search MCP Server",
	Long:  "MCP (Model Context Protocol) server for searching Giant Swarm documentation",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to serve command when no subcommand is specified
		return runServe(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add version command
	versionCmd, err := version.New(version.Config{})
	if err != nil {
		panic(err)
	}
	rootCmd.AddCommand(versionCmd)

	// Add serve flags to root command so they work without 'serve' subcommand
	rootCmd.Flags().StringVar(&transport, "transport", "stdio", "Transport type: stdio or streamable-http")
	rootCmd.Flags().StringVar(&httpAddr, "http-addr", ":8080", "HTTP server address (for streamable-http transport)")
	rootCmd.Flags().StringVar(&httpEndpoint, "http-endpoint", "/mcp", "HTTP endpoint path (for streamable-http transport)")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
}

var (
	transport    string
	httpAddr     string
	httpEndpoint string
	debug        bool
)
