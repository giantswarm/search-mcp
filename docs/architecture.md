# Architecture

This document explains how the Giant Swarm Search MCP Server works internally.

## Overview

The MCP server is implemented in Go and provides a bridge between AI assistants and Giant Swarm's public documentation sources. It uses the Model Context Protocol (MCP) to expose search and content reading capabilities.

## Key Components

### 1. Search Endpoint

The server uses the public search endpoint:

- **Public endpoint**: `https://docs.giantswarm.io/searchapi/`
  - No authentication required
  - Provides access to public documentation sources (docs, blog, etc.)
  - Powered by OpenSearch v3.2

### 2. Package Structure

The server follows Go best practices with a modular structure:

```
search-mcp/
├── main.go           # Main entry point and CLI
├── internal/search/  # Core search functionality
│   ├── server.go     # MCP server setup and lifecycle
│   ├── tools.go      # MCP tool implementations
│   ├── client.go     # HTTP client for search API
│   ├── html.go       # HTML to Markdown conversion
│   └── types.go      # Data structures
├── internal/metrics/ # Prometheus metrics
│   ├── collector.go  # Collector interface
│   ├── prometheus.go # Prometheus implementation
│   └── noop.go       # No-op implementation (stdio mode)
```

#### Key Packages

- **main.go**: Entry point with command-line interface using Go's `flag` package
- **internal/search**: Core search functionality (not importable by external packages)
  - `Server`: MCP server lifecycle management
  - `Client`: HTTP client for search API and URL fetching
  - Tool handlers: Implement the 5 MCP tools (search, search_runbook, search_ops_recipe, read_handbook_url, read_intranet_url)

### 3. Search Backend

The search functionality is powered by OpenSearch v3.2:

- Uses OpenSearch query DSL (Elasticsearch-compatible) with `function_score`
- Two query modes, controlled by the `require_all_terms` parameter:
  - **Default (similarity mode):** Uses `multi_match` with `best_fields`, `operator: "or"`, and `minimum_should_match: "30%"`. Documents are ranked by relevance without requiring all terms to match. Best for longer, multi-term queries from AI agents.
  - **Strict AND mode (`require_all_terms: true`):** Uses `simple_query_string` with `default_operator: "AND"`. All search terms must be present in matching documents.
- Field weights: `title^5`, `body`, `text`
- Supports filters by type (e.g., "Blog", "Docs")
- Supports breadcrumb filtering for specific sections
- Implements scoring boosts for different content types
- Returns highlighted excerpts from matching content
- Compatible with both OpenSearch and Elasticsearch query syntax

### 4. Content Reading and Conversion

The `html.go` module handles URL fetching and conversion:

- Fetches HTML content from public URLs
- Uses `goquery` to parse and clean HTML (removes sidebars, scripts)
- Uses `html-to-markdown` library to convert to Markdown
- Cleans up excessive whitespace for readability
- Returns formatted Markdown with source URL

### 5. Transport Support

The server supports two MCP transports:

- **stdio**: Standard input/output for local integration (Cursor, Claude Desktop)
  - Uses `server.ServeStdio()` from mcp-go
  - Handles SIGTERM/SIGINT for graceful shutdown

- **streamable-http**: HTTP-based transport for network deployment
  - Uses `StreamableHTTPServer` from mcp-go
  - Configurable endpoint path (default: `/mcp`)
  - Graceful shutdown support

### 6. Metrics (HTTP mode only)

The server exposes Prometheus metrics at `/metrics` when running in HTTP mode:

| Metric | Type | Description |
|--------|------|-------------|
| `searchmcp_sse_connections_current` | Gauge | Current active SSE connections |
| `searchmcp_sse_connections_total` | Counter | Total connections since startup |
| `searchmcp_tool_calls_total{tool="..."}` | Counter | Tool invocations by name |

The metrics package uses an interface-driven design with a no-op implementation for stdio mode.

### 7. Error Handling

The server uses Go's standard error handling patterns:

- Returns descriptive error messages in MCP responses
- Uses structured logging with `log/slog`
- Provides context about failures (network, parsing, API errors)

## Request Flow

### Search Request

```
AI Assistant
    ↓
MCP Tool: search("kubernetes")
    ↓
tools.go: searchHandler()
    ↓
client.go: Client.Search()
    ↓
Build Elasticsearch query
    ↓
POST https://docs.giantswarm.io/searchapi/
    ↓
Parse JSON response
    ↓
client.go: FormatSearchResults()
    ↓
Return formatted Markdown results
```

### URL Reading Request

```
AI Assistant
    ↓
MCP Tool: read_handbook_url("https://handbook.giantswarm.io/...")
    ↓
tools.go: readHandbookURLHandler()
    ↓
Validate URL prefix
    ↓
client.go: Client.FetchURL()
    ↓
GET URL content
    ↓
html.go: ConvertHTMLToMarkdown()
    ↓
Parse with goquery, remove unwanted elements
    ↓
Convert to Markdown, clean whitespace
    ↓
Return formatted content
```

## Data Flow

1. **Client connects**
   - Stdio: Reads from stdin, writes to stdout
   - HTTP: Connects to configured endpoint

2. **MCP initialization**
   - Client sends initialize request
   - Server responds with capabilities and tool list
   - Server registers 5 tools with schemas

3. **Tool invocation**
   - Client sends tool call request
   - Server parses arguments using request.GetString(), request.GetInt(), etc.
   - Handler executes business logic
   - Results formatted as Markdown

4. **Search execution**
   - Build OpenSearch query with filters
   - POST to search API
   - Parse JSON response (handles both ES 6.x and OpenSearch 7.x+ formats)
   - Extract hits, highlights, metadata
   - Format as Markdown with titles, URLs, excerpts

5. **URL fetch and conversion**
   - Fetch HTML content
   - Parse with goquery
   - Remove navigation elements
   - Convert to Markdown
   - Clean up whitespace
   - Return formatted content

## Technology Stack

- **Language**: Go 1.23+
- **MCP Framework**: github.com/mark3labs/mcp-go
- **HTML Parsing**: github.com/PuerkitoBio/goquery
- **HTML to Markdown**: github.com/JohannesKaufmann/html-to-markdown/v2
- **Metrics**: github.com/prometheus/client_golang
- **Logging**: log/slog (Go standard library)
- **HTTP Client**: net/http (Go standard library)

## Security Considerations

- Public endpoint access only (no authentication implemented)
- No state storage or session management
- Standard Go HTTP client with reasonable timeouts
- See [Security](./security.md) for detailed information
