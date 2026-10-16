# gitlab-integration
Integración con GitLab on-premise (URL + token personal) para gestionar MR/CI/merge según el `origin` de cada proyecto.
Nuevo slice `src/modules/gitlab/` (hexagonal). Rama: pi/integrar-con-gitlab-58e61df0-9c12-40ad-876a-9b1f89ac37e3.

## Specs
- S1: "Usando una URL de onpremise y un token de usuario de gitlab" → ajustes globales (URL base + token, + opción TLS inseguro para CA internas) persistidos en SQLite y editables en la pantalla de Configuración (token nunca se devuelve al cliente; solo "configurado: sí/no").
- S2: "gestionar los repositorios según origin de los proyectos git" → el proyecto GitLab se deduce de `git remote get-url origin` del workspace (https o ssh, con/sin `.git`); su host debe coincidir con el de la URL configurada, si no se informa claramente.
- S3: "crear MR" → desde una tarea (rama `branch` → `base_branch` del workspace): creación del MR vía API v4. La app NUNCA hace `git push` (L2): si la rama no existe en origin se informa claramente que hay que publicarla; si ya existe un MR abierto para esa rama se reutiliza (no duplicar).
- S4: "ver CI" → ver estado del MR y del último pipeline con sus jobs (nombre, stage, estado, enlace web).
- S5: "mezclar" → mergear el MR vía API (`PUT merge_requests/:iid/merge`); errores de GitLab (no mergeable, pipeline fallando, etc.) se muestran con su mensaje.

## Tasks
- [x] T1 (S1,S2) dominio + puertos + parser de remote + migración/schema/adaptador de ajustes + tests — inline — commit: feat(gitlab) (ver Log)
- [x] T2 (S3,S4,S5) cliente HTTP reqwest (adaptador `GitLabClientPort`) + tests contra servidor axum local — inline — commit: feat(gitlab) (ver Log)
- [x] T3 (S2) adaptador git: solo leer origin URL (sin push) + tests con repos temporales — inline — commit: feat(gitlab) (ver Log)
- [x] T4 (S1-S5) casos de uso + mocks.rs + tests unitarios — inline — commit: feat(gitlab) (ver Log)
- [x] T5 (S1-S5) DI context + app_state + DTOs + server fns + tests de mapeo — inline — commit: feat(gitlab) (ver Log)
- [x] T6 (S1,S3-S5) UI: tarjeta en Configuración + panel MR/CI en detalle de tarea — inline — commit: feat(gitlab) (ver Log)
- [x] T7 (all) fmt, clippy ssr/hydrate, tests; docs; verificación independiente (token/credenciales = riesgo alto) — commit: feat(gitlab) (ver Log)

## Log
- L1: "Usando una URL de onpremise y un token de usuario de gitlab quiero poder gestionar los repositorios según origin de los proyectos git y crear MR, ver CI y mezclar"
- L2: "Nada de push" (tras: "Porque quieres hacer git push?")
- L3: Evidencia: fmt --check ok; clippy ssr y hydrate -D warnings ok; `cargo test --features ssr --lib modules::gitlab` 51 ok; tests de integración gitlab_settings_repository_adapter_test (3) y git_cli_remote_adapter_test (3) ok; `diesel migration redo` ok en BD temporal. Verificación independiente: S1-S5 cumplen; hallazgo SSRF por redirección (token en cabecera PRIVATE-TOKEN) corregido con Policy::none + test; Debug manual de DTO/Input de guardado; MR abierto preferido sobre el último cerrado. Un solo commit por solapamiento de mod.rs/Cargo.toml entre tareas.
