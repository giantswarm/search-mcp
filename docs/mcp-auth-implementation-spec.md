# MCP Authorization Implementation Specification

## Document Information
- **Created**: 2026-01-13
- **Project**: Giant Swarm Search MCP Server
- **Feature**: OAuth 2.1 Authentication for Intranet Access
- **Version**: Phase 1 (HTTP OAuth Flow)

## Executive Summary

This specification defines the implementation of OAuth 2.1-based authentication for the Giant Swarm Search MCP Server using the `github.com/giantswarm/mcp-oauth` library. The goal is to enable authenticated requests to the Giant Swarm intranet on behalf of the agent end user.

**Phase 1 Scope**: HTTP transport OAuth flow only. Stdio mode will return "not implemented" errors for intranet tools.

## 1. Requirements Overview

### 1.1 Functional Requirements

| Requirement | Description | Priority |
|------------|-------------|----------|
| FR-1 | Implement OAuth 2.1 authentication using mcp-oauth library | P0 |
| FR-2 | Support Dex identity provider | P0 |
| FR-3 | Authenticate only intranet-specific tools | P0 |
| FR-4 | Encrypted token storage in filesystem | P0 |
| FR-5 | Proactive token refresh before expiration | P1 |
| FR-6 | Auto-generate OAuth redirect URIs from server address | P1 |
| FR-7 | Clear error messages with guidance | P1 |
| FR-8 | Lock-based token refresh for concurrent requests | P2 |
| FR-9 | Auto-recovery from corrupted token files | P2 |

### 1.2 Non-Functional Requirements

| Requirement | Description | Target |
|------------|-------------|--------|
| NFR-1 | Auth flow completion rate | >90% |
| NFR-2 | Token encryption | AES-256-GCM |
| NFR-3 | Token storage location | XDG Base Directory compliant |
| NFR-4 | Minimal logging | Auth events + token operations only |
| NFR-5 | Single-user mode | One auth session per server instance |
| NFR-6 | Backward compatibility | Not required (v0.x version) |

## 2. Architecture

### 2.1 Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     MCP Server                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Server (internal/search/server.go)                  │  │
│  │  - Existing server logic                             │  │
│  │  - Transport handlers (stdio/HTTP)                   │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Auth Module (NEW: internal/auth/)                   │  │
│  │  ┌────────────────┐  ┌────────────────┐             │  │
│  │  │ OAuth Client   │  │ Token Manager  │             │  │
│  │  │ (mcp-oauth)    │  │ (storage)      │             │  │
│  │  └────────────────┘  └────────────────┘             │  │
│  │  ┌────────────────┐  ┌────────────────┐             │  │
│  │  │ HTTP Handler   │  │ Middleware     │             │  │
│  │  │ (callbacks)    │  │ (inject auth)  │             │  │
│  │  └────────────────┘  └────────────────┘             │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Tools (internal/search/tools.go)                    │  │
│  │  - read_intranet_url (requires auth)                 │  │
│  │  - search_runbook (requires auth)                    │  │
│  │  - search_ops_recipe (requires auth)                 │  │
│  │  - search (public - no auth)                         │  │
│  │  - read_handbook_url (public - no auth)              │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Client (internal/search/client.go)                  │  │
│  │  - Modified to inject Bearer tokens                  │  │
│  │  - Domain replacement for intranet                   │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
         ┌──────────────────────────────────┐
         │  Token Storage                   │
         │  ~/.config/giantswarm/           │
         │  tokens.enc (AES-256-GCM)        │
         └──────────────────────────────────┘
```

### 2.2 Authentication Flow (HTTP Mode)

```
┌───────┐          ┌──────────┐         ┌──────────┐         ┌─────────────┐
│ User  │          │   MCP    │         │   Dex    │         │  Intranet   │
│(Agent)│          │  Server  │         │  OAuth   │         │             │
└───┬───┘          └────┬─────┘         └────┬─────┘         └──────┬──────┘
    │                   │                    │                       │
    │ 1. Call intranet │                    │                       │
    │    tool           │                    │                       │
    ├──────────────────>│                    │                       │
    │                   │                    │                       │
    │                   │ 2. Check token    │                       │
    │                   │    (not found)     │                       │
    │                   │                    │                       │
    │                   │ 3. Initiate OAuth  │                       │
    │                   ├───────────────────>│                       │
    │                   │                    │                       │
    │ 4. Auth URL       │                    │                       │
    │ <─────────────────┤                    │                       │
    │   (via MCP        │                    │                       │
    │    notification)  │                    │                       │
    │                   │                    │                       │
    │ 5. Open browser   │                    │                       │
    │    & authenticate │                    │                       │
    ├────────────────────────────────────────>│                       │
    │                   │                    │                       │
    │                   │  6. OAuth callback │                       │
    │                   │<───────────────────┤                       │
    │                   │                    │                       │
    │                   │ 7. Exchange code   │                       │
    │                   │    for tokens      │                       │
    │                   ├───────────────────>│                       │
    │                   │<───────────────────┤                       │
    │                   │                    │                       │
    │                   │ 8. Store encrypted │                       │
    │                   │    tokens          │                       │
    │                   │    (~/.config/)    │                       │
    │                   │                    │                       │
    │                   │ 9. Retry request   │                       │
    │                   │    with Bearer     │                       │
    │                   │    token           │                       │
    │                   ├────────────────────────────────────────────>│
    │                   │                    │                       │
    │                   │                    │   10. Return content  │
    │ 11. Success       │<────────────────────────────────────────────┤
    │ <─────────────────┤                    │                       │
```

### 2.3 Token Refresh Flow

```
┌──────────┐              ┌──────────┐              ┌──────────┐
│   Tool   │              │  Token   │              │   Dex    │
│  Handler │              │ Manager  │              │  OAuth   │
└────┬─────┘              └────┬─────┘              └────┬─────┘
     │                         │                         │
     │ 1. Request with auth    │                         │
     ├────────────────────────>│                         │
     │                         │                         │
     │                         │ 2. Check expiration     │
     │                         │    (expires in <20%)    │
     │                         │                         │
     │                         │ 3. Acquire refresh lock │
     │                         │                         │
     │                         │ 4. Refresh token        │
     │                         ├────────────────────────>│
     │                         │                         │
     │                         │ 5. New tokens           │
     │                         │<────────────────────────┤
     │                         │                         │
     │                         │ 6. Store encrypted      │
     │                         │                         │
     │                         │ 7. Release lock         │
     │                         │                         │
     │ 8. Return fresh token   │                         │
     │<────────────────────────┤                         │
```

## 3. Configuration

### 3.1 Environment Variables

| Variable | Description | Required | Default | Example |
|----------|-------------|----------|---------|---------|
| `OAUTH_ISSUER_URL` | Dex OAuth issuer URL | Yes* | - | `https://dex.operations.awsprod.gigantic.io` |
| `OAUTH_CLIENT_ID` | OAuth client identifier | Yes* | - | `searchmcp` |
| `OAUTH_CLIENT_SECRET` | OAuth client secret | Yes* | - | `dummysecret` |
| `OAUTH_REDIRECT_URI` | OAuth callback URL (optional) | No | Auto-generated | `http://localhost:8080/oauth/callback` |

\* Required only when using intranet tools. Server starts without these but fails gracefully on intranet tool usage.

### 3.2 Dex Configuration

**Production Instance:**
- **Issuer URL**: `https://dex.operations.awsprod.gigantic.io`
- **Client ID**: `searchmcp`
- **Client Secret**: `dummysecret`
- **Scopes**: `openid`, `offline_access`
- **Grant Types**: `authorization_code`, `refresh_token`
- **PKCE**: Required
- **Redirect URI**: Auto-generated from `--http-addr` flag
  - Format: `http://{host}:{port}/callback`
  - Example: `http://localhost:8080/callback`

### 3.3 Token Storage

**Location**: XDG Base Directory compliant
- **Linux**: `~/.config/giantswarm/tokens.enc`
- **macOS**: `~/Library/Application Support/giantswarm/tokens.enc`
- **Windows**: `%APPDATA%\giantswarm\tokens.enc`

**Encryption**:
- Algorithm: AES-256-GCM
- Key derivation: Machine ID based (using hardware identifiers)
- File permissions: 0600 (owner read/write only)

**Token Data Structure**:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expiry": "2026-01-14T10:30:00Z",
  "id_token": "eyJhbGc..."
}
```

## 4. Implementation Details

### 4.1 New Packages/Files

```
internal/
  auth/
    auth.go           # Main auth module interface
    oauth.go          # OAuth client implementation (mcp-oauth wrapper)
    token_storage.go  # Encrypted file-based token storage
    token_manager.go  # Token lifecycle management (refresh, expiry checks)
    middleware.go     # HTTP middleware for auth injection
    encryption.go     # AES-256-GCM encryption utilities
    machine_id.go     # Machine ID derivation for encryption key
```

### 4.2 Modified Files

```
internal/search/
  server.go           # Initialize auth module, add OAuth routes
  client.go           # Inject Bearer tokens, domain replacement
  tools.go            # Add auth checks to intranet tools

cmd/
  serve.go            # Add auth-related flags (future use)

docs/
  README.md           # Add authentication section
  authentication.md   # New: detailed auth guide
  security.md         # Update with auth security model
```

### 4.3 Core Interfaces

#### 4.3.1 Auth Module Interface

```go
// internal/auth/auth.go
package auth

type AuthManager interface {
    // GetToken returns a valid access token, refreshing if necessary
    GetToken(ctx context.Context) (string, error)

    // IsAuthenticated checks if user has valid credentials
    IsAuthenticated() bool

    // InitiateAuth starts OAuth flow, returns auth URL for user
    InitiateAuth(ctx context.Context) (authURL string, err error)

    // HandleCallback processes OAuth callback and stores tokens
    HandleCallback(ctx context.Context, code string, state string) error

    // ClearTokens removes stored tokens (logout)
    ClearTokens() error
}
```

#### 4.3.2 Token Storage Interface

```go
// internal/auth/token_storage.go
package auth

type TokenStorage interface {
    // Store saves tokens with encryption
    Store(ctx context.Context, tokens *TokenData) error

    // Load retrieves and decrypts tokens
    Load(ctx context.Context) (*TokenData, error)

    // Delete removes token file
    Delete() error

    // Exists checks if token file exists
    Exists() bool
}

type TokenData struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    Expiry       time.Time `json:"expiry"`
    IDToken      string    `json:"id_token,omitempty"`
}
```

### 4.4 Tool Authentication Logic

#### 4.4.1 Tools Requiring Authentication

Update `internal/search/tools.go`:

```go
// Tools that require authentication
var authRequiredTools = map[string]bool{
    "read_intranet_url": true,
    "search_runbook":    true,
    "search_ops_recipe": true,
}

// Modified handler wrapper
func requireAuth(handler server.ToolHandlerFunc, authMgr auth.AuthManager) server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Check if tool requires auth
        if !authRequiredTools[request.Name] {
            return handler(ctx, request)
        }

        // Check authentication
        if !authMgr.IsAuthenticated() {
            return mcp.NewToolResultError(
                "Authentication required. This tool accesses Giant Swarm intranet which requires authentication.\n" +
                "Please ensure OAUTH_ISSUER_URL, OAUTH_CLIENT_ID, and OAUTH_CLIENT_SECRET are configured.\n" +
                "Run with --transport=streamable-http to enable authentication.",
            ), nil
        }

        // Get valid token (refreshes if needed)
        token, err := authMgr.GetToken(ctx)
        if err != nil {
            return mcp.NewToolResultError(
                fmt.Sprintf("Authentication failed: %v\n"+
                "Your session may have expired. Please restart the server to re-authenticate.", err),
            ), nil
        }

        // Inject token into context for client to use
        ctx = context.WithValue(ctx, "auth_token", token)

        return handler(ctx, request)
    }
}
```

### 4.5 HTTP Client Modifications

#### 4.5.1 Bearer Token Injection

Update `internal/search/client.go`:

```go
// FetchURL with authentication support
func (c *Client) FetchURL(ctx context.Context, url string) (string, error) {
    // Check if URL requires authentication (intranet)
    requiresAuth := strings.HasPrefix(url, "https://intranet.giantswarm.io/")

    // IMPORTANT: Domain replacement for JWT authentication
    if requiresAuth {
        url = strings.Replace(url, "intranet.giantswarm.io", "intranet-searchmcp.giantswarm.io", 1)
    }

    client := &http.Client{
        Timeout: fetchTimeout,
    }

    httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }

    // Inject Bearer token if available in context
    if requiresAuth {
        if token, ok := ctx.Value("auth_token").(string); ok && token != "" {
            httpReq.Header.Set("Authorization", "Bearer "+token)
        }
    }

    resp, err := client.Do(httpReq)
    if err != nil {
        return "", fmt.Errorf("failed to fetch URL: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return "", fmt.Errorf("authentication failed (401 Unauthorized)")
    }

    if resp.StatusCode == http.StatusForbidden {
        return "", fmt.Errorf("access denied (403 Forbidden)")
    }

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response: %w", err)
    }

    return string(body), nil
}
```

### 4.6 Server Initialization

Update `internal/search/server.go`:

```go
type Server struct {
    mcpServer *server.MCPServer
    client    *Client
    authMgr   auth.AuthManager  // NEW
    logger    *slog.Logger
    config    ServerConfig
}

func NewServer(config ServerConfig) (*Server, error) {
    // ... existing logger setup ...

    // Initialize auth manager (only if in HTTP mode)
    var authMgr auth.AuthManager
    if config.Transport == "streamable-http" {
        authConfig := auth.Config{
            IssuerURL:    os.Getenv("OAUTH_ISSUER_URL"),
            ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
            ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
            Scopes:       []string{"openid", "offline_access"},
            RedirectURI:  "", // Auto-generated in auth module
        }

        var err error
        authMgr, err = auth.NewManager(authConfig, logger)
        if err != nil {
            // Non-fatal: auth is optional if not using intranet tools
            logger.Warn("auth manager initialization failed", "error", err,
                "note", "Intranet tools will not be available")
        }
    }

    // ... rest of initialization ...

    return &Server{
        mcpServer: mcpServer,
        client:    searchClient,
        authMgr:   authMgr,
        logger:    logger,
        config:    config,
    }, nil
}
```

### 4.7 OAuth Routes (HTTP Mode)

Add to `internal/search/server.go`:

```go
func (s *Server) startHTTP(ctx context.Context) error {
    s.logger.Info("starting MCP server", "transport", "streamable-http",
        "addr", s.config.HTTPAddr, "endpoint", s.config.HTTPEndpoint)

    // Create StreamableHTTPServer
    httpServer := server.NewStreamableHTTPServer(
        s.mcpServer,
        server.WithEndpointPath(s.config.HTTPEndpoint),
    )

    // Add OAuth routes if auth is enabled
    if s.authMgr != nil {
        // OAuth initiation endpoint
        http.HandleFunc("/oauth/login", func(w http.ResponseWriter, r *http.Request) {
            authURL, err := s.authMgr.InitiateAuth(r.Context())
            if err != nil {
                http.Error(w, fmt.Sprintf("Auth initiation failed: %v", err),
                    http.StatusInternalServerError)
                return
            }
            http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
        })

        // OAuth callback endpoint
        http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
            code := r.URL.Query().Get("code")
            state := r.URL.Query().Get("state")

            if code == "" {
                http.Error(w, "Missing authorization code", http.StatusBadRequest)
                return
            }

            err := s.authMgr.HandleCallback(r.Context(), code, state)
            if err != nil {
                http.Error(w, fmt.Sprintf("Auth callback failed: %v", err),
                    http.StatusInternalServerError)
                return
            }

            w.Header().Set("Content-Type", "text/html")
            fmt.Fprintf(w, `
                <html>
                <body>
                    <h1>Authentication Successful</h1>
                    <p>You have successfully authenticated with Giant Swarm.</p>
                    <p>You can now close this window and return to your AI assistant.</p>
                </body>
                </html>
            `)
        })

        s.logger.Info("OAuth endpoints registered",
            "login", "/oauth/login", "callback", "/oauth/callback")
    }

    // ... existing HTTP server startup ...
}
```

### 4.8 Token Refresh Strategy

Implement proactive token refresh in `internal/auth/token_manager.go`:

```go
// GetToken returns a valid access token, refreshing if near expiration
func (m *Manager) GetToken(ctx context.Context) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Load current tokens
    tokens, err := m.storage.Load(ctx)
    if err != nil {
        return "", fmt.Errorf("failed to load tokens: %w", err)
    }

    // Check if refresh is needed (expires in <20% of lifetime)
    now := time.Now()
    timeUntilExpiry := tokens.Expiry.Sub(now)

    // Assume 1 hour token lifetime, refresh if <12 minutes remaining
    refreshThreshold := 12 * time.Minute

    if timeUntilExpiry < refreshThreshold {
        m.logger.Debug("token nearing expiration, refreshing",
            "time_until_expiry", timeUntilExpiry)

        // Refresh token
        newTokens, err := m.refreshToken(ctx, tokens.RefreshToken)
        if err != nil {
            return "", fmt.Errorf("token refresh failed: %w", err)
        }

        // Store new tokens
        if err := m.storage.Store(ctx, newTokens); err != nil {
            return "", fmt.Errorf("failed to store refreshed tokens: %w", err)
        }

        tokens = newTokens
        m.logger.Info("token refreshed successfully")
    }

    return tokens.AccessToken, nil
}
```

### 4.9 Encryption Key Derivation

Implement in `internal/auth/machine_id.go`:

```go
package auth

import (
    "crypto/sha256"
    "fmt"
    "os"
    "runtime"
)

// DeriveEncryptionKey generates a 32-byte encryption key from machine identifiers
func DeriveEncryptionKey() ([]byte, error) {
    var identifiers []string

    switch runtime.GOOS {
    case "darwin":
        // macOS: use IOPlatformUUID
        data, err := os.ReadFile("/sys/class/dmi/id/product_uuid")
        if err == nil {
            identifiers = append(identifiers, string(data))
        }

    case "linux":
        // Linux: use machine-id
        data, err := os.ReadFile("/etc/machine-id")
        if err == nil {
            identifiers = append(identifiers, string(data))
        }

    case "windows":
        // Windows: use MachineGUID from registry
        // Implementation would use windows registry APIs
    }

    // Fallback: use hostname
    hostname, err := os.Hostname()
    if err == nil {
        identifiers = append(identifiers, hostname)
    }

    // Combine all identifiers
    combined := ""
    for _, id := range identifiers {
        combined += id
    }

    if combined == "" {
        return nil, fmt.Errorf("failed to derive encryption key: no machine identifiers found")
    }

    // Hash to get 32-byte key
    hash := sha256.Sum256([]byte(combined))
    return hash[:], nil
}
```

### 4.10 Error Handling

Comprehensive error messages in `internal/auth/oauth.go`:

```go
func (m *Manager) GetToken(ctx context.Context) (string, error) {
    // ... token retrieval logic ...

    if err != nil {
        // Provide detailed guidance based on error type
        switch {
        case errors.Is(err, ErrTokenNotFound):
            return "", fmt.Errorf(
                "not authenticated: no valid session found\n" +
                "Please authenticate by visiting: %s/oauth/login\n" +
                "Or ensure environment variables are set: OAUTH_ISSUER_URL, OAUTH_CLIENT_ID, OAUTH_CLIENT_SECRET",
                m.config.ServerAddr,
            )

        case errors.Is(err, ErrTokenExpired):
            return "", fmt.Errorf(
                "authentication expired: your session has timed out\n" +
                "Please re-authenticate by visiting: %s/oauth/login",
                m.config.ServerAddr,
            )

        case errors.Is(err, ErrRefreshFailed):
            return "", fmt.Errorf(
                "token refresh failed: %w\n" +
                "This may indicate revoked access or network issues\n" +
                "Try re-authenticating at: %s/oauth/login",
                err, m.config.ServerAddr,
            )

        default:
            return "", fmt.Errorf("authentication error: %w", err)
        }
    }

    return token, nil
}
```

## 5. Phase 1: Stdio Mode Behavior

### 5.1 Stdio Mode Handling

In Phase 1 (HTTP OAuth only), stdio mode will not support authentication:

```go
// Update tool handlers to check transport mode
func requireAuth(handler server.ToolHandlerFunc, authMgr auth.AuthManager,
                 transport string) server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        if !authRequiredTools[request.Name] {
            return handler(ctx, request)
        }

        // Phase 1: Stdio mode not supported
        if transport == "stdio" {
            return mcp.NewToolResultError(
                "Authentication not available in stdio mode (Phase 1)\n" +
                "Intranet tools require authentication which is only available in HTTP mode.\n" +
                "Please run the server with: --transport=streamable-http --http-addr=:8080\n" +
                "Then authenticate at: http://localhost:8080/oauth/login",
            ), nil
        }

        // ... existing auth checks for HTTP mode ...
    }
}
```

## 6. Testing Strategy

### 6.1 Unit Tests

**Priority: P0** - Must implement

Test coverage for:

1. **Token Storage Tests** (`internal/auth/token_storage_test.go`)
   - Encryption/decryption correctness
   - File creation with correct permissions (0600)
   - XDG directory path resolution (Linux/macOS/Windows)
   - Corrupted file handling (delete and re-auth)

2. **Token Manager Tests** (`internal/auth/token_manager_test.go`)
   - Token refresh trigger logic (<20% lifetime remaining)
   - Proactive refresh before expiration
   - Refresh lock mechanism for concurrent requests
   - Token expiry calculations

3. **Encryption Tests** (`internal/auth/encryption_test.go`)
   - AES-256-GCM encryption/decryption
   - Key derivation from machine ID
   - Error handling for invalid keys

4. **Client Auth Injection Tests** (`internal/search/client_test.go`)
   - Bearer token injection into HTTP headers
   - Domain replacement (intranet.giantswarm.io → intranet-searchmcp.giantswarm.io)
   - 401/403 error handling

5. **Tool Auth Checks Tests** (`internal/search/tools_test.go`)
   - Auth-required vs public tools
   - Error messages when auth not configured
   - Stdio mode "not implemented" errors

### 6.2 Manual Testing Checklist

**Test Environment**:
- Dex instance: `https://dex.operations.awsprod.gigantic.io`
- Test URL: `https://intranet.giantswarm.io/docs/` (replace domain to `intranet-searchmcp.giantswarm.io`)

**Test Cases**:

| # | Test Case | Expected Result | Pass/Fail |
|---|-----------|----------------|-----------|
| 1 | Start server without env vars, call `read_intranet_url` | Clear error about missing config | |
| 2 | Start server with valid env vars in HTTP mode | Server starts, OAuth routes registered | |
| 3 | Visit `/oauth/login`, authenticate with Dex | Redirect to Dex, successful auth | |
| 4 | After auth, call `read_intranet_url` | Returns intranet content | |
| 5 | Check token file exists at `~/.config/giantswarm/tokens.enc` | File exists with 0600 permissions | |
| 6 | Restart server, call `read_intranet_url` | Uses cached tokens, no re-auth | |
| 7 | Delete token file, call `read_intranet_url` | Error prompts to re-authenticate | |
| 8 | Wait for token to approach expiration, make request | Token auto-refreshes | |
| 9 | Corrupt token file, call `read_intranet_url` | File deleted, prompted to re-auth | |
| 10 | Start in stdio mode, call `read_intranet_url` | "Not implemented" error | |
| 11 | Call public tool `search` without auth | Works without authentication | |
| 12 | Call public tool `read_handbook_url` | Works without authentication | |

## 7. Documentation

### 7.1 README Updates

Add to `README.md`:

```markdown
## Authentication

The MCP server supports OAuth 2.1 authentication for accessing Giant Swarm's internal resources (intranet, runbooks, ops recipes).

### Quick Start

1. **Set environment variables**:
   ```bash
   export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
   export OAUTH_CLIENT_ID=searchmcp
   export OAUTH_CLIENT_SECRET=dummysecret
   ```

2. **Run in HTTP mode** (authentication requires HTTP transport):
   ```bash
   search-mcp serve --transport=streamable-http --http-addr=:8080
   ```

3. **Authenticate**:
   - Visit http://localhost:8080/oauth/login
   - Sign in with your Giant Swarm credentials
   - You're authenticated!

### What Requires Authentication?

- ✅ **Public tools** (no auth required):
  - `search` - Search public documentation
  - `read_handbook_url` - Read public handbook pages

- 🔒 **Intranet tools** (authentication required):
  - `read_intranet_url` - Read intranet pages
  - `search_runbook` - Search DevOps runbooks
  - `search_ops_recipe` - Search ops recipes

### Token Storage

Tokens are encrypted and stored at:
- **Linux**: `~/.config/giantswarm/tokens.enc`
- **macOS**: `~/Library/Application Support/giantswarm/tokens.enc`
- **Windows**: `%APPDATA%\giantswarm\tokens.enc`

Tokens are automatically refreshed before expiration.

For more details, see [Authentication Guide](docs/authentication.md).
```

### 7.2 Dedicated Authentication Guide

Create `docs/authentication.md`:

```markdown
# Authentication Guide

This guide covers OAuth 2.1 authentication for the Giant Swarm Search MCP Server.

## Overview

The server uses OAuth 2.1 with PKCE to authenticate users against Giant Swarm's Dex identity provider. This enables secure access to internal resources like the intranet, runbooks, and ops recipes.

## Prerequisites

- OAuth client credentials (client ID and secret)
- Access to Giant Swarm's Dex instance
- Server running in HTTP mode (stdio mode not yet supported)

## Configuration

### Environment Variables

| Variable | Description | Required | Example |
|----------|-------------|----------|---------|
| `OAUTH_ISSUER_URL` | Dex OAuth issuer URL | Yes | `https://dex.operations.awsprod.gigantic.io` |
| `OAUTH_CLIENT_ID` | OAuth client identifier | Yes | `searchmcp` |
| `OAUTH_CLIENT_SECRET` | OAuth client secret | Yes | `dummysecret` |

### Example Configuration

```bash
export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
export OAUTH_CLIENT_ID=searchmcp
export OAUTH_CLIENT_SECRET=dummysecret

search-mcp serve --transport=streamable-http --http-addr=:8080
```

## Authentication Flow

### First Time Authentication

1. **Start the server** in HTTP mode
2. **Trigger authentication** by:
   - Visiting http://localhost:8080/oauth/login in your browser, OR
   - Calling an intranet tool (you'll get an error with the login URL)
3. **Sign in** with your Giant Swarm credentials via Dex
4. **Return to the app** - you're now authenticated!

### Subsequent Sessions

Tokens are cached locally and automatically refreshed:
- Tokens stored encrypted at `~/.config/giantswarm/tokens.enc`
- Automatically refreshed before expiration
- Survive server restarts

## Token Management

### Token Lifecycle

```
User Auth → Access Token (1hr) → Proactive Refresh (at 48min) → New Token → ...
            └─ Refresh Token (30d) ─→ Used for refresh ────────┘
```

### Token Storage

**Location**: Platform-specific config directory
- Linux: `~/.config/giantswarm/tokens.enc`
- macOS: `~/Library/Application Support/giantswarm/tokens.enc`
- Windows: `%APPDATA%\giantswarm\tokens.enc`

**Encryption**: AES-256-GCM with machine-derived key

**Permissions**: File is created with 0600 (owner read/write only)

### Manual Token Management

```bash
# View token file location
ls -la ~/.config/giantswarm/

# Delete tokens (force re-authentication)
rm ~/.config/giantswarm/tokens.enc

# Check token file permissions
ls -l ~/.config/giantswarm/tokens.enc
# Should show: -rw------- (0600)
```

## Troubleshooting

### "Authentication required" error

**Symptom**: Error when calling intranet tools

**Solutions**:
1. Check environment variables are set: `echo $OAUTH_ISSUER_URL`
2. Ensure running in HTTP mode: `--transport=streamable-http`
3. Authenticate at: http://localhost:8080/oauth/login

### "Authentication failed (401)" error

**Symptom**: Token rejected by intranet

**Solutions**:
1. Delete cached tokens: `rm ~/.config/giantswarm/tokens.enc`
2. Re-authenticate at: http://localhost:8080/oauth/login
3. Check Dex is accessible: `curl https://dex.operations.awsprod.gigantic.io/.well-known/openid-configuration`

### Token refresh fails

**Symptom**: Errors about expired refresh token

**Cause**: Refresh tokens expire after 30 days of inactivity

**Solution**: Delete tokens and re-authenticate

### Corrupted token file

**Symptom**: Decryption errors on startup

**Behavior**: Server automatically deletes corrupted file and prompts re-auth

**Manual Fix**: Delete token file: `rm ~/.config/giantswarm/tokens.enc`

## Security Considerations

### Token Security

- ✅ Tokens encrypted at rest (AES-256-GCM)
- ✅ File permissions restricted (0600)
- ✅ Machine-specific encryption key
- ✅ Tokens never logged
- ✅ HTTPS for all OAuth communication

### Best Practices

1. **Protect your client secret**: Don't commit `OAUTH_CLIENT_SECRET` to git
2. **Use environment variables**: Don't hardcode credentials
3. **Secure your machine**: Token encryption relies on machine security
4. **Regular token rotation**: Re-authenticate periodically
5. **Monitor access**: Check Dex logs for unauthorized access

## Phase 1 Limitations

**Current Phase**: HTTP OAuth flow only

**Not Yet Supported**:
- ❌ Stdio mode authentication (device flow)
- ❌ CIMD (Client ID Metadata Documents)
- ❌ Multi-user sessions in HTTP mode
- ❌ Custom token lifetimes
- ❌ Rate limiting
- ❌ Observability metrics (Prometheus/OTLP)

These features are planned for future phases.

## Advanced Configuration

### Custom Redirect URI

By default, redirect URI is auto-generated from `--http-addr`:
```
http://localhost:8080/callback
```

For custom deployments (reverse proxies, public URLs), OAuth client must be registered with the correct redirect URI.

### Token Refresh Threshold

Tokens are proactively refreshed when <20% of lifetime remains:
- Default token lifetime: 1 hour
- Refresh threshold: 12 minutes before expiration

This behavior is hardcoded in Phase 1.

## Support

For issues or questions:
- GitHub Issues: https://github.com/giantswarm/search-mcp/issues
- Internal Slack: #search-mcp
```

### 7.3 Environment Variable Reference

Add to `docs/authentication.md` or create separate file:

```markdown
## Environment Variable Reference

### Required for Authentication

#### OAUTH_ISSUER_URL
- **Description**: OAuth 2.0 issuer URL for Dex
- **Format**: `https://` URL
- **Example**: `https://dex.operations.awsprod.gigantic.io`
- **Required**: Yes (for intranet tools)
- **Default**: None

#### OAUTH_CLIENT_ID
- **Description**: OAuth client identifier registered in Dex
- **Format**: String
- **Example**: `searchmcp`
- **Required**: Yes (for intranet tools)
- **Default**: None

#### OAUTH_CLIENT_SECRET
- **Description**: OAuth client secret for authentication
- **Format**: String (sensitive)
- **Example**: `dummysecret`
- **Required**: Yes (for intranet tools)
- **Default**: None
- **Security**: Keep this secret! Do not commit to version control

### Optional

#### OAUTH_REDIRECT_URI
- **Description**: OAuth callback URL (usually auto-generated)
- **Format**: `http://` or `https://` URL ending in `/callback`
- **Example**: `http://localhost:8080/callback`
- **Required**: No (auto-generated from `--http-addr`)
- **Default**: `http://localhost:{port}/callback`
- **Note**: Must match registered redirect URI in Dex client config
```

## 8. Implementation Phases

### Phase 1: HTTP OAuth Flow (Current Spec)

**Deliverables**:
- ✅ OAuth 2.1 authentication with Dex
- ✅ Encrypted token storage (AES-256-GCM)
- ✅ Proactive token refresh
- ✅ HTTP mode authentication
- ✅ Bearer token injection
- ✅ Domain replacement for intranet
- ✅ Auth-required tool checks
- ✅ Stdio mode graceful failure
- ✅ Unit tests
- ✅ Documentation (README, auth guide, env var reference)

**Timeline**: Initial implementation

### Phase 2: Stdio Device Flow (Future)

**Deliverables**:
- Device flow implementation for stdio mode
- MCP notification for auth URL
- Browser auto-open (optional)
- Remove stdio mode restrictions

**Timeline**: Future release

### Phase 3: Advanced Features (Future)

**Deliverables**:
- CIMD (Client ID Metadata Documents) support
- Observability (Prometheus metrics, OTLP tracing)
- Rate limiting (per-IP, per-user)
- Multi-user sessions in HTTP mode
- Configurable token lifetimes

**Timeline**: As needed based on user feedback

## 9. Success Criteria

### 9.1 Functional Success

- [ ] Users can authenticate via OAuth in HTTP mode
- [ ] Intranet tools successfully fetch authenticated content
- [ ] Tokens persist across server restarts
- [ ] Tokens auto-refresh without user intervention
- [ ] Public tools work without authentication
- [ ] Clear error messages guide users to fix issues

### 9.2 Performance Metrics

- **Auth Flow Completion Rate**: >90% of auth attempts succeed
- **Token Refresh Success Rate**: >99% of refreshes succeed
- **Average Auth Time**: <10 seconds (depends on Dex)

### 9.3 Security Metrics

- **Zero Token Leaks**: No tokens in logs, error messages, or stdout
- **File Permissions**: Token file always 0600
- **Encryption Validation**: All stored tokens encrypted with AES-256-GCM

## 10. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Dex unavailable | Low | High | Clear error messages, retry with backoff |
| Token file corruption | Low | Medium | Auto-delete and re-auth |
| Encryption key instability | Low | Medium | Document machine ID requirements |
| OAuth misconfiguration | Medium | High | Detailed setup documentation, validation checks |
| Token refresh failure | Low | High | Graceful fallback, prompt re-auth |
| Domain replacement breaks | Low | High | Unit tests for domain replacement logic |

## 11. Dependencies

### 11.1 External Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/giantswarm/mcp-oauth` | Latest | OAuth 2.1 client library |
| `github.com/mark3labs/mcp-go` | v0.43.2 | MCP server framework (existing) |
| `golang.org/x/crypto` | Latest | AES-256-GCM encryption |
| `golang.org/x/oauth2` | Latest | OAuth 2.0 primitives (via mcp-oauth) |

### 11.2 System Dependencies

- **Go**: 1.25.5 or later
- **Dex**: Production instance at `https://dex.operations.awsprod.gigantic.io`
- **Network**: Access to Dex issuer URL
- **Filesystem**: Write access to config directory

## 12. Open Questions

### Resolved
- ✅ Which mcp-auth library? → `github.com/giantswarm/mcp-oauth`
- ✅ Auth flow for stdio mode? → Phase 2 (device flow)
- ✅ IdP provider? → Dex
- ✅ Token storage location? → XDG Base Dir
- ✅ Encryption key management? → Machine ID derived
- ✅ OAuth scopes? → `openid`, `offline_access`
- ✅ Multi-user support? → Single-user only
- ✅ Token refresh strategy? → Proactive (at 80% lifetime)
- ✅ Error handling? → Detailed with guidance
- ✅ Testing approach? → Unit tests
- ✅ Documentation scope? → README, auth guide, env var ref

### To Be Determined During Implementation
- ⏳ Exact machine ID derivation on Windows
- ⏳ Token lifetime from Dex (assumed 1 hour)
- ⏳ Refresh token lifetime from Dex (assumed 30 days)
- ⏳ mcp-oauth library API details (may differ from assumptions)

## 13. Implementation Checklist

### Phase 1: HTTP OAuth

#### Core Auth Module
- [ ] Create `internal/auth/` package structure
- [ ] Implement `auth.go` - AuthManager interface
- [ ] Implement `oauth.go` - Dex OAuth client using mcp-oauth
- [ ] Implement `token_storage.go` - Encrypted file storage
- [ ] Implement `token_manager.go` - Token lifecycle management
- [ ] Implement `encryption.go` - AES-256-GCM utilities
- [ ] Implement `machine_id.go` - Machine ID derivation

#### Server Integration
- [ ] Update `internal/search/server.go` - Initialize auth manager
- [ ] Add OAuth routes (`/oauth/login`, `/oauth/callback`)
- [ ] Update `internal/search/tools.go` - Add auth checks
- [ ] Update `internal/search/client.go` - Bearer token injection
- [ ] Implement domain replacement (intranet.giantswarm.io → intranet-searchmcp.giantswarm.io)

#### Error Handling
- [ ] Implement detailed error messages with guidance
- [ ] Add stdio mode "not implemented" errors
- [ ] Handle missing configuration gracefully

#### Testing
- [ ] Write unit tests for token storage
- [ ] Write unit tests for token manager
- [ ] Write unit tests for encryption
- [ ] Write unit tests for client auth injection
- [ ] Write unit tests for tool auth checks
- [ ] Manual testing against production Dex

#### Documentation
- [ ] Update `README.md` with authentication section
- [ ] Create `docs/authentication.md` guide
- [ ] Update `docs/security.md` with auth security model
- [ ] Create environment variable reference

#### Validation
- [ ] Run all unit tests
- [ ] Manual testing with production Dex
- [ ] Verify token file permissions (0600)
- [ ] Verify encryption/decryption correctness
- [ ] Verify domain replacement
- [ ] Verify public tools still work without auth

## 14. Notes

### Important Implementation Details

1. **Domain Replacement**: Critical for JWT authentication
   - User-facing URL: `https://intranet.giantswarm.io/docs/`
   - Actual request URL: `https://intranet-searchmcp.giantswarm.io/docs/`
   - Replacement must happen BEFORE adding Bearer token

2. **Token File Permissions**: Must be 0600 (owner only)
   - Verify after creation
   - Fail loudly if permissions are wrong

3. **Machine ID Fallback**: If machine ID unavailable
   - Use hostname as fallback
   - Log warning about reduced security

4. **OAuth Redirect URI**: Auto-generated from --http-addr
   - Example: `--http-addr=:8080` → `http://localhost:8080/oauth/callback`
   - Must be registered in Dex client config

5. **Token Refresh Threshold**: Hardcoded to 20% of lifetime
   - Assumed 1hr token → refresh at 48min
   - May need adjustment based on actual Dex configuration

### Questions for Code Review

1. Is the machine ID derivation approach acceptable for security?
2. Should we add a health check endpoint for auth status?
3. Do we need metrics even in Phase 1 (disabled by default)?
4. Should token file path be configurable via env var?

---

**End of Specification**

*This document serves as the implementation guide for Phase 1 of MCP authorization support in the Giant Swarm Search MCP Server.*
