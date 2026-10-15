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
- [ ] T2 (S3) Global default: migration agent_settings, domain/repo/DTO/UI, mapped into LaunchOptions
- [ ] T3 (S1,S3) Per-task override: migration tasks, Task model, create/edit DTO + form, resolution over global
- [ ] T4 fmt, clippy ssr/hydrate, tests

## Log
- L1: Limitar recursos por tarea
- L2: user chose RAM + CPU, systemd-run --user (cgroups), config "Ambos"; asked about Windows -> best effort, no limits
