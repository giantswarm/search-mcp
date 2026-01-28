package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	publicSearchEndpoint   = "https://docs.giantswarm.io/searchapi/"
	intranetSearchEndpoint = "https://intranet-searchmcp.giantswarm.io/searchapi/"
	searchTimeout          = 30 * time.Second
	fetchTimeout           = 60 * time.Second
)

// Client handles search operations
type Client struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new search client
func NewClient(logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: searchTimeout,
		},
		logger: logger,
	}
}

// Search performs a search query against the Giant Swarm documentation
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	// Build Elasticsearch/OpenSearch query
	query := c.buildQuery(req)

	// Marshal to JSON
	payload, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Determine endpoint based on auth token presence
	endpoint := publicSearchEndpoint
	var authToken string
	if token, ok := ctx.Value("auth_token").(string); ok && token != "" {
		endpoint = intranetSearchEndpoint
		authToken = token
		c.logger.Debug("using authenticated intranet search endpoint")
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Add auth token if present
	if authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
		c.logger.Debug("auth token added to search request")
	}

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

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
		if token, ok := ctx.Value("auth_token").(string); ok && token != "" {
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
	defer resp.Body.Close()

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
	// Base query with function_score
	baseQuery := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query": map[string]interface{}{
				"simple_query_string": map[string]interface{}{
					"fields":           []string{"title^5", "uri^5", "description^5", "text"},
					"default_operator": "AND",
					"query":            req.Term,
				},
			},
			"functions": []map[string]interface{}{
				{"filter": map[string]interface{}{"term": map[string]string{"type": "Intranet"}}, "weight": 10},
				{"filter": map[string]interface{}{"term": map[string]string{"type": "Blog"}}, "weight": 0.01},
				{"filter": map[string]interface{}{"term": map[string]string{"breadcrumb": "changes"}}, "weight": 0.0001},
				{"filter": map[string]interface{}{"term": map[string]string{"breadcrumb": "api"}}, "weight": 0.0001},
			},
		},
	}

	// Apply filters if specified
	var mustClauses []map[string]interface{}
	mustClauses = append(mustClauses, baseQuery)

	// Add type filter
	if req.TypeFilter != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]string{"type": req.TypeFilter},
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
				"term": map[string]string{fieldName: breadcrumb},
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
			"excludes": {"text", "body"},
		},
		Query: finalQuery,
		Highlight: map[string]interface{}{
			"fields": map[string]interface{}{
				"body": map[string]interface{}{
					"type":                "unified",
					"number_of_fragments": 1,
					"no_match_size":       200,
					"fragment_size":       150,
				},
				"title": map[string]interface{}{
					"type":                "unified",
					"number_of_fragments": 1,
				},
			},
		},
	}
}

// FormatSearchResults formats search results as Markdown
func FormatSearchResults(term string, startIndex int, resp *SearchResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Search results for %s\n\n", term))
	sb.WriteString(fmt.Sprintf("Showing %d out of %d search results", len(resp.Hits.Hits), resp.Hits.Total.Value))
	if startIndex > 0 {
		sb.WriteString(fmt.Sprintf(", starting at %d", startIndex+1))
	}
	sb.WriteString("\n\n")

	for i, hit := range resp.Hits.Hits {
		n := startIndex + i + 1
		source := hit.Source

		sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**\n", n, source.Title, source.URL))
		sb.WriteString(fmt.Sprintf("   **Type:** %s\n", source.Type))

		if len(source.Breadcrumb) > 0 {
			breadcrumbStr := strings.Join(source.Breadcrumb, " / ")
			sb.WriteString(fmt.Sprintf("   **Breadcrumb:** %s\n", breadcrumbStr))
		}

		if source.Description != "" {
			sb.WriteString(fmt.Sprintf("   **Description:** %s\n", source.Description))
		}

		// Add excerpt from highlight if available
		if bodyHighlights, ok := hit.Highlight["body"]; ok && len(bodyHighlights) > 0 {
			sb.WriteString(fmt.Sprintf("   **Excerpt:** %s\n", bodyHighlights[0]))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
