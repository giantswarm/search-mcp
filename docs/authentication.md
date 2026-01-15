# Authentication Guide

This guide explains how authentication works in the Giant Swarm Search MCP Server for both HTTP and stdio transports.

## Overview

The MCP server uses OAuth 2.1 for authentication with Giant Swarm's Dex identity provider. Authentication is required only for accessing internal resources (intranet, runbooks, ops recipes). Public documentation tools work without authentication.

## Transport Modes

The server supports two transport modes, each with its own authentication flow:

### HTTP Mode (Streamable HTTP)

Used when running the server as a web service. Best for:
- Direct HTTP API access
- Custom integrations
- Development and testing

**Authentication Flow**: Standard OAuth 2.1 Authorization Code Flow with PKCE

### Stdio Mode (Standard Input/Output)

Used by MCP clients like Claude Desktop, Cursor, and other AI assistants. Best for:
- Integration with AI coding assistants
- Command-line workflows
- IDE integrations

**Authentication Flow**: OAuth 2.1 Device Authorization Grant (RFC 8628)

## HTTP Mode Setup

### 1. Configure Environment

Set the required OAuth environment variables:

```bash
export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
export OAUTH_CLIENT_ID=searchmcp
export OAUTH_CLIENT_SECRET=<your-secret>
```

### 2. Start Server

```bash
search-mcp serve --transport=streamable-http --http-addr=:8080
```

### 3. Authenticate

1. Open your browser and visit: http://localhost:8080/oauth/login
2. You'll be redirected to Dex for authentication
3. Sign in with your Giant Swarm credentials
4. After successful authentication, you'll be redirected back
5. Your tokens are now stored and the server is authenticated

### 4. Use Authenticated Tools

Once authenticated, you can use any of the intranet tools via HTTP API requests. The server will automatically inject your Bearer token into requests.

### Endpoints

- **Login**: `http://localhost:8080/oauth/login` - Initiates OAuth flow
- **Callback**: `http://localhost:8080/callback` - OAuth callback handler (automatic)
- **MCP Endpoint**: `http://localhost:8080/mcp` - MCP protocol endpoint

## Stdio Mode Setup

### 1. Configure Environment

Set the required OAuth environment variables:

```bash
export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
export OAUTH_CLIENT_ID=searchmcp
export OAUTH_CLIENT_SECRET=<your-secret>
```

### 2. Start Server

```bash
search-mcp serve
# or explicitly:
search-mcp serve --transport=stdio
```

When started in stdio mode with OAuth configured, the server automatically starts a background HTTP server on port 8080 for device authorization.

### 3. Authenticate When Needed

Authentication happens on-demand when you first use an intranet tool:

1. Use an intranet tool (e.g., `read_intranet_url` in Cursor)
2. The server responds with device authorization instructions:
   ```
   ❌ Authentication required

   To access intranet resources, please authorize this device:

   1. Visit: http://localhost:8080/device
   2. Enter code: ABCD-EFGH
   3. Sign in with your Giant Swarm credentials

   After authorizing, please retry this tool.

   Code expires in 10 minutes.
   ```
3. Open your browser and visit the URL
4. Enter the displayed device code (format: XXXX-XXXX)
5. Complete the OAuth flow with Dex
6. Return to your MCP client and retry the tool
7. The tool now works - you're authenticated!

### Device Flow Details

The device authorization flow works as follows:

1. **Device Code Generation**: Server generates a device code and user-friendly code
2. **User Authorization**: User visits verification URL and enters code
3. **OAuth Flow**: User completes standard OAuth flow via Dex
4. **Token Storage**: Tokens are stored in shared token storage
5. **Retry Pattern**: User retries the tool, server validates authentication

**Key Features**:
- Device codes expire after 10 minutes
- User codes are 8 characters (format: XXXX-XXXX) for easy entry
- Tokens are shared between device flow and HTTP mode
- Background HTTP server runs on localhost:8080

### Endpoints (Background Server)

When running in stdio mode, a background HTTP server provides:

- **Device Authorization**: `http://localhost:8080/device` - Enter device code
- **OAuth Callback**: `http://localhost:8080/callback` - OAuth callback handler
- **Login** (optional): `http://localhost:8080/oauth/login` - Direct login for stdio mode

## Token Storage

Tokens are encrypted using AES-256-GCM and stored in a platform-specific location:

- **Linux**: `~/.config/giantswarm/tokens.enc`
- **macOS**: `~/Library/Application Support/giantswarm/tokens.enc`
- **Windows**: `%APPDATA%\giantswarm\tokens.enc`

### Encryption

Tokens are encrypted using a key derived from your machine's unique identifier:
- **Linux**: `/etc/machine-id`
- **macOS**: IOPlatformUUID from IOKit
- **Windows**: Hostname

The encryption key is derived using SHA-256 from the machine identifier, ensuring tokens are machine-specific and cannot be easily transferred.

### Token Refresh

Tokens are automatically refreshed when they approach expiration:
- Refresh occurs when token has less than 20% of its lifetime remaining
- For 1-hour tokens, refresh happens at the 12-minute mark
- Refresh is atomic with mutex locking to prevent race conditions
- Failed refresh triggers re-authentication prompt

### Token Lifecycle

1. **Acquisition**: Tokens obtained via OAuth flow
2. **Storage**: Encrypted and written to disk
3. **Usage**: Automatically injected as Bearer token in requests
4. **Refresh**: Proactive refresh before expiration
5. **Expiration**: User prompted to re-authenticate if refresh fails

## Tools and Authentication

### Public Tools (No Auth Required)

- `search` - Search public documentation
- `read_handbook_url` - Read public handbook pages

### Intranet Tools (Auth Required)

- `read_intranet_url` - Read intranet pages
- `search_runbook` - Search DevOps runbooks
- `search_ops_recipe` - Search ops recipes

## Troubleshooting

### Environment Not Configured

**Error**: "Authentication not configured"

**Solution**: Set the required environment variables:
```bash
export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
export OAUTH_CLIENT_ID=searchmcp
export OAUTH_CLIENT_SECRET=<your-secret>
```

### Not Authenticated

**HTTP Mode**: Visit http://localhost:8080/oauth/login to authenticate

**Stdio Mode**: Use an intranet tool to trigger device flow, then follow the instructions

### Token Expired

Tokens are automatically refreshed. If you see an expiration error:

**HTTP Mode**: Visit http://localhost:8080/oauth/login to re-authenticate

**Stdio Mode**: Use an intranet tool again - you'll get device flow instructions if needed

### Device Code Expired

Device codes expire after 10 minutes. If your code expires:

1. Retry the tool in your MCP client
2. A new device code will be generated
3. Complete authorization with the new code

### Port 8080 Already in Use

If port 8080 is already in use:

**HTTP Mode**: Use a different port:
```bash
search-mcp serve --transport=streamable-http --http-addr=:8081
```

**Stdio Mode**: The background HTTP server uses port 8080 by default. If you're running multiple instances, stop other instances or use a different machine.

### Clear Stored Tokens

To manually clear stored tokens:

**Linux/macOS**:
```bash
rm ~/.config/giantswarm/tokens.enc  # Linux
rm ~/Library/Application\ Support/giantswarm/tokens.enc  # macOS
```

**Windows**:
```powershell
del %APPDATA%\giantswarm\tokens.enc
```

## Security Considerations

### Token Security

- Tokens are encrypted at rest using AES-256-GCM
- File permissions are set to 0600 (owner read/write only)
- Encryption key derived from machine-specific identifier
- Tokens cannot be easily transferred between machines

### OAuth Security

- PKCE (Proof Key for Code Exchange) enabled
- CSRF protection via state parameter
- Secure cookie handling (HttpOnly, SameSite)
- All OAuth traffic over HTTPS

### Best Practices

1. **Never commit OAuth secrets**: Keep `OAUTH_CLIENT_SECRET` out of version control
2. **Use environment variables**: Don't hardcode credentials
3. **Protect token storage**: Don't share your tokens.enc file
4. **Regular token rotation**: Tokens automatically refresh, but re-authenticate periodically
5. **Secure your machine**: Token encryption is only as secure as your machine's access controls

## Technical Details

### OAuth 2.1 vs OAuth 2.0

This implementation uses OAuth 2.1 best practices:
- PKCE required for authorization code flow
- Refresh token rotation supported
- No implicit flow support
- Secure credential storage

### Device Authorization Grant (RFC 8628)

Stdio mode implements RFC 8628 Device Authorization Grant:
- Device code: Long cryptographically secure code (32 bytes, base64)
- User code: Short human-friendly code (8 chars, format: XXXX-XXXX)
- Verification URI: Local HTTP endpoint for authorization
- Polling: User retry pattern (not automatic polling)

### Implementation

For technical implementation details, see:
- [Architecture Documentation](architecture.md)
- [Security Documentation](security.md)
- [MCP Auth Implementation Spec](mcp-auth-implementation-spec.md)

## Support

For issues or questions:
- Check the main [README](../README.md)
- Review [troubleshooting](#troubleshooting) above
- Open an issue on GitHub
