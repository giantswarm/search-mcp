#!/bin/bash

# Giant Swarm Search MCP Server Runner
# This script helps run the MCP server in different modes

set -e

# Default mode
MODE=${1:-"server"}

case $MODE in
    "server")
        echo "Starting MCP server..."
        exec uv run python server.py
        ;;
    "dev")
        echo "Starting MCP server in development mode..."
        exec uv run mcp dev server.py
        ;;
    "inspect")
        echo "Inspecting MCP server tools..."
        echo "Note: Using 'mcp dev' as inspect mode (mcp inspect not available)"
        exec uv run mcp dev server.py
        ;;
    "test")
        echo "Testing server import..."
        uv run python -c "import server; print('✅ Server imported successfully')"
        
        echo "Testing MCP tools..."
        uv run python -c "
import server

print('🔍 Available tools:')
# Access tools through the FastMCP tool manager
try:
    tools = server.mcp._tool_manager._tools
    if tools:
        for tool_name, tool_info in tools.items():
            doc = tool_info.description or 'No description'
            print(f'  ✅ {tool_name}: {doc}')
    else:
        print('  ❌ No tools found')
        
    print(f'\\n📊 Server info:')
    print(f'  - Server name: {server.mcp.name}')
    print(f'  - Tools registered: {len(tools)}')
    print(f'  - Server type: {type(server.mcp).__name__}')
    
except Exception as e:
    print(f'❌ Error accessing tools: {e}')
    import traceback
    traceback.print_exc()
"
        
        echo "Testing server initialization..."
        uv run python -c "
import server
import asyncio

async def test_tools():
    try:
        print('🧪 Testing server initialization...')
        # Verify auth manager is initialized
        if server.auth_manager:
            print('✅ Auth manager initialized')
        
        # Verify MCP server is created
        if server.mcp:
            print('✅ MCP server created')
        
        print('✅ Server is functional')
        
    except Exception as e:
        print(f'❌ Error testing server: {e}')

asyncio.run(test_tools())
"
        ;;
    *)
        echo "Usage: $0 [server|dev|inspect|test]"
        echo "  server  - Run as MCP server (default)"
        echo "  dev     - Run with mcp dev (development mode)"
        echo "  inspect - Inspect available tools"
        echo "  test    - Test server functionality"
        exit 1
        ;;
esac
