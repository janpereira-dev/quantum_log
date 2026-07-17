# ADR-002: Attribute events to active project context

Status: accepted

`Project`, `ProjectLocation`, and `WorkContext` remain separate. Resolver precedence is explicit argument, `QLOG_PROJECT`, CWD, Git root, registered path, then `unattributed`. Provider, model, and agent names never determine project ownership.
