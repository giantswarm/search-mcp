package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	publicSearchEndpoint   = "https://docs.giantswarm.io/searchapi/"
	intranetSearchEndpoint = "https://intranet-searchmcp.giantswarm.io/searchapi/"
	searchTimeout          = 30 * time.Second
	fetchTimeout           = 60 * time.Second
)

// Limits of the search gateways (docs.giantswarm.io/searchapi/ and the
// intranet's). They refuse requests outside them.
const (
	gatewayMaxSize             = 50
	gatewayMaxStartIndex       = 1000
	gatewayMaxBreadcrumbs      = 5
	gatewayMaxBreadcrumbLength = 100
	gatewayMaxTermLength       = 200
)

// Query parameters of the search gateways.
const (
	gatewayParamFrom = "from"
	gatewayParamSize = paramSize
)

// typeIntranet is the type of intranet pages, which only the intranet's
// gateway serves.
const typeIntranet = "Intranet"

// esFieldBody is the highlight field the result excerpts come from.
const esFieldBody = "body"

// Client handles search operations
type Client struct {
	httpClient       *http.Client
	logger           *slog.Logger
	publicEndpoint   string
	intranetEndpoint string
}

// NewClient creates a new search client
func NewClient(logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: searchTimeout,
		},
		logger:           logger,
		publicEndpoint:   publicSearchEndpoint,
		intranetEndpoint: intranetSearchEndpoint,
	}
}

// Search searches the Giant Swarm documentation through a search gateway,
// which takes only a few query parameters and never a query body. With a
// sign-in token it uses the intranet's gateway, which also serves intranet
// pages; without one, the public gateway at docs.giantswarm.io/searchapi/.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	token, _ := ctx.Value(authTokenContextKey).(string)
	if token == "" {
		// The public gateway never serves the intranet: answer with no
		// results, as the public index did before.
		if req.TypeFilter == typeIntranet {
			return &SearchResponse{}, nil
		}
		return c.searchGateway(ctx, c.publicEndpoint, "", req)
	}
	c.logger.Debug("using authenticated intranet search endpoint")
	return c.searchGateway(ctx, c.intranetEndpoint, token, req)
}

// searchGateway sends a search as query parameters to a search gateway,
// checking it against the gateway's limits first.
func (c *Client) searchGateway(ctx context.Context, endpoint, token string, req SearchRequest) (*SearchResponse, error) {
	term := strings.TrimSpace(req.Term)
	if term == "" {
		return nil, fmt.Errorf("search term is empty")
	}
	if len([]rune(term)) > gatewayMaxTermLength {
		return nil, fmt.Errorf("search term is longer than %d characters", gatewayMaxTermLength)
	}
	switch req.TypeFilter {
	case "", "Documentation", "Handbook", "Blog":
	case typeIntranet:
		if token == "" {
			return &SearchResponse{}, nil
		}
	default:
		return nil, fmt.Errorf("type_filter must be one of Documentation, Handbook, Blog or Intranet, exact case")
	}
	if len(req.BreadcrumbFilter) > gatewayMaxBreadcrumbs {
		return nil, fmt.Errorf("breadcrumb_filter has more than %d segments", gatewayMaxBreadcrumbs)
	}
	for _, b := range req.BreadcrumbFilter {
		if n := len([]rune(b)); n == 0 || n > gatewayMaxBreadcrumbLength {
			return nil, fmt.Errorf("each breadcrumb_filter segment must be 1 to %d characters", gatewayMaxBreadcrumbLength)
		}
	}
	if req.StartIndex < 0 || req.StartIndex > gatewayMaxStartIndex {
		return nil, fmt.Errorf("start_index must be between 0 and %d", gatewayMaxStartIndex)
	}
	size := req.Size
	if size < 1 {
		size = 1
	}
	if size > gatewayMaxSize {
		size = gatewayMaxSize
	}

	params := url.Values{}
	params.Set("q", term)
	if req.TypeFilter != "" {
		params.Set("type", req.TypeFilter)
	}
	for _, b := range req.BreadcrumbFilter {
		params.Add("breadcrumb", b)
	}
	params.Set(gatewayParamFrom, strconv.Itoa(req.StartIndex))
	params.Set(gatewayParamSize, strconv.Itoa(size))
	if req.RequireAllTerms {
		params.Set("match", "all")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	return c.do(httpReq)
}

// do sends a search request and decodes the OpenSearch-shaped response.
func (c *Client) do(httpReq *http.Request) (*SearchResponse, error) {
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("search results", "total", searchResp.Hits.Total, "returned", len(searchResp.Hits.Hits))

	return &searchResp, nil
}

// FetchURL fetches content from a URL
func (c *Client) FetchURL(ctx context.Context, url string) (string, error) {
	// Check if URL requires authentication (intranet)
	requiresAuth := strings.HasPrefix(url, "https://intranet.giantswarm.io/")

	// CRITICAL: Domain replacement for JWT authentication
	// User-facing URL: https://intranet.giantswarm.io/
	// Actual API URL:  https://intranet-searchmcp.giantswarm.io/
	if requiresAuth {
		url = strings.Replace(url, "intranet.giantswarm.io", "intranet-searchmcp.giantswarm.io", 1)
		c.logger.Debug("domain replacement performed",
			"original_domain", "intranet.giantswarm.io",
			"target_domain", "intranet-searchmcp.giantswarm.io")
	}

	// Create a client with longer timeout for fetching pages
	client := &http.Client{
		Timeout: fetchTimeout,
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Inject Bearer token if available in context (for authenticated requests)
	if requiresAuth {
		if token, ok := ctx.Value(authTokenContextKey).(string); ok && token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
			c.logger.Debug("auth token injected into request")
		} else {
			c.logger.Warn("intranet URL requested but no auth token in context")
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle auth-specific status codes
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication failed (401 Unauthorized): token may be invalid or expired")
	}

	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("access denied (403 Forbidden): insufficient permissions")
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("page not found: %s", url)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// FormatSearchResults formats search results as Markdown
func FormatSearchResults(term string, startIndex int, resp *SearchResponse) string {
	var sb strings.Builder

	_, _ = fmt.Fprintf(&sb, "# Search results for %s\n\n", term)
	_, _ = fmt.Fprintf(&sb, "Showing %d out of %d search results", len(resp.Hits.Hits), resp.Hits.Total.Value)
	if startIndex > 0 {
		_, _ = fmt.Fprintf(&sb, ", starting at %d", startIndex+1)
	}
	sb.WriteString("\n\n")

	for i, hit := range resp.Hits.Hits {
		n := startIndex + i + 1
		source := hit.Source

		_, _ = fmt.Fprintf(&sb, "%d. **[%s](%s)**\n", n, source.Title, source.URL)
		_, _ = fmt.Fprintf(&sb, "   **Type:** %s\n", source.Type)

		if len(source.Breadcrumb) > 0 {
			breadcrumbStr := strings.Join(source.Breadcrumb, " / ")
			_, _ = fmt.Fprintf(&sb, "   **Breadcrumb:** %s\n", breadcrumbStr)
		}

		if source.Description != "" {
			_, _ = fmt.Fprintf(&sb, "   **Description:** %s\n", source.Description)
		}

		// Add excerpt from highlight if available
		if bodyHighlights, ok := hit.Highlight[esFieldBody]; ok && len(bodyHighlights) > 0 {
			excerpt := bodyHighlights[0]
			// Add ellipsis to indicate truncation
			runes := []rune(excerpt)
			if len(runes) > 0 && !unicode.IsUpper(runes[0]) {
				excerpt = "…" + excerpt
			}
			if len(runes) > 0 && !strings.ContainsAny(string(runes[len(runes)-1:]), ".!?") {
				excerpt = excerpt + "…"
			}
			_, _ = fmt.Fprintf(&sb, "   **Excerpt:** %s\n", excerpt)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
