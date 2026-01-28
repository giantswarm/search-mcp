# Giant Swarm Search MCP Server

An MCP (Model Context Protocol) server that provides AI assistants with search access to Giant Swarm's public documentation and handbook.

## Features

- Search public Giant Swarm documentation
- Read and convert documentation pages to Markdown
- Support for both stdio and HTTP transports
- Fast and efficient Go implementation

## Quick Start

### Cursor

1. Open Settings → Tools & MCP
2. Click **New MCP Server**
3. Edit the settings like this:

```json
"giantswarm-search": {
   "command": "docker",
   "args": [
      "run",
      "-i",
      "--rm",
      "gsoci.azurecr.io/giantswarm/search-mcp:TAG",
      "serve"
   ]
}
```

or alternatively with the binary installed

"giantswarm-search": {
   "command": "search-mcp",
   "args": ["serve"]
}
```

Make sure to adapt the version TAG to the version you intend to use. [Check here](https://oci.dag.dev/?repo=gsoci.azurecr.io%2Fgiantswarm%2Fsearch-mcp) for available versions.

### Installation

#### Using Go

```bash
go install github.com/giantswarm/search-mcp@latest
```

#### Using Docker

```bash
# Pull the image
docker pull gsoci.azurecr.io/giantswarm/search-mcp:TAG

# Run with stdio transport (for direct interaction)
docker run -i --rm gsoci.azurecr.io/giantswarm/search-mcp:TAG serve

# Run with HTTP transport (for network access)
docker run -d -p 8080:8080 \
  gsoci.azurecr.io/giantswarm/search-mcp:TAG \
  serve --transport=streamable-http --http-addr=:8080

# Check version
docker run --rm gsoci.azurecr.io/giantswarm/search-mcp:TAG version
```

#### From Source

```bash
git clone https://github.com/giantswarm/search-mcp
cd search-mcp
make build
```

## Usage

### Running the Server

```bash
# Run with stdio transport (default, for Cursor)
search-mcp
# or explicitly:
search-mcp serve

# Run with HTTP transport
search-mcp serve --transport=streamable-http --http-addr=:8080

# Run with debug logging
search-mcp serve --debug

# Show version
search-mcp version
```

### Available Tools

The server provides these tools to AI assistants:

#### `search(term, start_index, size, type_filter, breadcrumb_filter)`

Search Giant Swarm documentation (public docs from docs.giantswarm.io).

**Parameters:**
- `term` (required): Search term
- `start_index` (optional): Starting index for pagination (default: 0)
- `size` (optional): Number of results to return (default: 30)
- `type_filter` (optional): Filter by source type
- `breadcrumb_filter` (optional): Filter by section path

#### `search_runbook(term, start_index, size)`

Search for DevOps runbooks in the Giant Swarm intranet. **Requires authentication.**

#### `search_ops_recipe(term, start_index, size)`

Search for Ops Recipes (legacy runbooks) in the Giant Swarm intranet. **Requires authentication.**

#### `read_handbook_url(url)`

Read content from Giant Swarm handbook (public, no authentication required).

**Parameters:**
- `url` (required): URL from https://handbook.giantswarm.io/

#### `read_intranet_url(url)`

Read content from Giant Swarm intranet. **Requires authentication.**

**Parameters:**
- `url` (required): URL from https://intranet.giantswarm.io/

## Authentication

The MCP server supports OAuth 2.1 authentication for accessing Giant Swarm's internal resources (intranet, runbooks, ops recipes).

### Quick Start

#### HTTP Mode (Streamable HTTP Transport)

1. **Set environment variables**:
   ```bash
   export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
   export OAUTH_CLIENT_ID=searchmcp
   # export OAUTH_CLIENT_SECRET=<your-secret>  # Optional - only if client requires a secret
   ```

2. **Run in HTTP mode**:
   ```bash
   search-mcp serve --transport=streamable-http --http-addr=:8080
   ```

3. **Authenticate**:
   - Visit http://localhost:8080/oauth/login
   - Sign in with your Giant Swarm credentials
   - You're authenticated!

#### Stdio Mode (Claude Desktop, Cursor, etc.)

1. **Set environment variables**:
   ```bash
   export OAUTH_ISSUER_URL=https://dex.operations.awsprod.gigantic.io
   export OAUTH_CLIENT_ID=searchmcp
   # export OAUTH_CLIENT_SECRET=<your-secret>  # Optional - only if client requires a secret
   ```

2. **Run in stdio mode** (default):
   ```bash
   search-mcp serve
   ```

3. **Authenticate when needed**:
   - When you use an intranet tool (like `read_intranet_url`), the server will display:
     - A verification URL (e.g., http://localhost:8080/device)
     - A device code (e.g., ABCD-EFGH)
   - Visit the URL in your browser and enter the code
   - Sign in with your Giant Swarm credentials
   - Return to your MCP client and retry the tool
   - You're now authenticated!

**Note**: Stdio mode uses OAuth Device Authorization Grant (RFC 8628). The server automatically starts a background HTTP server on port 8080 for device authorization.

### What Requires Authentication?

- ✅ **Public tools** (no auth required):
  - `read_docs_index` - Get index of public documentation pages
  - `read_docs_url` - Read public documentation pages
  - `read_handbook_url` - Read public handbook pages
  - `search_docs` - Search public documentation
  - `search` - Search public documentation

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

## Configuration

### Command-Line Flags

- `--transport`: Transport type - `stdio` or `streamable-http` (default: `stdio`)
- `--http-addr`: HTTP server address (default: `:8080`)
- `--http-endpoint`: HTTP endpoint path (default: `/mcp`)
- `--debug`: Enable debug logging
- `--version`: Show version information

## Known Issues / TODO

None at this time.

## Troubleshooting

### Connection errors

- Verify your network connection
- Check that docs.giantswarm.io is accessible

### Debug logging

Enable debug logging to see detailed information:

```bash
search-mcp serve --debug
```

### Building from source

If you encounter issues with the pre-built binaries:

```bash
git clone https://github.com/giantswarm/search-mcp
cd search-mcp
go build -o search-mcp .
./search-mcp version
```

## Development

For development setup, building from source, and contributing:

```bash
# Clone repository
git clone https://github.com/giantswarm/search-mcp
cd search-mcp

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run locally
make run
```

See [Development Guide](docs/development.md) for detailed instructions.

## Technical Documentation

- **[Architecture](docs/architecture.md)** - How the server works internally
- **[Security](docs/security.md)** - Security considerations and best practices  
- **[Development](docs/development.md)** - Development setup and contribution guide

## License

Apache License 2.0 - See [LICENSE](LICENSE) file for details.
