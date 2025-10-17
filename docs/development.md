# Development Guide

This guide covers development setup, testing, and deployment of the Giant Swarm Search MCP Server.

## Prerequisites

- Python 3.13+
- `uv` package manager
- Docker (optional, for containerized deployment)

## Development Setup

### 1. Clone and Setup Environment

```bash
git clone <repository-url>
cd search-mcp

# Create virtual environment
uv venv

# Activate virtual environment
source .venv/bin/activate  # On Unix/macOS
# or
.venv\Scripts\activate  # On Windows

# Install dependencies
uv pip install -r requirements.txt
```

### 2. Configure Environment

```bash
# Copy example environment file
cp env.example .env

# Edit .env with your settings (optional for development)
# - INTRANET_SESSION_COOKIE (optional, for testing authenticated features)
# - PYTHONLOGLEVEL (optional, set to DEBUG for verbose logging)
```

### 3. Run the Server

```bash
# Run directly
python server.py

# Or use the run script
./run.sh
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
├── server.py           # Main MCP server implementation
├── requirements.txt    # Python dependencies (pinned versions)
├── pyproject.toml      # Project metadata and uv configuration
├── uv.lock            # Locked dependency versions
├── Dockerfile         # Container image definition
├── run.sh             # Helper script to run the server
├── env.example        # Example environment configuration
├── README.md          # User-facing documentation
└── docs/              # Developer documentation
    ├── architecture.md
    ├── security.md
    └── development.md
```

## Code Organization

### Main Components

- **AuthManager**: Handles authentication state and session persistence
- **MCP Tools**: Decorated functions that expose functionality to AI assistants
  - `authenticate()`: Authentication setup and verification
  - `search()`: Main search functionality
  - `search_ops_recipe()`: Specialized intranet search
  - `read_intranet_url()`: Fetch and parse intranet pages
  - `read_handbook_url()`: Fetch and parse handbook pages
- **Helper Functions**: Internal utilities like `_read_url_content()`

### Key Dependencies

- `fastmcp`: MCP server framework
- `aiohttp`: Async HTTP client
- `beautifulsoup4`: HTML parsing
- `markdownify`: HTML to Markdown conversion

## Testing

### Manual Testing

```bash
# Test public search (no authentication needed)
# Use your MCP client to call: search("kubernetes")

# Test authenticated search
# 1. Set up authentication first
# 2. Call: search("kubernetes")
# Should see both public and intranet results

# Test URL reading
# Call: read_handbook_url("https://handbook.giantswarm.io/...")
```

### Testing Authentication

```bash
# 1. Get a session cookie from the browser
# 2. Set environment variable
export INTRANET_SESSION_COOKIE="your_cookie_here"

# 3. Run server and test authenticated endpoints
python server.py
```

## Dependency Management

### Adding Dependencies

```bash
# Add a new package
uv pip install <package-name>

# Update requirements.txt with pinned versions
uv pip freeze > requirements.txt

# Update uv.lock
uv lock
```

### Updating Dependencies

```bash
# Update a specific package
uv pip install --upgrade <package-name>

# Update all packages
uv pip install --upgrade -r requirements.txt

# Regenerate requirements.txt
uv pip freeze > requirements.txt

# Update lock file
uv lock
```

## Debugging

### Enable Debug Logging

```bash
export PYTHONLOGLEVEL=DEBUG
python server.py
```

### Common Issues

#### "No module named X"
- Ensure virtual environment is activated
- Reinstall dependencies: `uv pip install -r requirements.txt`

#### "Authentication required" for public content
- Check that the endpoint selection logic is working
- Verify `is_authenticated()` returns correct value
- Check logs for endpoint being used

#### SSL/Certificate errors
- SSL verification is disabled in development (`ssl=False`)
- For production, remove this and ensure proper certificates

## Contributing

### Code Style

- Follow PEP 8 guidelines
- Use type hints where appropriate
- Add docstrings to functions and classes
- Keep functions focused and single-purpose

### Before Committing

1. Test your changes manually
2. Ensure no credentials are in the code
3. Update documentation if needed
4. Update `requirements.txt` if dependencies changed

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

- [ ] Enable SSL verification (remove `ssl=False`)
- [ ] Set appropriate log level (INFO or WARNING)
- [ ] Secure session file location
- [ ] Set up monitoring/logging
- [ ] Document deployment-specific configuration
- [ ] Test both authenticated and unauthenticated access
- [ ] Verify error handling works correctly

### Docker Deployment

```bash
# Build production image
docker build -t giantswarm-search-mcp:latest .

# Tag for registry
docker tag giantswarm-search-mcp:latest registry.example.com/giantswarm-search-mcp:latest

# Push to registry
docker push registry.example.com/giantswarm-search-mcp:latest

# Deploy (example using docker-compose or kubernetes)
```

## Troubleshooting Development Issues

### Server won't start
- Check Python version: `python --version` (need 3.13+)
- Verify dependencies: `uv pip check`
- Check for port conflicts

### Authentication not working
- Verify cookie format (should be from `_oauth2_proxy`)
- Check cookie hasn't expired (try getting a fresh one)
- Ensure environment variable is set correctly
- Check logs for authentication attempts

### Docker build fails
- Ensure Dockerfile is present
- Check Docker daemon is running
- Verify base image is available

## Resources

- [MCP Documentation](https://modelcontextprotocol.io/)
- [FastMCP](https://github.com/jlowin/fastmcp)
- [OAuth2 Proxy](https://github.com/oauth2-proxy/oauth2-proxy)
- [Elasticsearch 6.8 Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/6.8/query-dsl.html)

