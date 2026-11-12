package search

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/search-mcp/internal/auth/authtest"
)

// TestServer_ForwardedTokenOverHTTP drives a server configured for forwarded
// tokens through its streamable HTTP handler, as an MCP gateway would.
func TestServer_ForwardedTokenOverHTTP(t *testing.T) {
	issuer := authtest.NewIssuer(t)
	t.Setenv("FORWARDED_TOKEN_ISSUER_URL", issuer.URL)
	t.Setenv("FORWARDED_TOKEN_AUDIENCE", "searchmcp")
	t.Setenv("OAUTH_ISSUER_URL", "")

	srv, err := NewServer(ServerConfig{Transport: transportStreamableHTTP, HTTPEndpoint: testEndpoint, MaxRequestBodyBytes: DefaultMCPRequestBodyBytes})
	if err != nil {
		t.Fatal(err)
	}
	if srv.authMgr != nil {
		t.Fatal("the process-wide login is set up next to forwarded tokens")
	}
	httpServer := httptest.NewServer(srv.streamableHTTPServer())
	t.Cleanup(httpServer.Close)

	call := func(t *testing.T, headers map[string]string) (*mcp.ListToolsResult, *mcp.CallToolResult) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, err := client.NewStreamableHttpClient(httpServer.URL+testEndpoint, transport.WithHTTPHeaders(headers))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Close() }()
		if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
			t.Fatal(err)
		}
		tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		request := mcp.CallToolRequest{}
		request.Params.Name = "read_intranet_url"
		request.Params.Arguments = map[string]any{"url": "https://intranet.giantswarm.io/docs/"}
		result, err := c.CallTool(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		return tools, result
	}

	text := func(result *mcp.CallToolResult) string {
		var b strings.Builder
		for _, content := range result.Content {
			if tc, ok := content.(mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String()
	}

	t.Run("without a token", func(t *testing.T) {
		tools, result := call(t, nil)
		advertised := map[string]bool{}
		for _, tool := range tools.Tools {
			advertised[tool.Name] = true
		}
		for _, name := range intranetTools {
			if !advertised[name] {
				t.Errorf("tool %q is not advertised", name)
			}
		}
		if !result.IsError || !strings.Contains(text(result), "Authentication required") {
			t.Errorf("want an authentication-required error, got %q", text(result))
		}
	})

	t.Run("with a token for another audience", func(t *testing.T) {
		token := issuer.Token(t, issuer.URL, "other", "alice", time.Hour)
		_, result := call(t, map[string]string{"Authorization": "Bearer " + token})
		if !result.IsError || !strings.Contains(text(result), "Authentication failed") {
			t.Errorf("want an authentication-failed error, got %q", text(result))
		}
	})
}

// A body beyond the bound is refused with 413, whether its length is
// declared or not; a body of exactly the bound is not, and the endpoint keeps
// answering.
func TestServer_MCPBodyIsBounded(t *testing.T) {
	for _, limit := range []int64{DefaultMCPRequestBodyBytes, 4096} {
		t.Run(strconv.FormatInt(limit, 10), func(t *testing.T) {
			t.Setenv("FORWARDED_TOKEN_ISSUER_URL", "")
			t.Setenv("OAUTH_ISSUER_URL", "")
			srv, err := NewServer(ServerConfig{Transport: transportStreamableHTTP, HTTPEndpoint: testEndpoint, MaxRequestBodyBytes: limit})
			if err != nil {
				t.Fatal(err)
			}
			httpServer := httptest.NewServer(srv.mcpHandler())
			t.Cleanup(httpServer.Close)
			// post sends body; with unknownLength, without a Content-Length.
			post := func(body string, unknownLength bool) int {
				var r io.Reader = strings.NewReader(body)
				if unknownLength {
					r = struct{ io.Reader }{r}
				}
				req, err := http.NewRequest(http.MethodPost, httpServer.URL+testEndpoint, r)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()
				return resp.StatusCode
			}
			// ping returns a ping request of exactly n bytes.
			ping := func(n int) string {
				const prefix, suffix = `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"pad":"`, `"}}`
				return prefix + strings.Repeat("x", n-len(prefix)-len(suffix)) + suffix
			}

			for _, unknownLength := range []bool{false, true} {
				if got := post(ping(int(limit)+1), unknownLength); got != http.StatusRequestEntityTooLarge {
					t.Errorf("body one byte over the bound (unknown length %t): status %d, want 413", unknownLength, got)
				}
				if got := post(ping(int(limit)), unknownLength); got == http.StatusRequestEntityTooLarge {
					t.Errorf("body of exactly the bound (unknown length %t): status 413", unknownLength)
				}
			}
			if got := post(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`, false); got != http.StatusOK {
				t.Errorf("initialize: status %d, want 200", got)
			}
		})
	}
}

// A bound below 1 would switch it off over streamable HTTP, so it is refused
// there; stdio does not use it.
func TestNewServer_BodyBoundBelowOne(t *testing.T) {
	t.Setenv("FORWARDED_TOKEN_ISSUER_URL", "")
	t.Setenv("OAUTH_ISSUER_URL", "")
	for _, limit := range []int64{0, -1} {
		if _, err := NewServer(ServerConfig{Transport: transportStreamableHTTP, HTTPEndpoint: testEndpoint, MaxRequestBodyBytes: limit}); err == nil {
			t.Errorf("streamable-http accepted a body bound of %d", limit)
		}
		if _, err := NewServer(ServerConfig{Transport: transportStdio, MaxRequestBodyBytes: limit}); err != nil {
			t.Errorf("stdio refused a body bound of %d: %v", limit, err)
		}
	}
}

// testEndpoint is the MCP endpoint path the tests serve.
const testEndpoint = "/mcp"
