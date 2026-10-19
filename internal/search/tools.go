package search

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/giantswarm/mcp-toolkit/tooldesc"
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
	descSearchTerm      = "What to search for. Results are ranked by similarity; require_all_terms keeps only pages with every word."
	descStartIndex      = "Offset of the first result, to page through results."
	descSize            = "Number of results to return."
	descRequireAllTerms = "Only pages containing every word (strict AND). Leave false for longer queries ranked by relevance."
)

// contextKey is a private type for context values set by this package,
// avoiding collisions with keys from other packages.
type contextKey string

// authTokenContextKey carries the OAuth access token for authenticated requests.
const authTokenContextKey contextKey = "auth_token"

// forwardedTokenContextKey carries the bearer token of the HTTP request, not
// yet verified.
const forwardedTokenContextKey contextKey = "forwarded_token"

// withForwardedToken puts the request's bearer token, if any, into the context
// of that request's tool calls.
func withForwardedToken(ctx context.Context, r *http.Request) context.Context {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return ctx
	}
	return context.WithValue(ctx, forwardedTokenContextKey, token)
}

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
			schemaKeyDescription: descSize,
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

// newTool builds a read-only tool: every tool of this server only reads.
func newTool(name string, describe mcp.ToolOption, schema mcp.ToolInputSchema) mcp.Tool {
	tool := mcp.NewTool(name, describe, mcp.WithToolAnnotation(mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}))
	tool.InputSchema = schema
	return tool
}

// RegisterTools registers all search tools with the MCP server
func RegisterTools(s *server.MCPServer, client *Client, authMgr auth.AuthManager, forwarded *auth.ForwardedTokenVerifier, transport string, logger *slog.Logger, metricsCollector metrics.Collector) {
	// withLogging wraps a handler to log tool invocations and record metrics
	withLogging := func(name string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			logger.Info("tool invoked", "tool", name, "params", sanitizeForLog(request.GetArguments()))
			metricsCollector.RecordToolCall(name)
			return handler(ctx, request)
		}
	}
	// A server over HTTP without OAuth or forwarded tokens cannot serve the
	// intranet: its tools are not advertised. Over stdio they stay and explain
	// how to configure it.
	intranet := authMgr != nil || forwarded != nil || transport == transportStdio

	s.AddTool(newTool("search", tooldesc.Describe(
		"Search Giant Swarm docs, release notes, handbook, blog posts and, when signed in, the intranet; docs only: search_docs.",
		"Without sign-in, intranet results are left out and the answer says why. "+
			"Release notes (changelogs) rank last and blog posts low: to search release notes, set breadcrumb_filter [\"changes\"].",
	), searchInputSchema(map[string]interface{}{
		"type_filter": map[string]interface{}{
			schemaKeyType:        schemaTypeString,
			schemaKeyDescription: "Restrict to one source type, exact case: \"Documentation\", \"Handbook\", \"Blog\" or \"Intranet\".",
			schemaKeyDefault:     "",
		},
		"breadcrumb_filter": map[string]interface{}{
			schemaKeyType:        "array",
			schemaKeyDescription: "Section path from the top level, as a result's Breadcrumb shows it, e.g. [\"changes\"] for release notes.",
			"items": map[string]interface{}{
				schemaKeyType: schemaTypeString,
			},
			schemaKeyDefault: []interface{}{},
		},
	})), withLogging("search", optionalAuth(searchHandler(client), authMgr, forwarded)))

	s.AddTool(newTool("search_docs", tooldesc.Describe(
		"Search only the public documentation at docs.giantswarm.io; for every other source, use search.",
		"Release notes (changelogs) are included but rank last: to search them, use search with breadcrumb_filter [\"changes\"].",
	), searchInputSchema(nil)), withLogging("search_docs", searchDocsHandler(client)))

	if intranet {
		s.AddTool(newTool("search_runbook", tooldesc.Describe(
			"Find an operations runbook in the Giant Swarm intranet.",
			"Requires sign-in.",
		), searchInputSchema(nil)), withLogging("search_runbook", requireAuth(searchRunbookHandler(client), authMgr, forwarded, transport)))

		s.AddTool(newTool("search_ops_recipe", tooldesc.Describe(
			"Retired: the intranet's ops-recipes section is gone, so this finds nothing; search runbooks with search_runbook.",
			"Requires sign-in.",
		), searchInputSchema(nil)), withLogging("search_ops_recipe", requireAuth(searchOpsRecipeHandler(client), authMgr, forwarded, transport)))
	}

	s.AddTool(newTool("read_docs_url", tooldesc.Describe(
		"Read one documentation page of docs.giantswarm.io as Markdown; find it with search_docs or read_docs_index.",
		"",
	), urlInputSchema("A page on docs.giantswarm.io, e.g. https://docs.giantswarm.io/some-page/")), withLogging("read_docs_url", readDocsURLHandler(client)))

	s.AddTool(newTool("read_docs_index", tooldesc.Describe(
		"List the documentation pages of docs.giantswarm.io (llms.txt) with title, description and link; no release notes.",
		"All of it comes in one large answer: to find one page, search_docs is cheaper.",
	), mcp.ToolInputSchema{
		Type:       schemaTypeObject,
		Properties: map[string]interface{}{},
		Required:   []string{},
	}), withLogging("read_docs_index", readDocsIndexHandler(client)))

	s.AddTool(newTool("read_handbook_url", tooldesc.Describe(
		"Read one page of the public Giant Swarm handbook (handbook.giantswarm.io) as Markdown.",
		"",
	), urlInputSchema("A page on handbook.giantswarm.io, e.g. https://handbook.giantswarm.io/docs/some-page/")), withLogging("read_handbook_url", readHandbookURLHandler(client)))

	if intranet {
		s.AddTool(newTool("read_intranet_url", tooldesc.Describe(
			"Fetch one page of the Giant Swarm intranet (intranet.giantswarm.io) as Markdown.",
			"Requires sign-in.",
		), urlInputSchema("A page on intranet.giantswarm.io, e.g. https://intranet.giantswarm.io/docs/some-page/")), withLogging("read_intranet_url", requireAuth(readIntranetURLHandler(client), authMgr, forwarded, transport)))
	}
}

// optionalAuth wraps a handler that serves public content to anyone and also
// intranet content to an authenticated caller. Without a token the handler
// still runs, and its result says why intranet content is left out.
func optionalAuth(handler server.ToolHandlerFunc, authMgr auth.AuthManager, forwarded *auth.ForwardedTokenVerifier) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var token, missing string
		switch {
		case forwarded != nil:
			var err error
			token, err = forwardedToken(ctx, forwarded)
			switch {
			case err != nil:
				missing = fmt.Sprintf("the forwarded token was rejected (%v). Sign in to the MCP gateway again.", err)
			case token == "":
				missing = "the request carried no bearer token. Sign in to the MCP gateway, and have it forward your token to this server."
			}
		case authMgr != nil:
			// GetToken, not IsAuthenticated: it refreshes an expired token.
			var err error
			token, err = authMgr.GetToken(ctx)
			if err != nil {
				missing = fmt.Sprintf("not signed in (%v). Call an intranet tool, such as search_runbook, to sign in.", err)
			}
		default:
			missing = "this server has no intranet access configured."
		}

		if token != "" {
			return handler(context.WithValue(ctx, authTokenContextKey, token), request)
		}
		result, err := handler(ctx, request)
		if err == nil && result != nil && !result.IsError {
			result.Content = append(result.Content, mcp.NewTextContent("Intranet results are not included: "+missing))
		}
		return result, err
	}
}

// forwardedToken returns the caller's forwarded bearer token once verified,
// or "" when the request carried none.
func forwardedToken(ctx context.Context, forwarded *auth.ForwardedTokenVerifier) (string, error) {
	token, _ := ctx.Value(forwardedTokenContextKey).(string)
	if token == "" {
		return "", nil
	}
	if err := forwarded.Verify(ctx, token); err != nil {
		return "", err
	}
	return token, nil
}

// requireAuth wraps a handler to require authentication
func requireAuth(handler server.ToolHandlerFunc, authMgr auth.AuthManager, forwarded *auth.ForwardedTokenVerifier, transport string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Behind an MCP gateway: the caller's own forwarded token, used for
		// this request only.
		if forwarded != nil {
			token, err := forwardedToken(ctx, forwarded)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("❌ Authentication failed: %v\n\n"+
					"Sign in to the MCP gateway again.", err)), nil
			}
			if token == "" {
				return mcp.NewToolResultError(
					"❌ Authentication required\n\n" +
						"This server reaches the intranet as the caller, with the bearer token " +
						"the MCP gateway forwards. The request carried none: sign in to the gateway, " +
						"and have it forward your token to this server."), nil
			}
			return handler(context.WithValue(ctx, authTokenContextKey, token), request)
		}

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
