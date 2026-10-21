# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Over streamable HTTP, `FORWARDED_TOKEN_ISSUER_URL` and `FORWARDED_TOKEN_AUDIENCE` make a shared server reach the intranet as each caller: it takes the bearer token an MCP gateway forwards with the request, verifies its signature, issuer, audience and expiry, and uses it for that request only, never storing it. The intranet tools are advertised in this mode; a call without a valid token gets an authentication error. Stdio and the `OAUTH_*` login are unchanged.

### Changed

- The tool descriptions say what each tool does today ([giantswarm#38592](https://github.com/giantswarm/giantswarm/issues/38592)): `search` names its sources, mentions the intranet only where a sign-in is configured, and says how to reach release notes, which rank last (`breadcrumb_filter ["changes"]`); `search_docs` takes `breadcrumb_filter` too, so it can reach them; `type_filter` names its exact values, and `breadcrumb_filter` takes one element per segment of a result's breadcrumb; `read_docs_index` lists the documentation pages without release notes, in one large answer. Tests hold every tool to read-only annotations and keep #111's similarity ranking as the default; `make tools-doc` regenerates `docs/tools.md`.
- Build with Go 1.27.3 (`toolchain go1.27.3`), which fixes GO-2026-6809 in the standard library.
- The browser pages of the OAuth login and the device flow carry the Giant Swarm design, from [pagekit](https://github.com/giantswarm/pagekit): the logo, light and dark mode, and no external requests. Errors that were plain text (a failed `/oauth/login` or `/callback`) are pages too, and they no longer show the internal error, which stays in the log. A refusal at the identity provider (`?error=access_denied`) says so instead of reporting a missing code, and is logged with the provider's error. The device flow's cookie is cleared whatever the callback's outcome, so a later sign-in in the same browser is no longer taken for a device flow. The device-code entry form is unchanged for now.
- Over streamable HTTP without OAuth configured, the intranet tools (`search_runbook`, `read_intranet_url`) are not advertised: a shared server without a login could never serve them. Over stdio they stay and explain how to configure OAuth.

### Removed

- `search_ops_recipe`: the intranet's ops-recipes section is gone (the recipes moved to runbooks), so it found nothing; search runbooks with `search_runbook` ([#233](https://github.com/giantswarm/search-mcp/issues/233)).

### Fixed

- The error an intranet tool returns when OAuth is not configured shows its line breaks instead of a literal `\n\n`, and `read_docs_url`'s error for a URL outside docs.giantswarm.io calls that site the Giant Swarm documentation instead of the handbook ([#234](https://github.com/giantswarm/search-mcp/issues/234)).
- With the `OAUTH_*` login, the intranet tools' sign-in errors link the server's own `/oauth/login`, built from `--http-addr`, once per message. Before, they always linked `http://localhost:8080/oauth/login`; over stdio, the login, callback and device-flow links used port 8080 whatever `--http-addr` said; a server listening on `0.0.0.0` or `[::]` linked that address instead of `localhost`; and an expired token that failed to refresh gave an error without a link.
- The MCP endpoint refuses a request body larger than 1 MiB before mcp-go reads it whole (CVE-2026-108859, no fixed mcp-go release yet; the scan ignores it until 2026-11-12) ([#](https://github.com/giantswarm/search-mcp/issues/)).
- `search` includes intranet results for an authenticated caller: it uses the forwarded token or the `OAUTH_*` login, as the intranet tools do. Before, it never sent a token, so it only ever returned public results, and a `type_filter` of `Intranet` returned none. A caller without a usable token still gets the public results, and the result says why intranet results are left out.
- The chart's `env` and `envFrom` values validate against the Kubernetes `EnvVar` and `EnvFromSource` schemas; before, the schema rejected every entry, so no environment variable could be set.
- Helm chart: installs with its default values. The route is off by default (`route.enabled: false`), `route.hostnames` may be empty while it is off, and an enabled route without a hostname fails the render with a clear message instead of matching every host on the Gateway. Before, the schema refused the defaults and every in-cluster deployment without a route.
- Helm chart: the `helm.sh/chart` label is valid for any chart version. A long version (a branch build, or the `<version>+<digest>` that helm-controller installs) was cut at 63 characters onto a trailing `.`, `_` or `-`, and the API server refused every object carrying the label.

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
