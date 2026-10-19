package search

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giantswarm/mcp-toolkit/tooldesc"
	"github.com/giantswarm/mcp-toolkit/tooldesc/tooldesctest"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/search-mcp/internal/auth"
	"github.com/giantswarm/search-mcp/internal/auth/authtest"
	"github.com/giantswarm/search-mcp/internal/metrics"
)

var intranetTools = []string{"search_runbook", "read_intranet_url"}

func newTestServer(transport string) *server.MCPServer {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	RegisterTools(s, NewClient(logger), nil, nil, transport, logger, metrics.NewNoopCollector())
	return s
}

// The servers the tests read from, built once per transport: over stdio every
// tool is registered, the intranet ones included; over HTTP without OAuth only
// the public ones.
var (
	stdioServer = sync.OnceValue(func() *server.MCPServer { return newTestServer(transportStdio) })
	httpServer  = sync.OnceValue(func() *server.MCPServer { return newTestServer(transportStreamableHTTP) })
)

func TestToolDescriptions(t *testing.T) {
	if err := tooldesc.CheckServer(stdioServer(), tooldesc.DefaultBudget); err != nil {
		t.Fatal(err)
	}
}

func TestToolsDocIsCurrent(t *testing.T) {
	var public []string
	for name := range httpServer().ListTools() {
		public = append(public, name)
	}
	sort.Strings(public)
	tooldesctest.AssertServerDoc(t, stdioServer(),
		"# Tools\n\nGenerated from the registered tools (`make tools-doc`). This is the stdio set; over HTTP without OAuth or forwarded tokens "+
			"only these are offered: `"+strings.Join(public, "`, `")+"`, and `search`'s description leaves out the intranet.",
		"../../docs/tools.md")
}

func TestToolsAreReadOnly(t *testing.T) {
	for name, st := range stdioServer().ListTools() {
		a := st.Tool.Annotations
		if a.ReadOnlyHint == nil || !*a.ReadOnlyHint {
			t.Errorf("%s is not annotated read-only", name)
		}
		if a.DestructiveHint == nil || *a.DestructiveHint {
			t.Errorf("%s is annotated destructive", name)
		}
		if a.IdempotentHint == nil || !*a.IdempotentHint {
			t.Errorf("%s is not annotated idempotent", name)
		}
	}
}

// TestSearchTermsAreNotStrictByDefault: agents send many terms, and strict AND
// matching left out relevant results (#111).
func TestSearchTermsAreNotStrictByDefault(t *testing.T) {
	const why = "agents send many terms, and strict AND matching left out relevant results"
	if q := NewClient(slog.New(slog.NewTextHandler(io.Discard, nil))).buildQuery(SearchRequest{Term: "two terms"}); !strings.Contains(fmt.Sprint(q.Query), "multi_match") {
		t.Errorf("a search without require_all_terms is not ranked by similarity (#111: %s)", why)
	}
	for _, name := range []string{"search", "search_docs", "search_runbook"} {
		tool := stdioServer().GetTool(name)
		if tool == nil {
			t.Fatalf("%s not registered", name)
		}
		props := tool.Tool.InputSchema.Properties
		if prop, ok := props[paramRequireAllTerms].(map[string]interface{}); !ok || prop[schemaKeyDefault] != false {
			t.Errorf("%s.%s must default to false (#111: %s)", name, paramRequireAllTerms, why)
		}
		for param, fact := range map[string]string{paramTerm: "every word", paramRequireAllTerms: "longer queries"} {
			prop, ok := props[param].(map[string]interface{})
			if !ok {
				t.Fatalf("%s has no parameter %s", name, param)
			}
			d, _ := prop[schemaKeyDescription].(string)
			if !strings.Contains(d, fact) {
				t.Errorf("%s.%s lost %q from #111: %s", name, param, fact, why)
			}
		}
	}
}

func TestRegisterTools_HTTPWithoutOAuthHidesIntranetTools(t *testing.T) {
	tools := httpServer().ListTools()
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
	tools := stdioServer().ListTools()
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

// stubAuthManager is an OAUTH_* login whose GetToken returns token or err.
type stubAuthManager struct {
	auth.AuthManager
	token string
	err   error
}

func (m stubAuthManager) GetToken(context.Context) (string, error) { return m.token, m.err }

func TestOptionalAuth(t *testing.T) {
	verifier, issuer := forwardedVerifier(t)
	valid := issuer.Token(t, issuer.URL, "searchmcp", "alice", time.Hour)

	tests := map[string]struct {
		authMgr   auth.AuthManager
		forwarded *auth.ForwardedTokenVerifier
		token     string
		wantToken string
		wantNote  string
	}{
		"forwarded token": {forwarded: verifier, token: valid, wantToken: valid},
		"no forwarded token": {forwarded: verifier,
			wantNote: "the request carried no bearer token"},
		"rejected forwarded token": {forwarded: verifier,
			token:    issuer.Token(t, issuer.URL, "other", "alice", time.Hour),
			wantNote: "the forwarded token was rejected"},
		"signed in": {authMgr: stubAuthManager{token: valid}, wantToken: valid},
		"not signed in": {authMgr: stubAuthManager{err: auth.ErrTokenNotFound},
			wantNote: "not signed in"},
		"no intranet access": {wantNote: "no intranet access configured"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var seen string
			handler := optionalAuth(func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				seen, _ = ctx.Value(authTokenContextKey).(string)
				return mcp.NewToolResultText("public results"), nil
			}, tc.authMgr, tc.forwarded)

			ctx := context.Background()
			if tc.token != "" {
				ctx = context.WithValue(ctx, forwardedTokenContextKey, tc.token)
			}
			result, err := handler(ctx, mcp.CallToolRequest{})
			if err != nil {
				t.Fatal(err)
			}

			// A caller without a usable token still gets the public results.
			if result.IsError {
				t.Fatalf("unexpected error result %v", result.Content)
			}
			if seen != tc.wantToken {
				t.Errorf("handler got token %q, want %q", seen, tc.wantToken)
			}
			var note string
			if len(result.Content) > 1 {
				note = result.Content[len(result.Content)-1].(mcp.TextContent).Text
			}
			if tc.wantNote == "" && note != "" {
				t.Errorf("unexpected note %q", note)
			}
			if !strings.Contains(note, tc.wantNote) {
				t.Errorf("note %q does not say %q", note, tc.wantNote)
			}
		})
	}
}
