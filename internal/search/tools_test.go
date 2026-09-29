package search

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/giantswarm/search-mcp/internal/auth/authtest"
	"github.com/giantswarm/search-mcp/internal/metrics"
)

var intranetTools = []string{"search_runbook", "search_ops_recipe", "read_intranet_url"}

func registeredTools(t *testing.T, transport string) map[string]*server.ServerTool {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	RegisterTools(s, NewClient(logger), nil, nil, transport, logger, metrics.NewNoopCollector())
	return s.ListTools()
}

func TestRegisterTools_HTTPWithoutOAuthHidesIntranetTools(t *testing.T) {
	tools := registeredTools(t, transportStreamableHTTP)
	for _, name := range intranetTools {
		if _, ok := tools[name]; ok {
			t.Errorf("tool %q is advertised over HTTP without OAuth", name)
		}
	}
	for _, name := range []string{"search", "search_docs", "read_docs_url", "read_docs_index", "read_handbook_url"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("public tool %q is missing", name)
		}
	}
}

func TestRegisterTools_StdioWithoutOAuthKeepsIntranetTools(t *testing.T) {
	tools := registeredTools(t, transportStdio)
	for _, name := range intranetTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q is missing over stdio", name)
		}
	}
}

func forwardedVerifier(t *testing.T) (*auth.ForwardedTokenVerifier, *authtest.Issuer) {
	t.Helper()
	issuer := authtest.NewIssuer(t)
	verifier, err := auth.NewForwardedTokenVerifier(context.Background(),
		auth.ForwardedConfig{IssuerURL: issuer.URL, Audience: "searchmcp"})
	if err != nil {
		t.Fatal(err)
	}
	return verifier, issuer
}

func TestRegisterTools_HTTPWithForwardedTokensAdvertisesIntranetTools(t *testing.T) {
	verifier, _ := forwardedVerifier(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	RegisterTools(s, NewClient(logger), nil, verifier, transportStreamableHTTP, logger, metrics.NewNoopCollector())
	tools := s.ListTools()
	for _, name := range intranetTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("tool %q is missing over HTTP with forwarded tokens", name)
		}
	}
}

func TestWithForwardedToken(t *testing.T) {
	tests := map[string]string{
		"Bearer abc.def": "abc.def",
		"Bearer ":        "",
		"Basic abc":      "",
		"":               "",
	}
	for header, want := range tests {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		got, _ := withForwardedToken(context.Background(), r).Value(forwardedTokenContextKey).(string)
		if got != want {
			t.Errorf("Authorization %q: token %q, want %q", header, got, want)
		}
	}
}

func TestRequireAuth_ForwardedTokens(t *testing.T) {
	verifier, issuer := forwardedVerifier(t)
	var seen string
	handler := requireAuth(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		seen, _ = ctx.Value(authTokenContextKey).(string)
		return mcp.NewToolResultText("ok"), nil
	}, nil, verifier, transportStreamableHTTP)

	call := func(token string) *mcp.CallToolResult {
		t.Helper()
		seen = ""
		ctx := context.Background()
		if token != "" {
			ctx = context.WithValue(ctx, forwardedTokenContextKey, token)
		}
		result, err := handler(ctx, mcp.CallToolRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	// Two callers of one server each reach the intranet with their own token.
	for _, user := range []string{"alice", "bob"} {
		token := issuer.Token(t, issuer.URL, "searchmcp", user, time.Hour)
		if result := call(token); result.IsError {
			t.Fatalf("%s: unexpected error result %v", user, result.Content)
		}
		if seen != token {
			t.Errorf("%s: handler got another token", user)
		}
	}

	// Without a token, or with one the issuer did not sign for this
	// audience, the handler is not reached.
	for name, token := range map[string]string{
		"no token":       "",
		"wrong audience": issuer.Token(t, issuer.URL, "other", "alice", time.Hour),
		"expired":        issuer.Token(t, issuer.URL, "searchmcp", "alice", -time.Minute),
	} {
		if result := call(token); !result.IsError {
			t.Errorf("%s: want an error result", name)
		}
		if seen != "" {
			t.Errorf("%s: handler was reached", name)
		}
	}
}

func TestOptionalAuth_ForwardedTokens(t *testing.T) {
	verifier, issuer := forwardedVerifier(t)
	var reached bool
	var seen string
	handler := optionalAuth(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		reached = true
		seen, _ = ctx.Value(authTokenContextKey).(string)
		return mcp.NewToolResultText("ok"), nil
	}, nil, verifier)

	call := func(token string) *mcp.CallToolResult {
		t.Helper()
		reached, seen = false, ""
		ctx := context.Background()
		if token != "" {
			ctx = context.WithValue(ctx, forwardedTokenContextKey, token)
		}
		result, err := handler(ctx, mcp.CallToolRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	// A caller with a valid token searches with it.
	token := issuer.Token(t, issuer.URL, "searchmcp", "alice", time.Hour)
	if result := call(token); result.IsError {
		t.Fatalf("unexpected error result %v", result.Content)
	}
	if seen != token {
		t.Error("handler did not get the caller's token")
	}

	// A caller without a token still searches public content.
	if result := call(""); result.IsError {
		t.Fatalf("no token: unexpected error result %v", result.Content)
	}
	if !reached || seen != "" {
		t.Errorf("no token: reached %v, token %q", reached, seen)
	}

	// A token that fails verification is reported, not silently dropped.
	if result := call(issuer.Token(t, issuer.URL, "other", "alice", time.Hour)); !result.IsError {
		t.Error("wrong audience: want an error result")
	}
	if reached {
		t.Error("wrong audience: handler was reached")
	}
}
