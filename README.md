# Giant Swarm Search MCP Server

An experimental MCP (Model Context Protocol) server that provides AI assistants with search access to Giant Swarm's documentation, handbook, and intranet.

## Features

- Search public Giant Swarm documentation (no authentication required)
- Search internal intranet resources (with optional authentication)
- Automatic endpoint selection based on authentication status
- Read and convert documentation pages to Markdown
- Session persistence across restarts

## Quick Start

### Installation

```bash
# Install dependencies
pip install -r requirements.txt

# Run the server
python server.py
```

**That's it!** The server is now running and you can search public documentation without any additional setup.

### Optional: Enable Intranet Access

To access internal Giant Swarm resources, set up authentication:

1. Visit [https://intranet.giantswarm.io/](https://intranet.giantswarm.io/) and login with GitHub
2. Open browser Developer Tools (F12) → Application → Cookies
3. Copy the `_oauth2_proxy` cookie value
4. Set environment variable:
   ```bash
   export INTRANET_SESSION_COOKIE="your_oauth2_proxy_cookie_value"
   ```

## Usage

The server provides these tools to AI assistants:

### `search(term: str)`
Search Giant Swarm documentation.
- Without authentication: Public docs only (docs.giantswarm.io, blog, etc.)
- With authentication: Public + intranet resources

### `search_ops_recipe(term: str)`
Search Ops Recipes (runbooks) in the intranet. Requires authentication.

### `read_handbook_url(url: str)`
Read content from Giant Swarm handbook. No authentication required.

### `read_intranet_url(url: str)`
Read content from Giant Swarm intranet. Requires authentication.

## Configuration

### Environment Variables

- `INTRANET_SESSION_COOKIE`: OAuth2 proxy session cookie (optional, for intranet access)
- `PYTHONLOGLEVEL`: Logging level - DEBUG, INFO, WARNING, or ERROR (optional)

Copy `env.example` to `.env` and customize as needed.

## Troubleshooting

### Limited search results

You're in public-only mode. To access intranet resources, set the `INTRANET_SESSION_COOKIE` environment variable following the authentication setup instructions above.

### "Authentication required" error

Your session cookie may have expired. To fix this:

1. Log into https://intranet.giantswarm.io/ again
2. Get a fresh cookie value:
   - Open the developer tools (F12 or Cmd + Option + I)
   - Go to Application → Cookies → `https://intranet.giantswarm.io`
   - Select the cookie named `_oauth2_proxy`
   - Copy the cookie value
3. Update your environment variable:
   - In Cursor: Settings → Tools & MCP → Edit `giantswarm-search` → Update INTRANET_SESSION_COOKIE
   - Or in terminal: `export INTRANET_SESSION_COOKIE="your_new_cookie_value"`

### Connection errors

- Verify your network connection
- Check that docs.giantswarm.io is accessible
- For intranet access, ensure your credentials are valid

## Documentation

- **[Architecture](docs/architecture.md)** - How the server works internally
- **[Security](docs/security.md)** - Security considerations and best practices  
- **[Development](docs/development.md)** - Development setup and contribution guide

## License

See LICENSE file for details.
