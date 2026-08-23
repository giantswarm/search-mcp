package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/giantswarm/search-mcp/internal/metrics"
)

const (
	itemsPerPageDefault = 10
	logMaxStringLength  = 100
	logMaxArrayElements = 5
)

// Tool parameter names shared across input schemas and handlers.
const (
	paramTerm            = "term"
	paramStartIndex      = "start_index"
	paramSize            = "size"
	paramRequireAllTerms = "require_all_terms"
	paramURL             = "url"
)

// JSON schema keys and type names used in tool input schemas.
const (
	schemaKeyType        = "type"
	schemaKeyDescription = "description"
	schemaKeyDefault     = "default"
	schemaTypeObject     = "object"
	schemaTypeString     = "string"
	schemaTypeInteger    = "integer"
)

// Parameter descriptions shared by the search tools.
const (
	descSearchTerm      = "The search term (required). Results are ranked by similarity to the search term. Use 'require_all_terms' to restrict results to only pages containing every word."
	descStartIndex      = "The start index of the search results (optional, defaults to 0)"
	descRequireAllTerms = "If true, only return documents that contain ALL search terms (strict AND matching). Default is false, which finds documents similar to the query and ranks by relevance — best for longer, multi-term queries."
)

// contextKey is a private type for context values set by this package,
// avoiding collisions with keys from other packages.
type contextKey string

// authTokenContextKey carries the OAuth access token for authenticated requests.
const authTokenContextKey contextKey = "auth_token"

// searchInputSchema builds the input schema shared by the search tools,
// optionally extended with tool-specific properties.
func searchInputSchema(extraProperties map[string]interface{}) mcp.ToolInputSchema {
	properties := map[string]interface{}{
		paramTerm: map[string]interface{}{
			schemaKeyType:        schemaTypeString,
			schemaKeyDescription: descSearchTerm,
		},
		paramStartIndex: map[string]interface{}{
			schemaKeyType:        schemaTypeInteger,
			schemaKeyDescription: descStartIndex,
			schemaKeyDefault:     0,
		},
		paramSize: map[string]interface{}{
			schemaKeyType:        schemaTypeInteger,
			schemaKeyDescription: fmt.Sprintf("The size of the search results (optional, defaults to %d)", itemsPerPageDefault),
			schemaKeyDefault:     itemsPerPageDefault,
		},
		paramRequireAllTerms: map[string]interface{}{
			schemaKeyType:        "boolean",
			schemaKeyDescription: descRequireAllTerms,
			schemaKeyDefault:     false,
		},
	}
	for name, property := range extraProperties {
		properties[name] = property
	}

	return mcp.ToolInputSchema{
		Type:       schemaTypeObject,
		Properties: properties,
		Required:   []string{paramTerm},
	}
}

// urlInputSchema builds the input schema for tools taking a single URL parameter.
func urlInputSchema(description string) mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type: schemaTypeObject,
		Properties: map[string]interface{}{
			paramURL: map[string]interface{}{
				schemaKeyType:        schemaTypeString,
				schemaKeyDescription: description,
			},
		},
		Required: []string{paramURL},
	}
}

// sanitizeForLog truncates long strings and limits array sizes to prevent log bloat
func sanitizeForLog(args map[string]any) map[string]any {
	result := make(map[string]any, len(args))
	for k, v := range args {
		result[k] = sanitizeValue(v)
	}
	return result
}

func sanitizeValue(v any) any {
	switch val := v.(type) {
	case string:
		if len(val) > logMaxStringLength {
			return val[:logMaxStringLength] + "...(truncated)"
		}
		return val
	case []any:
		if len(val) > logMaxArrayElements {
			truncated := make([]any, logMaxArrayElements)
			for i := 0; i < logMaxArrayElements; i++ {
				truncated[i] = sanitizeValue(val[i])
			}
			return append(truncated, fmt.Sprintf("...(%d more)", len(val)-logMaxArrayElements))
		}
		sanitized := make([]any, len(val))
		for i, item := range val {
			sanitized[i] = sanitizeValue(item)
		}
		return sanitized
	default:
		return v
	}
}

// RegisterTools registers all search tools with the MCP server
func RegisterTools(s *server.MCPServer, client *Client, authMgr auth.AuthManager, transport string, logger *slog.Logger, metricsCollector metrics.Collector) {
	// withLogging wraps a handler to log tool invocations and record metrics
	withLogging := func(name string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			logger.Info("tool invoked", "tool", name, "params", sanitizeForLog(request.GetArguments()))
			metricsCollector.RecordToolCall(name)
			return handler(ctx, request)
		}
	}
	// Register search tool
	s.AddTool(mcp.Tool{
		Name:        "search",
		Description: "Search public and internal Giant Swarm documentation. To paginate through results, use a non-zero start_index.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: searchInputSchema(map[string]interface{}{
			"type_filter": map[string]interface{}{
				schemaKeyType:        schemaTypeString,
				schemaKeyDescription: "To restrict the results to only one source type, e.g. \"Intranet\" (optional, defaults to \"\")",
				schemaKeyDefault:     "",
			},
			"breadcrumb_filter": map[string]interface{}{
				schemaKeyType:        "array",
				schemaKeyDescription: "To restrict the results to a specific section, e.g. [\"docs\", \"support-and-ops\"] (optional, defaults to [])",
				"items": map[string]interface{}{
					schemaKeyType: schemaTypeString,
				},
				schemaKeyDefault: []interface{}{},
			},
		}),
	}, withLogging("search", searchHandler(client)))

	// Register search_docs tool
	s.AddTool(mcp.Tool{
		Name:        "search_docs",
		Description: "Search public documentation. To paginate through results, use a non-zero start_index.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: searchInputSchema(nil),
	}, withLogging("search_docs", searchDocsHandler(client)))

	// Register search_runbook tool
	s.AddTool(mcp.Tool{
		Name:        "search_runbook",
		Description: "Search for DevOps runbooks in the Giant Swarm intranet. Requires authentication.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: searchInputSchema(nil),
	}, withLogging("search_runbook", requireAuth(searchRunbookHandler(client), authMgr, transport)))

	// Register search_ops_recipe tool
	s.AddTool(mcp.Tool{
		Name:        "search_ops_recipe",
		Description: "Search for Ops Recipes (legacy runbooks) in the Giant Swarm intranet. Requires authentication.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: searchInputSchema(nil),
	}, withLogging("search_ops_recipe", requireAuth(searchOpsRecipeHandler(client), authMgr, transport)))

	// Register read_docs_url tool
	s.AddTool(mcp.Tool{
		Name:        "read_docs_url",
		Description: "Returns content from a single URL of the Giant Swarm documentation website docs.giantswarm.io (public, no authentication required), in Markdown format.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: urlInputSchema("The URL to fetch content from, e.g. https://docs.giantswarm.io/some-page/"),
	}, withLogging("read_docs_url", readDocsURLHandler(client)))

	// Register read_docs_index tool
	s.AddTool(mcp.Tool{
		Name:        "read_docs_index",
		Description: "Returns an index of all documentation pages with title, description, and link, in Markdown format.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: mcp.ToolInputSchema{
			Type:       schemaTypeObject,
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	}, withLogging("read_docs_index", readDocsIndexHandler(client)))

	// Register read_handbook_url tool
	s.AddTool(mcp.Tool{
		Name:        "read_handbook_url",
		Description: "Returns content from a single URL on the Giant Swarm handbook (public, no authentication required), in Markdown format.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: urlInputSchema("The URL to fetch content from (e.g., https://handbook.giantswarm.io/docs/some-page/)"),
	}, withLogging("read_handbook_url", readHandbookURLHandler(client)))

	// Register read_intranet_url tool
	s.AddTool(mcp.Tool{
		Name:        "read_intranet_url",
		Description: "Returns content from a single URL on the Giant Swarm intranet, in Markdown format. Requires authentication.",
		Annotations: mcp.ToolAnnotation{
			ReadOnlyHint:  mcp.ToBoolPtr(true),
			OpenWorldHint: mcp.ToBoolPtr(false),
		},
		InputSchema: urlInputSchema("The URL to fetch content from (e.g., https://intranet.giantswarm.io/docs/some-page/)"),
	}, withLogging("read_intranet_url", requireAuth(readIntranetURLHandler(client), authMgr, transport)))
}

// requireAuth wraps a handler to require authentication
func requireAuth(handler server.ToolHandlerFunc, authMgr auth.AuthManager, transport string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Stdio mode: Use device flow
		if transport == transportStdio {
			// Check if auth manager is configured
			if authMgr == nil {
				return mcp.NewToolResultError(
					"❌ Authentication not configured\n\n" +
						"This tool requires authentication to access Giant Swarm intranet.\n\n" +
						"Please configure the following environment variables:\n" +
						"  OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io\n" +
						"  OAUTH_CLIENT_ID=searchmcp\n" +
						"  OAUTH_CLIENT_SECRET=<your-secret>  (optional)\\n\\n" +
						"Then restart the server.",
				), nil
			}

			// Check if already authenticated
			if !authMgr.IsAuthenticated() {
				// Need to initiate device flow
				mgr, ok := authMgr.(*auth.Manager)
				if !ok {
					return mcp.NewToolResultError("❌ Internal error: invalid auth manager type"), nil
				}

				// Initiate device flow
				deviceData, err := mgr.InitiateDeviceFlow(ctx)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("❌ Failed to initiate authentication: %v", err)), nil
				}

				// Return instructions for user
				return mcp.NewToolResultError(
					fmt.Sprintf("❌ Authentication required\n\n"+
						"To access intranet resources, please authorize this device:\n\n"+
						"1. Visit: %s\n"+
						"2. Enter code: %s\n"+
						"3. Sign in with your Giant Swarm credentials\n\n"+
						"After authorizing, please retry this tool.\n\n"+
						"Code expires in 10 minutes.",
						deviceData.VerificationURI,
						deviceData.UserCode),
				), nil
			}

			// Authenticated - proceed with request
		}

		// Check if auth is configured
		if authMgr == nil {
			return mcp.NewToolResultError(
				"❌ Authentication not configured\n\n" +
					"This tool requires authentication to access Giant Swarm intranet.\n\n" +
					"Please configure the following environment variables:\n" +
					"  OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io\n" +
					"  OAUTH_CLIENT_ID=searchmcp\n" +
					"  OAUTH_CLIENT_SECRET=<your-secret>  (optional)\\n\\n" +
					"Then restart the server and authenticate by visiting:\n" +
					"  http://localhost:8080/oauth/login",
			), nil
		}

		// Check if user is authenticated
		if !authMgr.IsAuthenticated() {
			return mcp.NewToolResultError(
				"❌ Authentication required\n\n" +
					"You need to authenticate before accessing intranet resources.\n\n" +
					"Please visit: http://localhost:8080/oauth/login\n\n" +
					"After authenticating, you can use this tool.",
			), nil
		}

		// Get valid token (will refresh if needed)
		token, err := authMgr.GetToken(ctx)
		if err != nil {
			return mcp.NewToolResultError(
				fmt.Sprintf("❌ Authentication failed: %v\n\n"+
					"Your session may have expired. Please re-authenticate at:\n"+
					"http://localhost:8080/oauth/login", err),
			), nil
		}

		// Inject token into context for client to use
		ctx = context.WithValue(ctx, authTokenContextKey, token)

		// Call original handler
		return handler(ctx, request)
	}
}

// searchHandler handles the search tool
func searchHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		term := request.GetString(paramTerm, "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt(paramStartIndex, 0)
		size := request.GetInt(paramSize, itemsPerPageDefault)
		typeFilter := request.GetString("type_filter", "")
		breadcrumbFilter := request.GetStringSlice("breadcrumb_filter", []string{})
		requireAllTerms := request.GetBool(paramRequireAllTerms, false)

		// Perform search
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			TypeFilter:       typeFilter,
			BreadcrumbFilter: breadcrumbFilter,
			RequireAllTerms:  requireAllTerms,
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

// searchDocsHandler handles the search tool
func searchDocsHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		term := request.GetString(paramTerm, "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt(paramStartIndex, 0)
		size := request.GetInt(paramSize, itemsPerPageDefault)
		requireAllTerms := request.GetBool(paramRequireAllTerms, false)

		// Perform search
		searchReq := SearchRequest{
			Term:            term,
			StartIndex:      startIndex,
			Size:            size,
			TypeFilter:      "Documentation",
			RequireAllTerms: requireAllTerms,
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
		term := request.GetString(paramTerm, "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt(paramStartIndex, 0)
		size := request.GetInt(paramSize, itemsPerPageDefault)
		requireAllTerms := request.GetBool(paramRequireAllTerms, false)

		// Perform search with runbook breadcrumb filter
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			BreadcrumbFilter: []string{"support-and-ops", "runbooks"},
			RequireAllTerms:  requireAllTerms,
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
		term := request.GetString(paramTerm, "")
		if term == "" {
			return mcp.NewToolResultError("term parameter is required"), nil
		}

		startIndex := request.GetInt(paramStartIndex, 0)
		size := request.GetInt(paramSize, itemsPerPageDefault)
		requireAllTerms := request.GetBool(paramRequireAllTerms, false)

		// Perform search with ops-recipes breadcrumb filter
		searchReq := SearchRequest{
			Term:             term,
			StartIndex:       startIndex,
			Size:             size,
			BreadcrumbFilter: []string{"support-and-ops", "ops-recipes"},
			RequireAllTerms:  requireAllTerms,
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

// readDocsURLHandler handles the read_docs_url tool
func readDocsURLHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		url := request.GetString(paramURL, "")
		if url == "" {
			return mcp.NewToolResultError("url parameter is required"), nil
		}

		// Validate URL is from handbook
		if !strings.HasPrefix(url, "https://docs.giantswarm.io/") {
			return mcp.NewToolResultError("❌ URL must be from the Giant Swarm handbook (https://docs.giantswarm.io/)."), nil
		}

		// Fetch content
		htmlContent, err := client.FetchURL(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error accessing %s: %v", url, err)), nil
		}

		// Pick only relevant content from selector '#main .container .content'
		htmlContent, err = ExtractSelector(htmlContent, ".base-content, .list-content")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error extracting content: %v", err)), nil
		}

		// Convert to Markdown
		markdown, err := ConvertHTMLToMarkdown(htmlContent, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error converting content: %v", err)), nil
		}

		return mcp.NewToolResultText(markdown), nil
	}
}

// readDocsIndexHandler handles the read_docs_url tool
func readDocsIndexHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Fetch content
		url := "https://docs.giantswarm.io/llms.txt"
		content, err := client.FetchURL(ctx, url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error accessing %s: %v", url, err)), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

// readHandbookURLHandler handles the read_handbook_url tool
func readHandbookURLHandler(client *Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse arguments
		url := request.GetString(paramURL, "")
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
		url := request.GetString(paramURL, "")
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
