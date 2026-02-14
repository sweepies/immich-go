# Contributing

Thanks for your interest in improving `immich-go`.

## Development Baseline

- Go 1.26 or newer is required (see `go.mod`).
- Use standard Go tooling (`go build`, `go test`) from the repository root.

## Local Workflow

```bash
go test -race -v -count=1 -coverprofile=coverage.out ./...
go build ./cmd/immich-go
```

## User-Facing Changes and Release Notes

We use [changie](https://github.com/miniscruff/changie) for release notes.

If your change is user-facing and affects the core application (not docs-only), add an unreleased changelog entry:

```bash
changie new -k <Added|Changed|Deprecated|Removed|Fixed|Security> -b "<conventional-commit-style message>"
```

This creates a file under `.changes/unreleased/`.

## Release Pipeline Compatibility

- `.github/workflows/release.yml` installs Go from `go.mod` (`go-version-file`) and runs GoReleaser with changie-generated release notes.
- `.goreleaser.yaml` is the source of truth for build and archive settings.

When adjusting contributor workflows, keep those files compatible with the commands above.
