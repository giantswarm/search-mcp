# Security

This document covers security considerations for the Giant Swarm Search MCP Server.

## Authentication Status

**Note**: Authentication is currently not implemented in this version of the server. The server only accesses public endpoints that do not require authentication.

- Public documentation at `docs.giantswarm.io` is accessible without credentials
- Internal resources at `intranet.giantswarm.io` may require authentication in future versions
- Authentication will be implemented differently in a future release

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

### Current Limitations

⚠️ **No authentication**: Server accesses only public endpoints
⚠️ **No rate limiting**: Consider adding for production deployments  
⚠️ **No input sanitization for HTML**: Relies on third-party libraries for safe conversion

### Recommended Improvements for Production

1. **Add rate limiting** to prevent abuse of search API
2. **Implement request logging** for audit trail
3. **Add health checks** for monitoring
4. **Set up alerts** for error conditions
5. **Consider adding authentication** when accessing protected resources

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

