package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	itemsPerPageDefault = 10
)

// RegisterTools registers all search tools with the MCP server
func RegisterTools(s *server.MCPServer, client *Client) {
	// Register search tool
	s.AddTool(mcp.Tool{
		Name:        "search",
		Description: "Search public and internal Giant Swarm documentation. To paginate through results, use a non-zero start_index.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"term": map[string]interface{}{
					"type":        "string",
					"description": "The search term (required)",
				},
				"start_index": map[string]interface{}{
					"type":        "integer",
					"description": "The start index of the search results (optional, defaults to 0)",
					"default":     0,
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"description": fmt.Sprintf("The size of the search results (optional, defaults to %d)", itemsPerPageDefault),
					"default":     itemsPerPageDefault,
				},
				"type_filter": map[string]interface{}{
					"type":        "string",
					"description": "To restrict the results to only one source type, e.g. \"Intranet\" (optional, defaults to \"\")",
					"default":     "",
				},
				"breadcrumb_filter": map[string]interface{}{
					"type":        "array",
					"description": "To restrict the results to a specific section, e.g. [\"docs\", \"support-and-ops\"] (optional, defaults to [])",
					"items": map[string]interface{}{
						"type": "string",
					},
					"default": []interface{}{},
				},
			},
			Required: []string{"term"},
		},
	}, searchHandler(client))

	// Register search_runbook tool
	s.AddTool(mcp.Tool{
		Name:        "search_runbook",
		Description: "Search for DevOps runbooks in the Giant Swarm intranet.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"term": map[string]interface{}{
					"type":        "string",
					"description": "The search term (required)",
				},
				"start_index": map[string]interface{}{
					"type":        "integer",
					"description": "The start index of the search results (optional, defaults to 0)",
					"default":     0,
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"description": fmt.Sprintf("The size of the search results (optional, defaults to %d)", itemsPerPageDefault),
					"default":     itemsPerPageDefault,
				},
			},
			Required: []string{"term"},
		},
	}, searchRunbookHandler(client))

	// Register search_ops_recipe tool
	s.AddTool(mcp.Tool{
		Name:        "search_ops_recipe",
		Description: "Search for Ops Recipes (legacy runbooks) in the Giant Swarm intranet.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"term": map[string]interface{}{
					"type":        "string",
					"description": "The search term (required)",
				},
				"start_index": map[string]interface{}{
					"type":        "integer",
					"description": "The start index of the search results (optional, defaults to 0)",
					"default":     0,
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"description": fmt.Sprintf("The size of the search results (optional, defaults to %d)", itemsPerPageDefault),
					"default":     itemsPerPageDefault,
				},
			},
			Required: []string{"term"},
		},
	}, searchOpsRecipeHandler(client))

	// Register read_handbook_url tool
	s.AddTool(mcp.Tool{
		Name:        "read_handbook_url",
		Description: "Read content from a single URL on the Giant Swarm handbook (public, no authentication required).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch content from (e.g., https://handbook.giantswarm.io/docs/some-page/)",
				},
			},
			Required: []string{"url"},
		},
	}, readHandbookURLHandler(client))

	// Register read_intranet_url tool
	s.AddTool(mcp.Tool{
		Name:        "read_intranet_url",
		Description: "Read content from a single URL on the Giant Swarm intranet. Note: This may fail without authentication.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch content from (e.g., https://intranet.giantswarm.io/docs/some-page/)",
				},
			},
			Required: []string{"url"},
		},
	}, readIntranetURLHandler(client))
}

// searchHandler handles the search tool
func searchHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		term := request.GetString("term", "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt("start_index", 0)
		size := request.GetInt("size", itemsPerPageDefault)
		typeFilter := request.GetString("type_filter", "")
		breadcrumbFilter := request.GetStringSlice("breadcrumb_filter", []string{})

		// Perform search
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			TypeFilter:       typeFilter,
			BreadcrumbFilter: breadcrumbFilter,
		}

		resp, err := client.Search(ctx, searchReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		// Format results
		result := FormatSearchResults(term, startIndex, resp)

		return mcp.NewToolResultText(result), nil
	}
}

// searchRunbookHandler handles the search_runbook tool
func searchRunbookHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		term := request.GetString("term", "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt("start_index", 0)
		size := request.GetInt("size", itemsPerPageDefault)

		// Perform search with runbook breadcrumb filter
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			BreadcrumbFilter: []string{"support-and-ops", "runbooks"},
		}

		resp, err := client.Search(ctx, searchReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		// Format results
		result := FormatSearchResults(term, startIndex, resp)

		return mcp.NewToolResultText(result), nil
	}
}

// searchOpsRecipeHandler handles the search_ops_recipe tool
func searchOpsRecipeHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		term := request.GetString("term", "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt("start_index", 0)
		size := request.GetInt("size", itemsPerPageDefault)

		// Perform search with ops-recipes breadcrumb filter
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			BreadcrumbFilter: []string{"support-and-ops", "ops-recipes"},
		}

		resp, err := client.Search(ctx, searchReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		// Format results
		result := FormatSearchResults(term, startIndex, resp)

		return mcp.NewToolResultText(result), nil
	}
}

// readHandbookURLHandler handles the read_handbook_url tool
func readHandbookURLHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		url := request.GetString("url", "")
		if url == "" {
			return mcp.NewToolResultError("url parameter is required"), nil
		}

		// Validate URL is from handbook
		if !strings.HasPrefix(url, "https://handbook.giantswarm.io/") {
			return mcp.NewToolResultError("❌ URL must be from the Giant Swarm handbook (https://handbook.giantswarm.io/)."), nil
		}

		// Fetch content
		htmlContent, err := client.FetchURL(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error accessing %s: %v", url, err)), nil
		}

		// Convert to Markdown
		markdown, err := ConvertHTMLToMarkdown(htmlContent, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error converting content: %v", err)), nil
		}

		return mcp.NewToolResultText(markdown), nil
	}
}

// readIntranetURLHandler handles the read_intranet_url tool
func readIntranetURLHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		url := request.GetString("url", "")
		if url == "" {
			return mcp.NewToolResultError("url parameter is required"), nil
		}

		// Validate URL is from intranet
		if !strings.HasPrefix(url, "https://intranet.giantswarm.io/") {
			return mcp.NewToolResultError("❌ URL must be from the Giant Swarm intranet (https://intranet.giantswarm.io/)."), nil
		}

		// Fetch content (may fail without authentication)
		htmlContent, err := client.FetchURL(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error accessing %s: %v (Note: This may require authentication)", url, err)), nil
		}

		// Convert to Markdown
		markdown, err := ConvertHTMLToMarkdown(htmlContent, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error converting content: %v", err)), nil
		}

		return mcp.NewToolResultText(markdown), nil
	}
}
