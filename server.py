# server.py
import os
import asyncio
import json
import logging
from pathlib import Path
from mcp.server.fastmcp import FastMCP
import aiohttp
from markdownify import markdownify as md
from urllib.parse import quote_plus

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Create an MCP server for search public and internal docs
mcp = FastMCP("giantswarm-search")

class AuthManager:
    """Manages authentication for OAuth2 proxy protected resources"""
    
    def get_auth_headers(self):
        """Get authentication headers/cookies for requests"""
        logger.debug("Getting authentication configuration...")
        
        auth_cookie = os.getenv('INTRANET_SESSION_COOKIE')
        if auth_cookie:
            logger.debug("Using session cookie from environment variable")
            return {"cookies": {"_oauth2_proxy": auth_cookie}}
        
        logger.debug("No authentication configured - using public access")
        return {}
    
    def is_authenticated(self):
        """Check if authentication is available"""
        return bool(os.getenv('INTRANET_SESSION_COOKIE'))

# Initialize auth manager
auth_manager = AuthManager()

async def _read_url_content(url: str, url_description: str) -> dict:
    """
    Helper function to read content from a URL.
    
    Uses authentication if available, but also works for public URLs without authentication.
    
    Args:
        url: The URL to fetch content from
        url_description: Description of the URL type (e.g., "Intranet", "Handbook")
    
    Returns:
        dict: MCP tool response with content or error message
    """
    # Get authentication configuration (if available)
    auth_config = auth_manager.get_auth_headers()
    
    try:
        # Create session with authentication
        connector = aiohttp.TCPConnector(ssl=False)  # For development, consider removing for production
        async with aiohttp.ClientSession(connector=connector) as session:
            
            # Prepare request parameters
            request_kwargs = {"url": url}
            
            # Add authentication
            if "headers" in auth_config:
                request_kwargs["headers"] = auth_config["headers"]
            if "cookies" in auth_config:
                request_kwargs["cookies"] = auth_config["cookies"]
            
            async with session.get(**request_kwargs) as response:
                # Get response text
                response_text = await response.text()
                
                # Log to console for server-side debugging
                logger.debug(f"Response status: {response.status}")
                logger.debug(f"Content-Type: {response.headers.get('content-type', 'unknown')}")
                
                # Check if we got redirected to login page (only check for actual auth redirects, not regular HTML content)
                if "Sign in to GitHub" in response_text or response.status == 401:
                    logger.warning("Authentication required - redirected to login or got 401")
                    return {
                        "content": [{"type": "text", "text": "❌ Authentication failed or expired. Please update your INTRANET_SESSION_COOKIE environment variable with a fresh cookie value."}]
                    }
                
                # Check for successful response
                if response.status == 404:
                    return {
                        "content": [{"type": "text", "text": f"❌ Page not found: {url}"}]
                    }
                elif response.status != 200:
                    logger.error(f"Non-200 response: {response.status} - {response.reason}")
                    return {
                        "content": [{"type": "text", "text": f"❌ HTTP {response.status}: {response.reason}. Please check the URL or try again."}]
                    }
                
                # Convert HTML to markdown for better readability
                try:
                    # Check if it's HTML content
                    content_type = response.headers.get('content-type', '').lower()
                    if 'html' in content_type:
                        # Parse HTML and remove unwanted elements before converting to markdown
                        from bs4 import BeautifulSoup
                        
                        soup = BeautifulSoup(response_text, 'html.parser')
                        
                        # Remove the sidebar element with class 'td-sidebar'
                        sidebar = soup.find('aside', class_='td-sidebar')
                        if sidebar:
                            sidebar.decompose()
                            logger.debug("Removed td-sidebar element from HTML")
                        
                        # Remove all script elements
                        scripts = soup.find_all('script')
                        for script in scripts:
                            script.decompose()
                        if scripts:
                            logger.debug(f"Removed {len(scripts)} script elements from HTML")
                        
                        # Convert cleaned HTML to markdown
                        cleaned_html = str(soup)
                        markdown_content = md(cleaned_html, 
                                            heading_style="ATX",
                                            bullets="-",
                                            strip=['script', 'style'])
                        
                        # Clean up excessive whitespace
                        lines = markdown_content.split('\n')
                        cleaned_lines = []
                        prev_empty = False
                        
                        for line in lines:
                            line = line.rstrip()
                            is_empty = len(line) == 0
                            
                            # Skip multiple consecutive empty lines
                            if is_empty and prev_empty:
                                continue
                                
                            cleaned_lines.append(line)
                            prev_empty = is_empty
                        
                        content = '\n'.join(cleaned_lines).strip()
                        
                        return {
                            "content": [{"type": "text", "text": f"# Content from {url}\n\n{content}"}]
                        }
                    else:
                        # Return raw content for non-HTML
                        return {
                            "content": [{"type": "text", "text": f"# Content from {url}\n\n{response_text}"}]
                        }
                        
                except Exception as e:
                    logger.error(f"Error processing content: {e}")
                    # Fallback to raw HTML if markdown conversion fails
                    return {
                        "content": [{"type": "text", "text": f"# Content from {url}\n\n{response_text}"}]
                    }
            
    except Exception as error:
        logger.exception(f"Exception occurred while reading URL: {error}")
        return {
            "content": [{"type": "text", "text": f"Error accessing {url}: {str(error)}"}]
        }

@mcp.tool()
async def search(term: str, start_index: int = 0, size: int = 30, type_filter: str = "", breadcrumb_filter: list = []) -> dict:
    """
    Search public and internal Giant Swarm documentation. To paginate through results, use a non-zero start_index.
    
    Without authentication, searches only public documentation sources using the public endpoint.
    With authentication, searches both public and internal (intranet) documentation.

    Args:
        term: The search term (required)
        start_index: The start index of the search results (optional, defaults to 0)
        size: The size of the search results (optional, defaults to 30)
        type_filter: To restrict the results to only one source type, e. g. "Intranet" (optional, defaults to "")
        breadcrumb_filter: To restrict the results to a specific section, e. g. ["docs", "support-and-ops"] (optional, defaults to [])
    """
    
    # Check if user is authenticated
    is_authenticated = auth_manager.is_authenticated()
    
    # Select endpoint based on authentication status
    if is_authenticated:
        url = "https://intranet.giantswarm.io/searchapi/"
        logger.info(f"Using authenticated endpoint: {url}")
    else:
        url = "https://docs.giantswarm.io/searchapi/"
        logger.info(f"Using public endpoint: {url}")
        
        # If user tries to filter for Intranet content without authentication, inform them
        if type_filter == "Intranet":
            return {
                "content": [{"type": "text", "text": "❌ Cannot search Intranet resources without authentication. Please set the INTRANET_SESSION_COOKIE environment variable or search without the Intranet filter."}]
            }

    # Build POST request payload using Elasticsearch v6.8.x syntax
    base_query = {
        "function_score": {
            "query": {
                "simple_query_string": {
                    "fields": ["title^5", "uri^5", "description^5", "text"],
                    "default_operator": "AND",
                    "query": term,
                },
            },
            "functions": [
                {"filter": {"term": {"type": "Intranet"}}, "weight": 10},
                {"filter": {"term": {"type": "Blog"}}, "weight": 0.01},
                {"filter": {"term": {"breadcrumb_1": "changes"}}, "weight": 0.0001},
                {"filter": {"term": {"breadcrumb_1": "api"}}, "weight": 0.0001},
            ]
        }
    }
    
    # Apply filters if specified
    must_clauses = [base_query]
    
    if type_filter != "":
        # Keep using term query for type field (it works)
        must_clauses.append({"term": {"type": type_filter}})
    
    if breadcrumb_filter != []:
        # Use match queries for breadcrumb fields (they are analyzed)
        n = 1
        for breadcrumb in breadcrumb_filter:
            must_clauses.append({"match": {f"breadcrumb_{n}": breadcrumb}})
            n += 1
    
    if len(must_clauses) > 1:
        query = {
            "bool": {
                "must": must_clauses
            }
        }
    else:
        query = base_query
    
    payload = {
        "from": start_index,
        "size": size,
        "sort": ["_score"],
        "_source": {"excludes": ["text", "body"]},
        "query": query,
        "highlight": {
            "fields": {
                "body": {"type": "unified", "number_of_fragments": 1, "no_match_size": 200, "fragment_size": 150},
                "title": {"type": "unified", "number_of_fragments": 1},
            }
        },
    }
    
    # Debug logging
    logger.debug(f"Search parameters - term: '{term}', type_filter: '{type_filter}', breadcrumb_filter: {breadcrumb_filter}")
    logger.debug(f"Must clauses count: {len(must_clauses)}")
    logger.debug(f"Search query payload: {json.dumps(payload, indent=2)}")
    
    # Get authentication configuration (only if authenticated)
    auth_config = auth_manager.get_auth_headers() if is_authenticated else {}
    
    try:
        # Create session with or without authentication
        connector = aiohttp.TCPConnector(ssl=False)  # For development, consider removing for production
        async with aiohttp.ClientSession(connector=connector) as session:
            
            # Prepare request parameters
            request_kwargs = {"url": url}
            
            # Add authentication only if available
            if is_authenticated:
                if "headers" in auth_config:
                    request_kwargs["headers"] = auth_config["headers"]
                if "cookies" in auth_config:
                    request_kwargs["cookies"] = auth_config["cookies"]
            request_kwargs["json"] = payload
            
            async with session.post(**request_kwargs) as response:
                # Get response text first for debugging
                response_text = await response.text()
                
                # Log to console for server-side debugging
                logger.debug(f"Response status: {response.status}")
                logger.debug(f"Raw response body: {response_text}")
                
                # Try to parse as JSON
                try:
                    response_json = json.loads(response_text)
                    logger.debug(f"Parsed JSON response: {json.dumps(response_json, indent=2)}")
                except json.JSONDecodeError as e:
                    logger.error(f"Failed to parse JSON response: {e}")
                    logger.error(f"Response text (first 500 chars): {response_text[:500]}")
                    return {
                        "content": [{"type": "text", "text": f"❌ Invalid JSON response from server.\n\nResponse status: {response.status}\nResponse preview: {response_text[:200]}"}]
                    }
                
                # Check if we got redirected to login page (search endpoint should return JSON, not HTML)
                if "Sign in to GitHub" in response_text or response.status == 401:
                    logger.warning("Authentication required - redirected to login or got 401")
                    if is_authenticated:
                        return {
                            "content": [{"type": "text", "text": f"❌ Authentication expired. Your INTRANET_SESSION_COOKIE has expired.\n\nTo search public documentation, remove the INTRANET_SESSION_COOKIE environment variable and try again.\n\nTo continue accessing intranet resources, update INTRANET_SESSION_COOKIE with a fresh cookie value."}]
                        }
                    else:
                        return {
                            "content": [{"type": "text", "text": f"❌ Authentication required for this resource. Please set the INTRANET_SESSION_COOKIE environment variable."}]
                        }
                
                # For search endpoint, if we got HTML instead of JSON, it might be an auth redirect
                if response_text.strip().startswith('<'):
                    logger.warning("Got HTML response instead of JSON - likely authentication redirect")
                    if is_authenticated:
                        return {
                            "content": [{"type": "text", "text": f"❌ Authentication expired. Your INTRANET_SESSION_COOKIE has expired.\n\nTo search public documentation, remove the INTRANET_SESSION_COOKIE environment variable and try again.\n\nTo continue accessing intranet resources, update INTRANET_SESSION_COOKIE with a fresh cookie value."}]
                        }
                    else:
                        return {
                            "content": [{"type": "text", "text": f"❌ Got HTML response instead of expected JSON. The search endpoint may be unavailable or requires authentication."}]
                        }
                
                # Check for successful response
                if response.status != 200:
                    logger.error(f"Non-200 response: {response.status} - {response.reason}")
                    return {
                        "content": [{"type": "text", "text": f"❌ HTTP {response.status}: {response.reason}. Please try again."}]
                    }
        
        # Process JSON response
        logger.debug("Processing search results...")
        
        try:
            hits = response_json.get("hits", {}).get("hits", [])
            total_hits = response_json.get('hits', {}).get('total', 0)
            logger.debug(f"Found {len(hits)} search results out of {total_hits} total hits")
            
            # Log first few hits for debugging
            if hits:
                for i, hit in enumerate(hits[:3]):
                    source = hit.get('_source', {})
                    logger.debug(f"Hit {i+1}: type='{source.get('type', 'N/A')}', breadcrumb_1='{source.get('breadcrumb_1', 'N/A')}', title='{source.get('title', 'N/A')}'")
            
            body = f"# Search results for {term}\n\n"
            if not is_authenticated:
                body += "ℹ️ **Note:** Searching public documentation only. For intranet access, set the INTRANET_SESSION_COOKIE environment variable.\n\n"
            body += f"Showing {len(hits)} out of {total_hits} search results"
            if start_index > 0:
                body += f", starting at {start_index + 1}"
            body += "\n\n"

            n = start_index + 1

            for i, hit in enumerate(hits):
                source = hit.get('_source', {})
                title = source.get('title', '')
                description = source.get('description', '')
                uri = source.get('url', 'No URL')
                logger.debug(f"Processing hit {i+1}: {title}")
                body += f"{n}. **[{title}]({uri})**\n"
                body += f"   **Type:** {source.get('type', 'Unknown')}\n"
                body += f"   **Breadcrumb:** {' / '.join(source.get('breadcrumb', []))}\n"

                if description != "":
                    body += f"   **Description:** {description}\n"
                
                excerpt = hit.get('highlight', {}).get('body', [''])[0]
                if excerpt:
                    body += f"   **Excerpt:** {excerpt}\n"

                body += "\n"
                n += 1
            
            logger.debug(f"Final response body length: {len(body)} characters")
            
            return {
                "content": [{"type": "text", "text": body}]
            }
        except KeyError as e:
            logger.error(f"Missing expected key in response: {e}")
            logger.error(f"Response structure: {list(response_json.keys()) if isinstance(response_json, dict) else 'Not a dict'}")
            return {
                "content": [{"type": "text", "text": f"❌ Unexpected response format. Missing key: {e}"}]
            }
            
    except Exception as error:
        logger.exception(f"Exception occurred during search: {error}")
        return {
            "content": [{"type": "text", "text": f"Error accessing {url}: {str(error)}"}]
        }

@mcp.tool()
async def search_runbook(term: str, start_index: int = 0, size: int = 30) -> dict:
    """
    Search for DevOps runbooks in the Giant Swarm intranet.
    
    Args:
        term: The search term (required)
        start_index: The start index of the search results (optional, defaults to 0)
        size: The size of the search results (optional, defaults to 30)
    """
    
    # Check authentication early
    if not auth_manager.is_authenticated():
        return {
            "content": [{"type": "text", "text": "❌ This tool requires authentication.\n\nRunbooks are internal intranet resources. Please set the INTRANET_SESSION_COOKIE environment variable."}]
        }

    return await search(term,
                        start_index,
                        size,
                        breadcrumb_filter=["support-and-ops", "runbooks"])

@mcp.tool()
async def search_ops_recipe(term: str, start_index: int = 0, size: int = 30) -> dict:
    """
    Search for Ops Recipes (legacy runbooks) in the Giant Swarm intranet.
    
    Args:
        term: The search term (required)
        start_index: The start index of the search results (optional, defaults to 0)
        size: The size of the search results (optional, defaults to 30)
    """
    
    # Check authentication early
    if not auth_manager.is_authenticated():
        return {
            "content": [{"type": "text", "text": "❌ This tool requires authentication.\n\nOps Recipes are internal intranet resources. Please set the INTRANET_SESSION_COOKIE environment variable."}]
        }

    return await search(term,
                        start_index,
                        size,
                        breadcrumb_filter=["support-and-ops", "ops-recipes"])

@mcp.tool()
async def read_intranet_url(url: str) -> dict:
    """
    Read content from a single URL on the Giant Swarm intranet using authenticated session.
    
    Args:
        url: The URL to fetch content from (e.g., https://intranet.giantswarm.io/docs/some-page/)
    """
    
    # Check authentication early
    if not auth_manager.is_authenticated():
        return {
            "content": [{"type": "text", "text": "❌ This tool requires authentication.\n\nThe intranet requires authentication via the INTRANET_SESSION_COOKIE environment variable."}]
        }
    
    # Validate URL is from Giant Swarm intranet
    if not url.startswith("https://intranet.giantswarm.io/"):
        return {
            "content": [{"type": "text", "text": "❌ URL must be from the Giant Swarm intranet (https://intranet.giantswarm.io/)."}]
        }
    
    return await _read_url_content(url, "Intranet")

@mcp.tool()
async def read_handbook_url(url: str) -> dict:
    """
    Read content from a single URL on the Giant Swarm handbook (public, no authentication required).
    
    Args:
        url: The URL to fetch content from (e.g., https://handbook.giantswarm.io/docs/some-page/)
    """
    
    # Validate URL is from Giant Swarm handbook
    if not url.startswith("https://handbook.giantswarm.io/"):
        return {
            "content": [{"type": "text", "text": "❌ URL must be from the Giant Swarm handbook (https://handbook.giantswarm.io/)."}]
        }
    
    return await _read_url_content(url, "Handbook")


if __name__ == "__main__":
    # Run the MCP server
    mcp.run()