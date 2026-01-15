# Security

This document covers security considerations for the Giant Swarm Search MCP Server.

## Authentication

The server implements OAuth 2.1 authentication with PKCE for accessing Giant Swarm's internal resources.

### Authentication Model

- **Public endpoints** (`docs.giantswarm.io`, `handbook.giantswarm.io`): No authentication required
- **Intranet endpoints** (`intranet.giantswarm.io`): OAuth 2.1 authentication required
- **Identity Provider**: Dex at `https://dex.operations.awsprod.gigantic.io`
- **OAuth Scopes**: `openid`, `offline_access`

### Token Security

**Storage:**
- Tokens encrypted at rest using AES-256-GCM
- File permissions: 0600 (owner read/write only)
- Stored in platform-specific config directories

**Encryption:**
- Algorithm: AES-256-GCM (authenticated encryption)
- Key derivation: Machine-specific identifiers (hardware UUID, machine-id, hostname)
- Nonce: Unique random nonce per encryption operation

**Token Lifecycle:**
- Access tokens valid for ~1 hour (determined by Dex)
- Refresh tokens valid for ~30 days (determined by Dex)
- Automatic proactive refresh when <20% of lifetime remains
- Refresh uses lock-based concurrency control

**Security Features:**
- ✅ Tokens never logged or exposed in error messages
- ✅ Bearer tokens transmitted only over HTTPS
- ✅ CSRF protection with random state parameter
- ✅ Automatic deletion of corrupted token files
- ✅ OAuth 2.1 with PKCE (Proof Key for Code Exchange)

### Authentication Limitations (Phase 1)

- ❌ **Stdio mode**: Authentication not available (Phase 1 - HTTP only)
- ❌ **Multi-user sessions**: Single-user mode only
- ❌ **Token revocation**: No active revocation mechanism
- ❌ **Session management**: No explicit logout endpoint

## Network Security

### SSL/TLS

The Go implementation uses the standard `net/http` package which:
- Enables SSL/TLS verification by default
- Uses the system's certificate pool
- Validates certificates automatically
- Provides secure HTTPS connections

### Endpoints

- **Public endpoint** (`docs.giantswarm.io/searchapi/`): No authentication required
- **Handbook** (`handbook.giantswarm.io`): Public access
- **Intranet** (`intranet.giantswarm.io`): May require authentication (not currently supported)

## Data Privacy

### What is stored locally

- Nothing is stored locally
- No search queries are stored
- No search results are cached
- No session or state files
- Stateless operation

### What is sent to servers

- Search queries are sent to OpenSearch at docs.giantswarm.io
- URL fetch requests to handbook.giantswarm.io or intranet.giantswarm.io
- Standard HTTP headers (User-Agent, Accept, Content-Type)

### What is NOT stored

- No credentials or authentication tokens
- No search history
- No personal identification information
- No cookies or session data

## Best Practices

### For Users

1. **Network security**
   - Ensure TLS/HTTPS connections are used
   - Verify certificate warnings if they appear
   - Use trusted networks when accessing documentation

2. **Binary verification**
   - Download binaries from official sources only
   - Verify checksums if provided
   - Use official Docker images

### For Developers

1. **Dependency management**
   - Keep dependencies up to date: `go get -u ./...`
   - Review security advisories: `go list -m -u all`
   - Use `go mod verify` to check integrity

2. **Secure coding**
   - Follow Go security best practices
   - Use structured logging (no sensitive data in logs)
   - Set appropriate HTTP timeouts
   - Handle errors properly

3. **Deployment**
   - Run as non-root user (Dockerfile uses `nobody`)
   - Use minimal container images (Alpine-based)
   - Set resource limits
   - Enable logging and monitoring

## Threat Model

### Threats Mitigated

✅ **Man-in-the-middle attacks**: HTTPS with certificate verification enabled by default
✅ **Code injection**: Input validation on all tool parameters
✅ **Container security**: Run as non-root user, minimal attack surface
✅ **Token theft (at rest)**: AES-256-GCM encryption with file permissions 0600
✅ **Token theft (in transit)**: Bearer tokens only over HTTPS
✅ **CSRF attacks**: Random state parameter validation in OAuth flow
✅ **Unauthorized intranet access**: OAuth 2.1 authentication required

### Current Limitations

⚠️ **Stdio mode authentication**: Not available in Phase 1 (HTTP only)
⚠️ **No rate limiting**: Consider adding for production deployments
⚠️ **No input sanitization for HTML**: Relies on third-party libraries for safe conversion
⚠️ **Machine-derived encryption keys**: Security depends on machine uniqueness

### Recommended Improvements for Production

1. **Add rate limiting** to prevent abuse of search and auth endpoints
2. **Implement request logging** for audit trail (especially auth events)
3. **Add health checks** for monitoring (including auth system health)
4. **Set up alerts** for error conditions (especially auth failures)
5. **Implement device flow** for stdio mode authentication (Phase 2)
6. **Add token revocation** mechanism for security incidents
7. **Consider hardware security modules** for encryption key storage

## Security Updates

To stay informed about security updates:

1. **Monitor dependencies**:
   ```bash
   # Check for updates
   go list -m -u all
   
   # Update dependencies
   go get -u ./...
   go mod tidy
   ```

2. **Subscribe to security advisories**:
   - GitHub security advisories for Go
   - Security mailing lists for dependencies
   - Giant Swarm security notifications

3. **Regular updates**:
   - Keep Go toolchain updated
   - Rebuild Docker images regularly
   - Apply security patches promptly

