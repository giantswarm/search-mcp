# Security

This document covers security considerations for the Giant Swarm Search MCP Server.

## Authentication Storage

### Environment Variables

- **Storage**: The `INTRANET_SESSION_COOKIE` environment variable contains authentication credentials
- **Scope**: Environment variables are process-scoped and not visible to other users
- **No Persistence**: Cookies are not stored in files - only in memory from the environment variable
- **Best Practice**: Never commit `.env` files with actual credentials to version control

## OAuth2 Proxy Integration

The intranet is protected by [OAuth2 Proxy](https://github.com/oauth2-proxy/oauth2-proxy), which:

- Handles GitHub OAuth authentication
- Issues session cookies (`_oauth2_proxy`)
- Manages session expiration
- Redirects unauthenticated requests to login

### Cookie Expiration

- OAuth2 proxy cookies typically expire after a period of inactivity
- When expired, users need to re-authenticate through the browser
- The server detects expired sessions and prompts users to refresh credentials

## Network Security

### SSL/TLS

**⚠️ Development Setting**: SSL verification is currently disabled in the code:

```python
connector = aiohttp.TCPConnector(ssl=False)
```

**Recommendation for Production**:
- Remove `ssl=False` to enable certificate verification
- Or provide proper certificate validation
- This prevents man-in-the-middle attacks

### Endpoints

- **Public endpoint** (`docs.giantswarm.io`): No authentication required
- **Intranet endpoint** (`intranet.giantswarm.io`): Requires OAuth2 proxy authentication

## Data Privacy

### What is stored locally

- Nothing is stored locally
- No search queries are stored
- No search results are cached
- No session files

### What is sent to servers

- Search queries are sent to Elasticsearch
- Authentication cookies (from environment variable) are sent to intranet.giantswarm.io
- Standard HTTP headers (User-Agent, etc.)

### What is NOT stored

- User passwords (authentication is cookie-based)
- Search history
- Personal identification information
- Session cookies (only read from environment variable)

## Best Practices

### For Users

1. **Protect your session cookie**
   - Don't share your `INTRANET_SESSION_COOKIE` value
   - Don't commit it to version control
   - Treat it like a password

2. **Refresh credentials regularly**
   - Re-authenticate when prompted
   - Don't use expired sessions

3. **Use environment files safely**
   - Copy `env.example` to `.env` (which is .gitignored)
   - Never commit `.env` with real credentials

### For Developers

1. **Enable SSL verification for production**
   - Remove `ssl=False` from connector configuration
   - Test with proper certificates

2. **Session management**
   - Cookies are only read from environment variables
   - No local file storage to secure
   - Expired sessions are detected and reported

3. **Audit logging**
   - Consider adding audit logs for security events
   - Log authentication failures and suspicious activity

## Threat Model

### Threats Mitigated

✅ **Unauthorized access to intranet**: OAuth2 proxy authentication required  
✅ **Credential exposure in code**: Credentials stored in environment variables and local files  
✅ **Session hijacking**: HTTPS transport encryption (when SSL verification is enabled)

### Potential Vulnerabilities

⚠️ **SSL disabled**: Current development setting disables certificate verification  
⚠️ **Environment variable exposure**: Environment variables could be logged or exposed in process listings

### Recommended Improvements

1. **Enable SSL verification** for production deployments
2. **Implement session validation** to verify cookie format and expiration
3. **Add rate limiting** to prevent abuse
4. **Log security events** for audit trail

## Incident Response

If you suspect your session cookie has been compromised:

1. **Immediately revoke access**:
   - Visit https://intranet.giantswarm.io/
   - Log out from all sessions
   - Log back in to get a new session

2. **Remove old credentials**:
   ```bash
   unset INTRANET_SESSION_COOKIE
   ```
   Or update your MCP configuration to remove/replace the cookie value

3. **Get new credentials**:
   - Follow the authentication setup process again
   - Obtain a fresh session cookie

4. **Report the incident** to the Giant Swarm security team if needed

