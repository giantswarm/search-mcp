package search

import (
	"bytes"
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

// Limits of the public search gateway at docs.giantswarm.io/searchapi/. It
// refuses requests outside them.
const (
	publicMaxSize        = 50
	publicMaxStartIndex  = 1000
	publicMaxBreadcrumbs = 5
	publicMaxTermLength  = 200
)

// Elasticsearch query keys and field names reused across query construction.
const (
	esKeyQuery  = "query"
	esKeyFilter = "filter"
	esKeyTerm   = "term"
	esKeyType   = "type"
	esKeyFields = "fields"
	esKeyWeight = "weight"
	esFieldBody = "body"
	esFieldText = "text"
)

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

// Search performs a search query against the Giant Swarm documentation.
// With a sign-in token it sends an OpenSearch query to the intranet endpoint.
// Without one it uses the public search gateway, which takes only a few
// query parameters and never a query body.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	token, _ := ctx.Value(authTokenContextKey).(string)
	if token == "" {
		return c.searchPublic(ctx, req)
	}

	payload, err := json.Marshal(c.buildQuery(req))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.intranetEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	c.logger.Debug("using authenticated intranet search endpoint")

	return c.do(httpReq)
}

// searchPublic searches docs, handbook and blog through the public search
// gateway at docs.giantswarm.io/searchapi/.
func (c *Client) searchPublic(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// The public gateway never serves the intranet: answer with no results,
	// as the public index did before.
	if req.TypeFilter == "Intranet" {
		return &SearchResponse{}, nil
	}
	if len([]rune(req.Term)) > publicMaxTermLength {
		return nil, fmt.Errorf("search term is longer than %d characters", publicMaxTermLength)
	}
	if len(req.BreadcrumbFilter) > publicMaxBreadcrumbs {
		return nil, fmt.Errorf("breadcrumb_filter has more than %d segments", publicMaxBreadcrumbs)
	}
	if req.StartIndex < 0 || req.StartIndex > publicMaxStartIndex {
		return nil, fmt.Errorf("start_index must be between 0 and %d", publicMaxStartIndex)
	}
	size := req.Size
	if size < 1 {
		size = 1
	}
	if size > publicMaxSize {
		size = publicMaxSize
	}

	params := url.Values{}
	params.Set("q", req.Term)
	if req.TypeFilter != "" {
		params.Set("type", req.TypeFilter)
	}
	for _, b := range req.BreadcrumbFilter {
		params.Add("breadcrumb", b)
	}
	params.Set("from", strconv.Itoa(req.StartIndex))
	params.Set("size", strconv.Itoa(size))
	if req.RequireAllTerms {
		params.Set("match", "all")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.publicEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

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

// buildQuery constructs an Elasticsearch/OpenSearch query from a SearchRequest
func (c *Client) buildQuery(req SearchRequest) ElasticsearchQuery {
	// Build the inner query based on matching mode
	var innerQuery map[string]interface{}
	if req.RequireAllTerms {
		// Strict AND mode: all search terms must be present
		innerQuery = map[string]interface{}{
			"simple_query_string": map[string]interface{}{
				esKeyFields:        []string{"title^5", esFieldBody, esFieldText},
				"default_operator": "AND",
				esKeyQuery:         req.Term,
			},
		}
	} else {
		// Default similarity mode: documents are ranked by how well they
		// match the search terms, without requiring all terms to be present.
		innerQuery = map[string]interface{}{
			"multi_match": map[string]interface{}{
				esKeyQuery:             req.Term,
				esKeyFields:            []string{"title^5", esFieldBody, esFieldText},
				esKeyType:              "best_fields",
				"operator":             "or",
				"minimum_should_match": "30%",
				"tie_breaker":          0.3,
			},
		}
	}

	// Wrap in function_score for type/breadcrumb boosting
	baseQuery := map[string]interface{}{
		"function_score": map[string]interface{}{
			esKeyQuery: innerQuery,
			"functions": []map[string]interface{}{
				{esKeyFilter: map[string]interface{}{esKeyTerm: map[string]string{esKeyType: "Intranet"}}, esKeyWeight: 10},
				{esKeyFilter: map[string]interface{}{esKeyTerm: map[string]string{esKeyType: "Blog"}}, esKeyWeight: 0.01},
				{esKeyFilter: map[string]interface{}{esKeyTerm: map[string]string{"breadcrumb": "changes"}}, esKeyWeight: 0.0001},
				{esKeyFilter: map[string]interface{}{esKeyTerm: map[string]string{"breadcrumb": "api"}}, esKeyWeight: 0.0001},
			},
		},
	}

	// Apply filters if specified
	var mustClauses []map[string]interface{}
	mustClauses = append(mustClauses, baseQuery)

	// Add type filter
	if req.TypeFilter != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			esKeyTerm: map[string]string{esKeyType: req.TypeFilter},
		})
	}

	// Add breadcrumb filters
	// The index has both 'breadcrumb' (array) and 'breadcrumb_1', 'breadcrumb_2', etc. (positional strings)
	// We use the positional fields to match specific positions in the hierarchy
	// These are keyword fields, so we use 'term' queries for exact matching
	if len(req.BreadcrumbFilter) > 0 {
		for i, breadcrumb := range req.BreadcrumbFilter {
			fieldName := fmt.Sprintf("breadcrumb_%d", i+1)
			mustClauses = append(mustClauses, map[string]interface{}{
				esKeyTerm: map[string]string{fieldName: breadcrumb},
			})
		}
	}

	// Build final query
	var finalQuery map[string]interface{}
	if len(mustClauses) > 1 {
		finalQuery = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": mustClauses,
			},
		}
	} else {
		finalQuery = baseQuery
	}

	return ElasticsearchQuery{
		From: req.StartIndex,
		Size: req.Size,
		Sort: []string{"_score"},
		Source: map[string][]string{
			"excludes": {esFieldText, esFieldBody},
		},
		Query: finalQuery,
		Highlight: map[string]interface{}{
			"pre_tags":  []string{"**"},
			"post_tags": []string{"**"},
			esKeyFields: map[string]interface{}{
				esFieldBody: map[string]interface{}{
					esKeyType:             "unified",
					"number_of_fragments": 1,
					"no_match_size":       200,
					"fragment_size":       150,
				},
				"title": map[string]interface{}{
					esKeyType:             "unified",
					"number_of_fragments": 1,
				},
			},
		},
	}
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
