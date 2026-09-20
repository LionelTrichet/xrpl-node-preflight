# Contributing

## Requirements

- Go 1.27.1, the pinned toolchain.
- Everything must pass before a change is proposed:

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

`make check` runs all of them and builds the binary.

## Rules

- The standard library is preferred. A new dependency needs a justification in the pull request; CLI, configuration and logging frameworks are out of scope.
- The runtime performs no external network request. Dependency downloads and `govulncheck` are development and CI concerns.
- Nothing is repaired automatically. This tool reports; the operator decides.
- No secret is ever printed, stored or derived from. Credential sections stay booleans.
- No shell execution, and no executable chosen by input.
- A new check needs a stable check ID, tests that fail without it, and a line in the README.
- Host-dependent behaviour is tested through the probe interface and fixtures, never against the machine running the suite.
- Scope expansion needs an issue first. Installation, monitoring, benchmarking, remote hosts and live XRPL checks are deliberately out of scope.

## Stable Check IDs

Check identifiers are a machine-readable contract once released. Renaming one is a breaking change and needs a report-schema decision, not a quiet edit.
