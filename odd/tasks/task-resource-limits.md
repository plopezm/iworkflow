# task-resource-limits

Limit RAM and CPU of each task's agent process via `systemd-run --user --scope` (cgroups). Linux-only,
best effort: without systemd-run (or off Linux) the agent starts unlimited. Global default in agent
settings plus per-task override (task value wins).

## Specs
- S1: "Limitar recursos por tarea" — limits apply per task.
- S2: Resources: memory (RAM) and CPU. Mechanism: systemd-run --user (cgroups).
- S3: Configuration: "Ambos" — global default (agent settings) and per-task override.
- S4: Not Linux/systemd: agent launches without limits (no failure); Windows is out of scope.

## Tasks
- [x] T1 (S2,S4) ResourceLimits value object, LaunchOptions.limits, systemd-run wrapping in pi runtime adapter + tests
- [x] T2 (S3) Global default: migration agent_settings, domain/repo/DTO/UI, mapped into LaunchOptions
- [x] T3 (S1,S3) Per-task override: migration tasks, Task model, create/edit DTO + form, resolution over global
- [x] T4 fmt, clippy ssr/hydrate, tests

## Log
- L1: Limitar recursos por tarea
- L2: user chose RAM + CPU, systemd-run --user (cgroups), config "Ambos"; asked about Windows -> best effort, no limits
- T1 commit: fea5e76
- T2 commit: a6d4722
- T3 commit: dcaa22d
- L3: user: "En configuración quiero poder deshabilitarlo, es decir, que la limitación sea opcional" -> master switch limits_disabled in agent settings (ignores global and per-task limits)
- L4: user: "no, quiero un nuevo menú que se llame configuración. Ponlo en esa nueva pantalla" -> /settings screen "Configuración" holds the limits switch + RAM/CPU; removed from sidebar bar; AgentSettingsState shared via context
