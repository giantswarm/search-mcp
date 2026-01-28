## Ci/CD checks

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
