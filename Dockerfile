FROM --platform=$BUILDPLATFORM gsoci.azurecr.io/giantswarm/alpine:3.24.0 AS certs

FROM scratch

COPY --from=certs /etc/passwd /etc/passwd
COPY --from=certs /etc/group /etc/group
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary is pre-built per architecture by the architect/go-build CI job
# (or by `make docker-build` locally); no RUN steps means no QEMU emulation.
ARG TARGETARCH
COPY search-mcp-linux-${TARGETARCH} /usr/local/bin/search-mcp

# Run as nobody user
USER nobody

# Expose port for HTTP transport
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/search-mcp", "version"]

# Run the MCP server (default: stdio transport)
ENTRYPOINT ["/usr/local/bin/search-mcp"]
