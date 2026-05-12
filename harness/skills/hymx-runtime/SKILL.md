---
name: hymx-runtime
description: Use vmdocker workspace-scoped runtime paths.
---

Use `VMDOCKER_RUNTIME_WORKSPACE`, `VMDOCKER_AGENT_WORKSPACE`, and related vmdocker environment variables for runtime files. Do not place agent-visible state under fixed system paths such as `/usr/local` or `/app`.
