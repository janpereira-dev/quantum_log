---
name: quantum-log-usage-governor
description: Resolve QUANTUM_LOG project context before relevant agent work.
---

1. Resolve project before work; never infer it from provider, model, or agent name.
2. Prefer explicit project context and record evidence/confidence.
3. Detect CWD, Git root, workspace, and project switches during a session.
4. Preserve unknown ownership as `unattributed`.
5. Do not record prompt, response, secret, authorization, or tool content by default.
6. Do not claim token, model, cost, or project metadata unavailable from a real source.
