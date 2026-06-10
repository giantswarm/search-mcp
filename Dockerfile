# The Go binary is built by CircleCI (architect/go-build) and attached to the
# build context as <binary>-<os>-<arch>; this image only assembles the runtime.
# For a local build, produce the binary first:
#   CGO_ENABLED=0 go build -o search-mcp-linux-amd64 .
FROM gsoci.azurecr.io/giantswarm/alpine:3.24.0

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

ARG TARGETOS
ARG TARGETARCH
COPY search-mcp-${TARGETOS}-${TARGETARCH} /usr/local/bin/search-mcp

# Run as nobody user
USER nobody

# Expose port for HTTP transport
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/search-mcp", "version"]

# Run the MCP server (default: stdio transport)
ENTRYPOINT ["/usr/local/bin/search-mcp"]
