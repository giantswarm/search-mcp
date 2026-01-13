# Development Guide

This guide covers development setup, testing, and deployment of the Giant Swarm Search MCP Server.

## Prerequisites

- Go 1.23 or later
- Docker (optional, for containerized deployment)
- Make (optional, for using Makefile targets)

## Development Setup

### 1. Clone and Setup

```bash
git clone <repository-url>
cd search-mcp

# Download dependencies
go mod download
```

### 2. Build the Server

```bash
# Using Make
make build

# Or using Go directly
go build -o bin/search-mcp .
```

### 3. Run the Server

```bash
# Using Make (stdio transport)
make run

# Using Make (HTTP transport)
make run-http

# Using Make (with debug logging)
make run-debug

# Or run directly
./bin/search-mcp serve
./bin/search-mcp serve --transport=streamable-http --http-addr=:8080
./bin/search-mcp serve --debug
```

## Docker Development

### Building the Image

```bash
docker build -t giantswarm-search-mcp .
```

### Running with Docker

```bash
# Run without authentication (public search only)
docker run -d \
    --name giantswarm-search-mcp \
    giantswarm-search-mcp

# Run with authentication for testing intranet access
docker run -d \
    --name giantswarm-search-mcp \
    -e INTRANET_SESSION_COOKIE="your_cookie_value" \
    -v ./session_data:/home/app/.giantswarm_mcp_session \
    giantswarm-search-mcp

# View logs
docker logs -f giantswarm-search-mcp

# Stop and remove
docker stop giantswarm-search-mcp
docker rm giantswarm-search-mcp
```

## Project Structure

```
search-mcp/
├── main.go                  # Entry point and CLI
├── internal/
│   └── search/
│       ├── server.go        # MCP server setup and lifecycle
│       ├── tools.go         # MCP tool implementations
│       ├── client.go        # HTTP client for search API
│       ├── html.go          # HTML to Markdown conversion
│       └── types.go         # Data structures
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
├── Dockerfile               # Multi-stage container build
├── Makefile                 # Generated build targets
├── Makefile.custom.mk       # Custom build targets
├── README.md                # User-facing documentation
└── docs/                    # Developer documentation
    ├── architecture.md
    ├── security.md
    └── development.md
```

## Code Organization

### Main Packages

- **main.go**: Entry point with CLI using Go's `flag` package
- **internal/search**: Core search functionality (internal, not exported)
  - `Server`: MCP server lifecycle management
  - `Client`: HTTP client for search and URL fetching
  - Tool handlers: Implement 5 MCP tools
  - HTML conversion: Parse and convert HTML to Markdown

### Key Dependencies

- `github.com/mark3labs/mcp-go`: MCP server framework
- `github.com/PuerkitoBio/goquery`: HTML parsing (like BeautifulSoup)
- `github.com/JohannesKaufmann/html-to-markdown/v2`: HTML to Markdown conversion

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests manually
go test -v ./...
go test -v -race ./...
```

### Manual Testing

Use an MCP client (like Cursor or Claude Desktop) to test:

1. **Search functionality**:
   - Call `search("kubernetes")`
   - Test pagination with `start_index`
   - Test filters with `type_filter` and `breadcrumb_filter`

2. **URL reading**:
   - Call `read_handbook_url("https://handbook.giantswarm.io/...")`
   - Verify Markdown conversion

3. **Specialized searches**:
   - Test `search_runbook("incident")`
   - Test `search_ops_recipe("deployment")`

## Dependency Management

This project uses Go modules for dependency management.

### Adding Dependencies

```bash
# Add a new dependency (automatically updates go.mod and go.sum)
go get github.com/package/name

# Add with specific version
go get github.com/package/name@v1.2.3

# Download all dependencies
go mod download

# Tidy up dependencies (remove unused)
go mod tidy
```

### Updating Dependencies

```bash
# Update specific dependency to latest
go get -u github.com/package/name

# Update all dependencies
go get -u ./...

# Tidy after updates
go mod tidy
```

### Verifying Dependencies

```bash
# Verify dependencies match checksums
go mod verify

# View dependency graph
go mod graph

# List all dependencies
go list -m all
```

## Debugging

### Enable Debug Logging

```bash
# Using flag
search-mcp serve --debug

# With Make
make run-debug
```

### Common Issues

#### Build errors
- Ensure Go 1.23+ is installed: `go version`
- Update dependencies: `go mod tidy`
- Clear module cache: `go clean -modcache`

#### Import errors
- Verify module path in go.mod: `github.com/giantswarm/search-mcp`
- Ensure internal packages aren't imported from outside

#### Runtime errors
- Check logs for detailed error messages
- Enable debug logging with `--debug`
- Verify network connectivity to docs.giantswarm.io

## Contributing

### Code Style

- Follow Go standard formatting: `gofmt`, `goimports`
- Use `golangci-lint` for linting
- Add comments for exported functions and types
- Keep functions focused and single-purpose

### Before Committing

1. Format code: `make fmt`
2. Run linters: `make lint`
3. Run tests: `make test`
4. Test changes manually
5. Ensure no credentials are in the code
6. Update documentation if needed
7. If dependencies changed:
   - Run `go mod tidy`
   - Commit both `go.mod` and `go.sum`

### Git Workflow

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Make changes and commit
git add .
git commit -m "Description of changes"

# Push and create pull request
git push origin feature/your-feature-name
```

## Deployment

### Production Checklist

- [ ] Build production binary with optimizations
- [ ] Set appropriate log level (INFO or WARNING)
- [ ] Set up monitoring/logging
- [ ] Configure proper timeouts
- [ ] Test all tools work correctly
- [ ] Verify error handling

### Docker Deployment

```bash
# Build production image using Make
make docker-build

# Or build manually
docker build -t gsoci.azurecr.io/giantswarm/search-mcp:latest .

# Push to registry
make docker-push

# Run container
docker run -d \
  --name search-mcp \
  -p 8080:8080 \
  gsoci.azurecr.io/giantswarm/search-mcp:latest \
  serve --transport=streamable-http --http-addr=:8080
```

### Binary Deployment

```bash
# Build optimized binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o search-mcp .

# Run as systemd service (example)
sudo cp search-mcp /usr/local/bin/
sudo systemctl start search-mcp
```

## Troubleshooting Development Issues

### Server won't start
- Check Go version: `go version` (need 1.23+)
- Verify dependencies: `go mod verify`
- Check for port conflicts (if using HTTP transport)
- Review logs with `--debug` flag

### Build fails
- Clean build cache: `go clean -cache`
- Update dependencies: `go mod tidy`
- Check for syntax errors: `go vet ./...`

### Docker build fails
- Ensure Docker daemon is running
- Check Dockerfile syntax
- Verify base images are accessible
- Try building with `--no-cache`

## Available Make Targets

```bash
make help           # Show all available targets
make build          # Build binary
make test           # Run tests
make lint           # Run linters
make fmt            # Format code
make docker-build   # Build Docker image
make run            # Run with stdio transport
make run-http       # Run with HTTP transport
make run-debug      # Run with debug logging
make clean          # Clean build artifacts
make deps           # Download dependencies
make verify         # Run all verification steps
```

## Resources

- [MCP Documentation](https://modelcontextprotocol.io/)
- [mcp-go](https://github.com/mark3labs/mcp-go)
- [Go Documentation](https://golang.org/doc/)
- [OpenSearch Query DSL](https://opensearch.org/docs/latest/query-dsl/)

