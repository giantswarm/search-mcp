# Architecture

This document explains how the Giant Swarm Search MCP Server works internally.

## Overview

The MCP server provides a bridge between AI assistants and Giant Swarm's documentation sources, supporting both public and authenticated access to different content repositories.

## Key Components

### 1. Dual Endpoint Support

The server automatically selects the appropriate search endpoint based on authentication status:

- **Public endpoint**: `https://docs.giantswarm.io/searchapi/` (no authentication required)
  - Used when no authentication is configured
  - Provides access to public documentation sources (docs, blog, etc.)
  
- **Intranet endpoint**: `https://intranet.giantswarm.io/searchapi/` (requires authentication)
  - Used when authentication is available
  - Provides access to both public and internal intranet resources

### 2. Authentication Management

The `AuthManager` class handles authentication:

- **Environment Variable**: Reads `INTRANET_SESSION_COOKIE` from environment
- **Automatic Detection**: The `is_authenticated()` method checks if credentials are available
- **No State Storage**: Authentication is purely based on environment variables

#### Authentication Flow

```
User → Set INTRANET_SESSION_COOKIE env var
     → AuthManager.get_auth_headers() checks for credentials
     → If found: returns cookies for authenticated requests
     → If not found: returns empty dict (public access only)
```

### 3. Search Backend

The search functionality is powered by Elasticsearch v6.8.x:

- Uses Elasticsearch query DSL
- Supports filters by type (e.g., "Intranet", "Blog")
- Supports breadcrumb filtering for specific sections
- Implements scoring boosts for different content types

### 4. Content Reading

The `_read_url_content()` helper function:

- Works with both authenticated and public URLs
- Converts HTML to Markdown for better readability
- Cleans up navigation elements (sidebars, scripts)
- Handles error cases gracefully

### 5. Error Handling

The server provides context-aware error messages:

- Detects authentication failures vs network errors
- Differentiates between expired sessions and missing credentials
- Provides actionable guidance based on the error type

## Request Flow

### Search Request (Authenticated)

```
AI Assistant
    ↓
MCP Tool: search("kubernetes")
    ↓
AuthManager.is_authenticated() → true
    ↓
Use https://intranet.giantswarm.io/searchapi/
    ↓
Add authentication cookies
    ↓
POST Elasticsearch query
    ↓
Return formatted results (public + intranet content)
```

### Search Request (Unauthenticated)

```
AI Assistant
    ↓
MCP Tool: search("kubernetes")
    ↓
AuthManager.is_authenticated() → false
    ↓
Use https://docs.giantswarm.io/searchapi/
    ↓
POST Elasticsearch query (no auth)
    ↓
Return formatted results (public content only)
    ↓
Include note about limited results
```

## Data Flow

1. **User configures authentication** (optional)
   - Sets `INTRANET_SESSION_COOKIE` environment variable
   - Cookie is validated on first use

2. **Search query is received**
   - Server checks authentication status
   - Selects appropriate endpoint
   - Builds Elasticsearch query with filters

3. **Results are processed**
   - Elasticsearch returns hits with metadata
   - Server formats results as Markdown
   - Highlights and excerpts are included

4. **Content is returned**
   - Structured response with titles, URLs, types
   - Authentication status indicators
   - Pagination information

## Security Considerations

See [Security](./security.md) for detailed security information.

