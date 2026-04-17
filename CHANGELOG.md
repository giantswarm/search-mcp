# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-04-17

### Added

- Added MCP tool annotations (`readOnlyHint`, `openWorldHint`) to all tools.

### Changed

- All search tools no longer use boolean `AND` query logic. The old behaviour can be activated setting the `require_all_terms` parameter to true.

### Fixed

- `read_docs_url`: strip site navigation and footer from returned content by matching the actual Hugo template classes (`.base-content` for single pages, `.list-content` for index pages). The previous selector matched neither template, causing chrome to leak into responses.

## [0.2.0] - 2026-01-30

### Added

- Added tools for public documentation access: `search_docs`, `read_docs_index`, `read_docs_url`.
- Added metrics collection for tool usage.
- Added more logging
- Add Helm chart

## [0.1.0] - 2026-01-15

### Added

- Added OAuth authentication support

### Changed

- Rewritten in Go
- Work with OpenSearch v3.2 as a backend instead of Elasticsearch v6.8
- Let searches return 10 items instead of 30 by default

### Removed

- Removed `INTRANET_SESSION_COOKIE` authentication

## [0.0.1] - 2025-10-17

### Added

- Initial version

[Unreleased]: https://github.com/giantswarm/search-mcp/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/giantswarm/search-mcp/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/search-mcp/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/giantswarm/search-mcp/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/giantswarm/search-mcp/releases/tag/v0.0.1
