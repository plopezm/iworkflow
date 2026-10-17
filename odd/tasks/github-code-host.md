# github-code-host
Un workspace puede ser un repo de GitHub o de GitLab: se detecta el proveedor por el `origin`, se aplica su configuración y se gestionan PR/MR, CI y merge con el mismo flujo.
El slice `gitlab` pasa a `code_hosting` (puerto neutro + factoría de clientes); GitLab y GitHub son adaptadores. Rama: pi/aceptar-github-ece0d480-4a57-48df-a7a5-0cdf6e1db558.

## Specs
- S1: "configurar github igual que gitlab" → ajustes globales por proveedor (URL base + token + TLS inseguro) editables en Configuración, con "Probar conexión"; el token nunca viaja al cliente. GitHub: `https://github.com` por defecto o GitHub Enterprise (`/api/v3`).
- S2: "Un workspace puede ser un repositorio de github o de gitlab, detectar y aplicar la configuración de uno u otro" → el proveedor se detecta por el host del `origin` contra los ajustes configurados; se aplica la configuración de ese proveedor; si ninguno coincide, error claro.
- S3: "para poder gestionar las PRs" → crear PR/MR (reutiliza el abierto, la app nunca hace push), ver estado y CI (jobs), mezclar; mismo flujo para ambos proveedores.
- S4: "utiliza el patron factoria y crea el adapter de github" → `CodeHostClientFactoryPort` devuelve el cliente según el `HostingProvider`; adaptador `GitHubHttpClientAdapter` (REST v3).
- S5: "si gitlab no es adapter, convertir" → GitLab pasa a ser un adaptador (`GitLabHttpClientAdapter`) de un puerto neutro `CodeHostClientPort`; el dominio no menciona GitLab.

## Tasks
- [x] T1 (S5) renombrar módulo gitlab→code_hosting y tipos neutros (sin cambio de comportamiento) — inline — commit: refactor(code-hosting) rename gitlab slice to code_hosting
- [x] T2 (S1,S2,S5) `HostingProvider`, ajustes por proveedor (migración `code_host_settings`, riesgo alto), errores con proveedor, detección por host del origin en el resolver — inline — commit: feat(code-hosting) provider-aware settings and origin detection
  Risk: item 1 (migración mueve gitlab_settings→code_host_settings con el token; verificada up/redo/down con dato real)
- [ ] T3 (S3,S4) `GitHubHttpClientAdapter` + soporte HTTP compartido + `CodeHostClientFactoryPort` y su factoría (resolver y test de conexión la usan) + tests con servidor axum falso — inline — commit:
- [ ] T4 (S1,S3) DTOs/server fns con proveedor + UI (tarjetas GitHub/GitLab, panel PR/MR) + docs — inline — commit:
- [ ] T5 (all) fmt, clippy ssr/hydrate, tests, differential check GitLab, revisión — commit:

## Log
- L1: "Quiero una feature que me permita configurar github igual que gitlab. Un workspace puede ser un repositorio de github o de gitlab, detectar y aplicar la configuración de uno u otro para poder gestionar las PRs. utiliza el patron factoria y crea el adapter de github. si gitlab no es adapter, convertir"
- L2: Límite de slice: T1 son 1518 líneas cambiadas pero casi todas son movimientos por renombrado (git detecta renames); es un único commit mecánico sin cambio de comportamiento. La factoría pasa de T2 a T3 porque su `match` necesita el adaptador GitHub para compilar.
- L3: T2 evidencia: migración probada en BD temporal con fila real (token conservado en up, redo y down); 73 tests lib modules::code_hosting ok; test de integración code_host_settings_repository_adapter_test 5 ok; clippy ssr+hydrate -D warnings ok; fmt ok. Cambio observable: origin con host sin proveedor configurado ahora da UnknownHost (antes HostMismatch).
