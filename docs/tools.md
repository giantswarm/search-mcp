# Tools

Generated from the registered tools (`make tools-doc`). This is the stdio set; over HTTP without OAuth or forwarded tokens only these are offered: `read_docs_index`, `read_docs_url`, `read_handbook_url`, `search`, `search_docs`.

| Tool | Kind |
|---|---|
| `read_docs_index` | read-only |
| `read_docs_url` | read-only |
| `read_handbook_url` | read-only |
| `read_intranet_url` | read-only |
| `search` | read-only |
| `search_docs` | read-only |
| `search_ops_recipe` | read-only |
| `search_runbook` | read-only |

## `read_docs_index`

```text
List the documentation pages of docs.giantswarm.io (llms.txt) with title, description and link; no release notes.

All of it comes in one large answer: to find one page, search_docs is cheaper.
```

```json
{
  "properties": {},
  "required": [],
  "type": "object"
}
```

## `read_docs_url`

```text
Read one documentation page of docs.giantswarm.io as Markdown; find it with search_docs or read_docs_index.
```

```json
{
  "properties": {
    "url": {
      "description": "A page on docs.giantswarm.io, e.g. https://docs.giantswarm.io/some-page/",
      "type": "string"
    }
  },
  "required": [
    "url"
  ],
  "type": "object"
}
```

## `read_handbook_url`

```text
Read one page of the public Giant Swarm handbook (handbook.giantswarm.io) as Markdown.
```

```json
{
  "properties": {
    "url": {
      "description": "A page on handbook.giantswarm.io, e.g. https://handbook.giantswarm.io/docs/some-page/",
      "type": "string"
    }
  },
  "required": [
    "url"
  ],
  "type": "object"
}
```

## `read_intranet_url`

```text
Fetch one page of the Giant Swarm intranet (intranet.giantswarm.io) as Markdown.

Requires sign-in.
```

```json
{
  "properties": {
    "url": {
      "description": "A page on intranet.giantswarm.io, e.g. https://intranet.giantswarm.io/docs/some-page/",
      "type": "string"
    }
  },
  "required": [
    "url"
  ],
  "type": "object"
}
```

## `search`

```text
Search Giant Swarm docs, release notes, handbook, blog posts and, when signed in, the intranet; docs only: search_docs.

Without sign-in, intranet results are left out and the answer says why. Release notes (changelogs) rank last and blog posts low: to search release notes, set breadcrumb_filter ["changes"].
```

```json
{
  "properties": {
    "breadcrumb_filter": {
      "default": [],
      "description": "Section path from the top level, as a result's Breadcrumb shows it, e.g. [\"changes\"] for release notes.",
      "items": {
        "type": "string"
      },
      "type": "array"
    },
    "require_all_terms": {
      "default": false,
      "description": "Only pages containing every word (strict AND). Leave false for longer queries ranked by relevance.",
      "type": "boolean"
    },
    "size": {
      "default": 10,
      "description": "Number of results to return.",
      "type": "integer"
    },
    "start_index": {
      "default": 0,
      "description": "Offset of the first result, to page through results.",
      "type": "integer"
    },
    "term": {
      "description": "What to search for. Results are ranked by similarity; require_all_terms keeps only pages with every word.",
      "type": "string"
    },
    "type_filter": {
      "default": "",
      "description": "Restrict to one source type, exact case: \"Documentation\", \"Handbook\", \"Blog\" or \"Intranet\".",
      "type": "string"
    }
  },
  "required": [
    "term"
  ],
  "type": "object"
}
```

## `search_docs`

```text
Search only the public documentation at docs.giantswarm.io; for every other source, use search.

Release notes (changelogs) are included but rank last: to search them, use search with breadcrumb_filter ["changes"].
```

```json
{
  "properties": {
    "require_all_terms": {
      "default": false,
      "description": "Only pages containing every word (strict AND). Leave false for longer queries ranked by relevance.",
      "type": "boolean"
    },
    "size": {
      "default": 10,
      "description": "Number of results to return.",
      "type": "integer"
    },
    "start_index": {
      "default": 0,
      "description": "Offset of the first result, to page through results.",
      "type": "integer"
    },
    "term": {
      "description": "What to search for. Results are ranked by similarity; require_all_terms keeps only pages with every word.",
      "type": "string"
    }
  },
  "required": [
    "term"
  ],
  "type": "object"
}
```

## `search_ops_recipe`

```text
Retired: the intranet's ops-recipes section is gone, so this finds nothing; search runbooks with search_runbook.

Requires sign-in.
```

```json
{
  "properties": {
    "require_all_terms": {
      "default": false,
      "description": "Only pages containing every word (strict AND). Leave false for longer queries ranked by relevance.",
      "type": "boolean"
    },
    "size": {
      "default": 10,
      "description": "Number of results to return.",
      "type": "integer"
    },
    "start_index": {
      "default": 0,
      "description": "Offset of the first result, to page through results.",
      "type": "integer"
    },
    "term": {
      "description": "What to search for. Results are ranked by similarity; require_all_terms keeps only pages with every word.",
      "type": "string"
    }
  },
  "required": [
    "term"
  ],
  "type": "object"
}
```

## `search_runbook`

```text
Find an operations runbook in the Giant Swarm intranet.

Requires sign-in.
```

```json
{
  "properties": {
    "require_all_terms": {
      "default": false,
      "description": "Only pages containing every word (strict AND). Leave false for longer queries ranked by relevance.",
      "type": "boolean"
    },
    "size": {
      "default": 10,
      "description": "Number of results to return.",
      "type": "integer"
    },
    "start_index": {
      "default": 0,
      "description": "Offset of the first result, to page through results.",
      "type": "integer"
    },
    "term": {
      "description": "What to search for. Results are ranked by similarity; require_all_terms keeps only pages with every word.",
      "type": "string"
    }
  },
  "required": [
    "term"
  ],
  "type": "object"
}
```
