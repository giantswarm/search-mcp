package search

// SearchRequest represents a search query request
type SearchRequest struct {
	Term             string
	StartIndex       int
	Size             int
	TypeFilter       string
	BreadcrumbFilter []string
}

// SearchResponse represents the response from Elasticsearch
type SearchResponse struct {
	Hits struct {
		Total int `json:"total"`
		Hits  []Hit `json:"hits"`
	} `json:"hits"`
}

// Hit represents a single search result
type Hit struct {
	Source    HitSource         `json:"_source"`
	Highlight map[string][]string `json:"highlight,omitempty"`
}

// HitSource contains the document fields
type HitSource struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url"`
	Type        string   `json:"type"`
	Breadcrumb  []string `json:"breadcrumb,omitempty"`
}

// ElasticsearchQuery represents the Elasticsearch query structure
type ElasticsearchQuery struct {
	From    int                    `json:"from"`
	Size    int                    `json:"size"`
	Sort    []string               `json:"sort"`
	Source  map[string][]string    `json:"_source"`
	Query   map[string]interface{} `json:"query"`
	Highlight map[string]interface{} `json:"highlight"`
}
