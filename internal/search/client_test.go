package search

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const testSearchResponse = `{"hits":{"total":{"value":1},"hits":[{"_source":{"title":"Scale node pools","url":"https://docs.giantswarm.io/x/","type":"Documentation","breadcrumb":["tutorials"]},"highlight":{"body":["**node** pools"]}}]}}`

// recordedRequest is what a test server saw.
type recordedRequest struct {
	method string
	path   string
	query  url.Values
	body   string
	auth   string
}

func newTestClient(t *testing.T) (*Client, *recordedRequest) {
	t.Helper()
	seen := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*seen = recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.Query(), body: string(body), auth: r.Header.Get("Authorization")}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, testSearchResponse)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.publicEndpoint = srv.URL + "/searchapi/"
	c.intranetEndpoint = srv.URL + "/intranet/searchapi/"
	return c, seen
}

func TestSearchPublicSendsOnlyQueryParameters(t *testing.T) {
	c, seen := newTestClient(t)

	resp, err := c.Search(context.Background(), SearchRequest{
		Term:             "node pools",
		StartIndex:       10,
		Size:             5,
		TypeFilter:       "Documentation",
		BreadcrumbFilter: []string{"tutorials", "node-pools"},
		RequireAllTerms:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if seen.method != http.MethodGet {
		t.Errorf("method = %s, want GET", seen.method)
	}
	if seen.body != "" {
		t.Errorf("body = %q, want none", seen.body)
	}
	if seen.auth != "" {
		t.Errorf("Authorization = %q, want none", seen.auth)
	}
	want := url.Values{
		"q":              {"node pools"},
		"type":           {"Documentation"},
		"breadcrumb":     {"tutorials", "node-pools"},
		gatewayParamFrom: {"10"},
		gatewayParamSize: {"5"},
		"match":          {"all"},
	}
	if !reflect.DeepEqual(seen.query, want) {
		t.Errorf("query = %v, want %v", seen.query, want)
	}
	if resp.Hits.Total.Value != 1 || len(resp.Hits.Hits) != 1 || resp.Hits.Hits[0].Source.Title != "Scale node pools" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestSearchPublicDefaults(t *testing.T) {
	c, seen := newTestClient(t)

	if _, err := c.Search(context.Background(), SearchRequest{Term: "x", Size: 500}); err != nil {
		t.Fatal(err)
	}
	want := url.Values{"q": {"x"}, gatewayParamFrom: {"0"}, gatewayParamSize: {"50"}}
	if !reflect.DeepEqual(seen.query, want) {
		t.Errorf("query = %v, want %v", seen.query, want)
	}
}

func TestSearchPublicIntranetTypeReturnsNothing(t *testing.T) {
	c, seen := newTestClient(t)

	resp, err := c.Search(context.Background(), SearchRequest{Term: "x", Size: 10, TypeFilter: typeIntranet})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits.Hits) != 0 {
		t.Errorf("got %d hits, want none", len(resp.Hits.Hits))
	}
	if seen.method != "" {
		t.Errorf("sent a %s request, want none", seen.method)
	}
}

func TestSearchPublicRejectsOutOfRangeRequests(t *testing.T) {
	cases := map[string]SearchRequest{
		"long term":       {Term: strings.Repeat("a", 201), Size: 10},
		"breadcrumbs":     {Term: "x", Size: 10, BreadcrumbFilter: []string{"a", "b", "c", "d", "e", "f"}},
		"start too large": {Term: "x", Size: 10, StartIndex: 1001},
		"negative start":  {Term: "x", Size: 10, StartIndex: -1},
		"blank term":      {Term: "   ", Size: 10},
		"unknown type":    {Term: "x", Size: 10, TypeFilter: "documentation"},
		"empty segment":   {Term: "x", Size: 10, BreadcrumbFilter: []string{""}},
		"long segment":    {Term: "x", Size: 10, BreadcrumbFilter: []string{strings.Repeat("a", 101)}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			c, seen := newTestClient(t)
			if _, err := c.Search(context.Background(), req); err == nil {
				t.Error("want an error")
			}
			if seen.method != "" {
				t.Errorf("sent a %s request, want none", seen.method)
			}
		})
	}
}

func TestSearchSignedInUsesIntranetGateway(t *testing.T) {
	c, seen := newTestClient(t)
	ctx := context.WithValue(context.Background(), authTokenContextKey, "token")

	if _, err := c.Search(ctx, SearchRequest{Term: "runbook", Size: 10, TypeFilter: typeIntranet,
		BreadcrumbFilter: []string{"support-and-ops", "runbooks"}}); err != nil {
		t.Fatal(err)
	}
	if seen.method != http.MethodGet {
		t.Errorf("method = %s, want GET", seen.method)
	}
	if seen.body != "" {
		t.Errorf("body = %q, want none", seen.body)
	}
	if seen.auth != "Bearer token" {
		t.Errorf("Authorization = %q", seen.auth)
	}
	if seen.path != "/intranet/searchapi/" {
		t.Errorf("path = %s, want the intranet endpoint", seen.path)
	}
	want := url.Values{
		"q":              {"runbook"},
		"type":           {typeIntranet},
		"breadcrumb":     {"support-and-ops", "runbooks"},
		gatewayParamFrom: {"0"},
		gatewayParamSize: {"10"},
	}
	if !reflect.DeepEqual(seen.query, want) {
		t.Errorf("query = %v, want %v", seen.query, want)
	}
}
