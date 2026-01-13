package project

var (
	description = "MCP server for searching Giant Swarm documentation"
	gitSHA      = "n/a"
	name        = "search-mcp"
	source      = "https://github.com/giantswarm/search-mcp"
	version     = "0.0.2-dev"
)

func Description() string {
	return description
}

func GitSHA() string {
	return gitSHA
}

func Name() string {
	return name
}

func Source() string {
	return source
}

func Version() string {
	return version
}
