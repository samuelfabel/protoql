# Contributing

Thanks for your interest in ProtoQL.

## Workflow

1. Open an issue or pick an existing milestone before large changes.
2. Use branch names: `{type}/{id}-{short-description}` (e.g. `feat/f1-01-query-compiler`).
3. Write commit messages in English using [Conventional Commits](https://www.conventionalcommits.org/).
4. Fill out the pull request template completely.

## Tests

```bash
go test ./...
```

All tests must pass before merge.

## Scope

This project is an incremental feasibility study. Keep pull requests focused on a single concern.
