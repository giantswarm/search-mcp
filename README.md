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
      "gsoci.azurecr.io/giantswarm/search-mcp:latest"
   ]
}
```

Make sure to adapt the version tag to the version you intend to use. [Check here](https://oci.dag.dev/?repo=gsoci.azurecr.io%2Fgiantswarm%2Fsearch-mcp) for available versions.

### Installation

#### Using Go

```bash
go install github.com/giantswarm/search-mcp/cmd/server@latest
```

#### Using Docker

```bash
docker pull gsoci.azurecr.io/giantswarm/search-mcp:latest
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
mcp-server

# Run with HTTP transport
mcp-server --transport=streamable-http --http-addr=:8080

# Run with debug logging
mcp-server --debug

# Show version
mcp-server --version
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

Search for DevOps runbooks in the documentation.

#### `search_ops_recipe(term, start_index, size)`

Search for Ops Recipes (legacy runbooks) in the documentation.

#### `read_handbook_url(url)`

Read content from Giant Swarm handbook (public, no authentication required).

**Parameters:**
- `url` (required): URL from https://handbook.giantswarm.io/

#### `read_intranet_url(url)`

Read content from Giant Swarm intranet (may require authentication depending on deployment).

**Parameters:**
- `url` (required): URL from https://intranet.giantswarm.io/

## Configuration

### Command-Line Flags

- `--transport`: Transport type - `stdio` or `streamable-http` (default: `stdio`)
- `--http-addr`: HTTP server address (default: `:8080`)
- `--http-endpoint`: HTTP endpoint path (default: `/mcp`)
- `--debug`: Enable debug logging
- `--version`: Show version information

## Troubleshooting

### Connection errors

- Verify your network connection
- Check that docs.giantswarm.io is accessible

### Debug logging

Enable debug logging to see detailed information:

```bash
mcp-server --debug
```

### Building from source

If you encounter issues with the pre-built binaries:

```bash
git clone https://github.com/giantswarm/search-mcp
cd search-mcp
go build -o mcp-server ./cmd/server
./mcp-server --version
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
