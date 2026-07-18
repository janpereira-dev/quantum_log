---
name: quantum-log-usage-governor
description: Govern QUANTUM_LOG project context, task summaries, unattributed repair, and usage alerts before relevant agent work.
---

1. Start server with `qlog mcp serve`; it uses stdio and requires `qlog init` first.
2. Resolve project before work with `get_current_project`. Never infer it from provider, model, or agent name.
3. Register unknown workspaces with `register_project`; call `switch_project` when session moves repositories. It records a work context, not process-global state.
4. Start work with `start_task`; call `finish_task` and retain returned recorded task summary. Use `qlog task summary <task-id> --json` outside MCP.
5. Use `get_project_summary` for recorded usage, tags, active tasks, and current budget alerts.
6. Preserve unknown ownership as `unattributed`. Use `get_unattributed_summary`, then explicit `assign_usage` or `split_usage`; split shares must total 10000 basis points.
7. Do not record prompt, response, secret, authorization, or tool content by default.
8. Do not claim token, model, cost, or project metadata unavailable from a real source.
9. Budgets are monthly allocated-cost alerts only. They never block work or prove provider spend.
