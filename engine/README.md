# Local Engine/CLI Service

This folder contains the Go `vibeengine` local engine and CLI.

Development rules:

- do not expose a local HTTP listener for normal hook capture
- write hook events to local SQLite and exit quickly
- keep local state outside user repositories
- do not touch the old `vibecode-dev-notify` helper runtime

Commands:

```bash
go test ./...
go build ./...
gofmt -w .
go run . status
go run . --version
```
