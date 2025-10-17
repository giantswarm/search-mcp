FROM python:3.13-slim

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Install system dependencies
RUN apt-get update && apt-get install -y \
    --no-install-recommends curl build-essential \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Create non-root user for security
RUN useradd --create-home --shell /bin/bash app && \
    chown -R app:app /app
USER app

# Create directory for session storage
RUN mkdir -p /home/app/.giantswarm_mcp_session


# Install the 'uv' CLI.
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# Make sure 'uv' is on PATH.
ENV PATH="/home/app/.local/bin:${PATH}"

# Copy requirements and install dependencies
COPY requirements.txt .
RUN uv venv
RUN uv pip install -r requirements.txt

# Copy application code
COPY server.py .

# Expose port (if needed for HTTP interface)
EXPOSE 8000

# Health check - use uv run for consistency
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD uv run python -c "import sys; sys.exit(0)"

# Run the MCP server
CMD ["uv", "run", "python", "server.py"]
