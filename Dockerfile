# Stage 1: Build
FROM gsoci.azurecr.io/giantswarm/golang:1.26.3-alpine3.23 AS builder

WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /search-mcp .

# Stage 2: Runtime
FROM gsoci.azurecr.io/giantswarm/alpine:3.23.4

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /search-mcp /usr/local/bin/search-mcp

# Run as nobody user
USER nobody

# Expose port for HTTP transport
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/search-mcp", "version"]

# Run the MCP server (default: stdio transport)
ENTRYPOINT ["/usr/local/bin/search-mcp"]
