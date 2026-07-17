# Contributing

Keep domain logic independent of Cobra, SQLite, providers, and TUI. Add an intention-relevant failing test before production behavior, then run focused tests and the full suite.

Required local checks:

```bash
go test ./...
go test -race ./...
go vet ./...
gofmt -w .
```

Do not commit databases, generated binaries, credentials, or data presented as real usage. Preserve `unattributed` rather than guessing ownership.
