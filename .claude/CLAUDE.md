## Changelog

Almost all Giant Swarm projects use a `CHANGELOG.md` following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. Always add an entry under `## [Unreleased]` when making user-facing changes. Use the appropriate subsection: `Added`, `Changed`, `Fixed`, `Removed`, etc.

## CI/CD checks

The following check is performed in CorcleCI in the go-build step:

```bash
go install golang.org/x/tools/cmd/goimports@latest && \
if [[ -n $(goimports -local github.com/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME} -l .) ]];
then
  goimports -local github.com/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME} -d . && exit 1;
fi
```

To pass this check, perform the following command after any changes to Go files:

```bash
goimports -local github.com/giantswarm/search-mcp -w .
```

# Chart values and Schema

Run `make schema` after changes to `values.yaml` in the chart directory `helm/search-mcp`.

Never modify `values.schema.json` nor `README.md` in the chart directory.
