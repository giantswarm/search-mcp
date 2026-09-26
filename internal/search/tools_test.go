package search

import (
	"io"
	"log/slog"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/search-mcp/internal/metrics"
)

var intranetTools = []string{"search_runbook", "search_ops_recipe", "read_intranet_url"}

func registeredTools(t *testing.T, transport string) map[string]*server.ServerTool {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	RegisterTools(s, NewClient(logger), nil, transport, logger, metrics.NewNoopCollector())
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
