# reset-task — Restablecer tarea

Acción "Restablecer" en la pantalla de la tarea: detiene el agente, elimina worktree y rama, borra eventos/diálogos y deja la tarea en `Backlog` (rama/worktree vaciados), lista para arrancar de nuevo. No permitido en `Backlog`, `Merged` ni `Finishing`.

- [x] 1. Dominio: `TaskStatus::can_reset` + puerto `TaskRepositoryPort::reset_to_backlog` (borra eventos, limpia branch/worktree/base/error/diálogos/subagentes, status=backlog) + mocks
- [x] 2. Adaptador Diesel + retrying wrapper + test de integración
- [x] 3. Caso de uso `ResetTask` + tests unitarios + DI
- [x] 4. Server fn `reset_task`, `can_reset` en `TaskActionsDto` + tests
- [x] 5. UI: botón "Restablecer" en `TaskActions`, `TaskDetailState.reset`
- [x] 6. fmt, clippy ssr/hydrate, tests
