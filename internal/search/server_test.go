package search

import (
	"context"
	"net/http/httptest"
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

	srv, err := NewServer(ServerConfig{Transport: transportStreamableHTTP, HTTPEndpoint: "/mcp"})
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
		c, err := client.NewStreamableHttpClient(httpServer.URL+"/mcp", transport.WithHTTPHeaders(headers))
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
