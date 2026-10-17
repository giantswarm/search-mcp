# Tools

Generated from the registered tools: run `TOOLS_DOC_UPDATE=1 go test ./internal/search/ -run TestToolsDocIsCurrent` to update.

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
List every docs.giantswarm.io page with its title, description and link, as Markdown.
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
Read one documentation page from docs.giantswarm.io as Markdown; read_docs_index lists every page.
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
Search every Giant Swarm source, intranet included; for the public product documentation only, use search_docs.

Without sign-in, intranet results are left out and the answer says why.
```

```json
{
  "properties": {
    "breadcrumb_filter": {
      "default": [],
      "description": "Restrict to one section by its path, e.g. [\"docs\", \"support-and-ops\"].",
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
      "description": "Restrict to one source type, e.g. \"Intranet\", \"Documentation\" or \"Blog\".",
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
Search the public Giant Swarm documentation at docs.giantswarm.io; to include the intranet and blog, use search.
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
Find a legacy ops recipe in the Giant Swarm intranet; current runbooks are found with search_runbook.

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
Find an operations runbook in the Giant Swarm intranet; for a legacy ops recipe, use search_ops_recipe.

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
