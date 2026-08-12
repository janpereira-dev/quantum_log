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

## Pull request review gate

Before opening or updating a pull request, request an independent Codex review (`@codex review`), address every material finding, and rerun the required checks. The pull-request template records that receipt; do not mark it complete while any review thread remains unresolved.
