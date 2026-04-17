package search

import "encoding/json"

// SearchRequest represents a search query request
type SearchRequest struct {
	Term             string
	StartIndex       int
	Size             int
	TypeFilter       string
	BreadcrumbFilter []string
	RequireAllTerms  bool
}

// SearchResponse represents the response from Elasticsearch/OpenSearch
type SearchResponse struct {
	Hits struct {
		Total TotalHits `json:"total"`
		Hits  []Hit     `json:"hits"`
	} `json:"hits"`
}

// TotalHits can handle both ES 6.x (int) and ES 7.x+/OpenSearch (object) formats
type TotalHits struct {
	Value int
}

// UnmarshalJSON handles both integer and object formats for total hits
// - ES 6.x returns: "total": 42
// - ES 7.x+/OpenSearch returns: "total": {"value": 42, "relation": "eq"}
func (t *TotalHits) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as an integer first (ES 6.x format)
	var value int
	if err := json.Unmarshal(data, &value); err == nil {
		t.Value = value
		return nil
	}

	// Try to unmarshal as an object (ES 7.x+/OpenSearch format)
	var obj struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	t.Value = obj.Value
	return nil
}

// Hit represents a single search result
type Hit struct {
	Source    HitSource           `json:"_source"`
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

// ElasticsearchQuery represents the Elasticsearch/OpenSearch query structure
type ElasticsearchQuery struct {
	From      int                    `json:"from"`
	Size      int                    `json:"size"`
	Sort      []string               `json:"sort"`
	Source    map[string][]string    `json:"_source"`
	Query     map[string]interface{} `json:"query"`
	Highlight map[string]interface{} `json:"highlight"`
}
