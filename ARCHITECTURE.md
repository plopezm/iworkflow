# Arquitectura de iworkflow

Guía para leer el código de iworkflow, pensada para alguien con experiencia sólida en Java que está aprendiendo Rust. Sigue el orden en que conviene leer el código: primero el mapa general, después el arranque, un *slice* completo (`tasks`) y, por último, el resto de módulos.

> Los diagramas usan [Mermaid](https://mermaid.js.org/) (GitHub y la mayoría de editores lo renderizan). Las rutas de fichero son relativas a la raíz del repositorio. Las reglas de estilo y de proceso del proyecto están en [AGENTS.md](AGENTS.md); este documento explica **cómo está construido**, no cómo contribuir.

## Índice

1. [Qué hace la aplicación](#1-qué-hace-la-aplicación)
2. [Rust para un desarrollador Java](#2-rust-para-un-desarrollador-java)
3. [Stack y compilación dual `ssr` / `hydrate`](#3-stack-y-compilación-dual-ssr--hydrate)
4. [Estructura del código](#4-estructura-del-código)
5. [Arranque y raíz de composición](#5-arranque-y-raíz-de-composición)
6. [Anatomía de un slice: `tasks`](#6-anatomía-de-un-slice-tasks)
7. [Flujos de extremo a extremo](#7-flujos-de-extremo-a-extremo)
8. [Persistencia](#8-persistencia)
9. [Frontend: Leptos y Signals](#9-frontend-leptos-y-signals)
10. [Resto de módulos](#10-resto-de-módulos)
11. [Estrategia de tests](#11-estrategia-de-tests)
12. [Ruta de lectura recomendada](#12-ruta-de-lectura-recomendada)

---

## 1. Qué hace la aplicación

iworkflow es un dashboard web para lanzar y supervisar agentes de código (`gentle-shell`, o `pi`) en modo RPC. Cada **tarea** se ejecuta en su propio **git worktree**; al terminar, el usuario revisa el diff y decide si integrar (`merge --no-ff`) o descartar.

```mermaid
flowchart LR
    U[Navegador<br/>Leptos hidratado] -- "server functions (HTTP)" --> S[Servidor axum]
    U -- "SSE: eventos de tarea" --> S
    U -- "WebSocket: terminal" --> S
    S --> DB[(SQLite<br/>Diesel)]
    S -- "git worktree / merge" --> G[Repos git del usuario]
    S -- "JSONL por stdin/stdout" --> A[Proceso agente<br/>gentle-shell --mode rpc]
    S -- "PTY" --> T[Shell interactiva]
```

Es un único binario Rust que sirve el HTML (SSR), el bundle WASM, las *server functions*, un endpoint SSE y un WebSocket.

Ciclo de vida de una tarea (la máquina de estados vive en `src/modules/tasks/domain/models/task.rs`):

```mermaid
stateDiagram-v2
    [*] --> Backlog
    Backlog --> Starting: start (claim atómico)
    Starting --> Running: RunStarted
    Running --> Idle: RunSettled
    Idle --> Running: RunStarted
    Running --> Finishing: finish
    Idle --> Finishing: finish
    Finishing --> AwaitingReview: commit_all OK
    Finishing --> Failed: error
    AwaitingReview --> Merged: merge OK
    AwaitingReview --> Conflict: merge con conflictos
    AwaitingReview --> Discarded: discard
    Starting --> Interrupted: proceso muerto / reinicio
    Running --> Interrupted: proceso muerto / reinicio
    Idle --> Interrupted: proceso muerto / reinicio
    Interrupted --> Running: resume
    Conflict --> Running: resume
    Failed --> Running: resume
    Merged --> [*]
    Discarded --> [*]
```

Las transiciones se validan con métodos del propio enum (`can_start`, `can_finish`, `can_merge`, …), no con `if` dispersos por los casos de uso.

---

## 2. Rust para un desarrollador Java

Tabla de equivalencias que necesitarás constantemente al leer este repo.

| Java | Rust en este proyecto | Notas |
|---|---|---|
| `interface Foo` | `trait Foo` | Los puertos outbound son traits (`TaskRepositoryPort`). |
| Inyectar `Foo` por constructor | `Arc<dyn Foo>` como campo | `Arc` = puntero compartido con conteo de referencias atómico (como una referencia Java que además sabes cuándo se libera). `dyn` = despacho dinámico (vtable), lo que en Java es lo normal y en Rust es explícito. |
| `class` con campos + métodos | `struct` + `impl` | Los datos y los métodos se declaran por separado. No hay herencia. |
| `enum` simple | `enum` | Pero los enums de Rust **llevan datos** (son *sum types*): `AgentEvent::ProcessExited { code }`. Equivalen a una jerarquía `sealed` de records. |
| `switch` / pattern matching | `match` | Exhaustivo: si añades una variante, el compilador te señala todos los `match` que faltan. |
| `Optional<T>` / `null` | `Option<T>` | No existe `null`. `Some(x)` / `None`. |
| `throws` / excepciones | `Result<T, E>` | Los errores son valores. `?` propaga el error al llamante (como un `throw` implícito pero visible en la firma). |
| `try/catch` | `match` sobre `Result`, `if let Err(e)`, `.map_err(...)` | `map_err` convierte un tipo de error en otro (p. ej. de Diesel a `TaskError`). |
| Lombok `@Data`, `equals` | `#[derive(Debug, Clone, PartialEq, ...)]` | Los `derive` generan implementaciones de traits. |
| Constructor / factory | `fn new(...) -> Self` | Convención, no sintaxis. |
| `final` | Inmutable por defecto | `let x` es inmutable, `let mut x` es mutable. AGENTS.md exige `mut` solo si hace falta. |
| `package` / `import` | `mod` / `use` | La jerarquía de módulos sigue el árbol de carpetas; cada carpeta tiene un `mod.rs`. |
| `public` / package-private | `pub` / privado por módulo | Por defecto todo es privado al módulo. |
| Tipos genéricos | Genéricos con *trait bounds* | `fn retry<T, F, Fut>(...) where F: FnMut() -> Fut` |
| Lambdas | Closures `\|x\| x + 1` | `move \|\| ...` captura por valor (necesario al pasar closures a otro hilo/tarea). |
| `CompletableFuture` | `async fn` + `.await` | Los futures de Rust son **perezosos**: no hacen nada hasta que se les hace `.await` (o se lanzan con `tokio::spawn`). |
| Spring `@Transactional` / pool | `deadpool` + Diesel | Ver [persistencia](#8-persistencia). |
| Mockito | `mockall` | `mock! { ... }` genera dobles para los traits. |
| `Thread` / `ExecutorService` | `tokio::spawn` | Tareas asíncronas ligeras sobre un pool de hilos. |
| `BlockingQueue` | `tokio::sync::mpsc` | Canal multi-productor / consumidor único. |
| `synchronized` / `ReentrantLock` | `Mutex<T>` | El mutex **posee** el dato: solo se accede a él a través del guard. |

### Conceptos que no tienen equivalente directo

**Propiedad y préstamos (*ownership* / *borrowing*).** Cada valor tiene un único dueño; pasar un valor lo *mueve*. Para leer sin tomar posesión se presta: `&T` (lectura, muchos a la vez) o `&mut T` (escritura, uno solo). Verás `&self` (método que solo lee), `&mut self` (muta) y `self` (consume el objeto). Esto sustituye al recolector de basura: no hay GC, el compilador decide cuándo se libera cada cosa.

**`Arc<T>` y `.clone()`.** Clonar un `Arc` **no** copia el dato, solo incrementa un contador. Por eso los contextos (`TasksContext`, etc.) son `#[derive(Clone)]` y se clonan libremente: cada campo es un `Arc<UseCase>`.

**`Send + Sync`.** Marcadores que dicen "este tipo se puede mover/compartir entre hilos". Los traits asíncronos se declaran `trait X: Send + Sync` porque el runtime de tokio puede mover las tareas entre hilos.

**`#[async_trait::async_trait]`.** Rust todavía no permite `async fn` cómodamente en traits usados como `dyn Trait`; esta macro lo resuelve. Aparece en todos los puertos y adaptadores.

**Macros (`!` y `#[...]`).** `view! { ... }`, `mock! { ... }`, `#[server]`, `#[derive(...)]`, `#[component]` generan código en tiempo de compilación. Piensa en ellas como un procesador de anotaciones de Java mucho más potente.

**Atributos `#[cfg(feature = "ssr")]`.** Compilación condicional. El mismo crate se compila dos veces con conjuntos distintos de código (ver siguiente sección). Es el mecanismo central de este proyecto.

**Newtypes.** `pub struct TaskId(Uuid);` es un tipo distinto de `Uuid` aunque contenga uno: evita mezclar identificadores. Equivale a un *value object* Java.

**Enums con estado y `impl` propio.** `TaskStatus` es un enum con métodos (`can_finish()`, `after_agent_event()`): el dominio es **rico**, no anémico.

---

## 3. Stack y compilación dual `ssr` / `hydrate`

| Pieza | Tecnología |
|---|---|
| Lenguaje | Rust (edición 2024) |
| UI + servidor | [Leptos](https://leptos.dev) 0.8 (SSR + hidratación, reactividad con *signals*) sobre `axum`, orquestado por `cargo-leptos` |
| Persistencia | Diesel + `diesel-async` (`SyncConnectionWrapper<SqliteConnection>`) + pool `deadpool`, SQLite embebido (`libsqlite3-sys` bundled) |
| Migraciones | `migrations/` (Diesel), embebidas con `embed_migrations!` y aplicadas al arrancar |
| Estilos | Tailwind CSS (`style/tailwind.css`) |
| Procesos / PTY | `tokio::process`, `portable-pty` |
| Tests | `cargo test`, `mockall`, `tempfile`, Playwright para e2e |

### Un crate, dos binarios

Leptos compila el mismo crate **dos veces**:

```mermaid
flowchart TB
    SRC[src/ — un solo crate iworkflow] --> SSR["cargo build --features ssr<br/>→ binario nativo (servidor)"]
    SRC --> HYD["cargo build --features hydrate<br/>--target wasm32-unknown-unknown<br/>→ iworkflow.wasm (navegador)"]
    SSR --> |incluye| D1[domain · application · Diesel · axum · tokio · procesos · git]
    SSR --> |incluye| P1[presentation · DTOs · server_fns - cuerpo real]
    HYD --> |incluye| P2[presentation · DTOs · server_fns - solo stub HTTP]
    HYD -. NO incluye .-> D1
```

Configuración en `Cargo.toml`:

```toml
[features]
hydrate = ["leptos/hydrate", "dep:console_error_panic_hook", "dep:wasm-bindgen", "dep:web-sys", "dep:js-sys"]
ssr = ["dep:axum", "dep:tokio", "dep:diesel", "dep:diesel-async", "leptos/ssr", /* ... */]

[package.metadata.leptos]
bin-features = ["ssr"]      # el binario del servidor se compila con ssr
lib-features = ["hydrate"]  # la librería WASM se compila con hydrate
```

Las dependencias del servidor son `optional = true` y solo las activa `ssr`: **Diesel y tokio nunca llegan al bundle WASM**. La regla se aplica en el código con `#[cfg(feature = "ssr")]`. Mira `src/modules/tasks/mod.rs`:

```rust
#[cfg(feature = "ssr")]
pub mod application;     // casos de uso: solo servidor
#[cfg(feature = "ssr")]
pub mod domain;          // dominio: solo servidor
pub mod infrastructure;  // existe en ambos (DTOs y server fns), pero por dentro filtra
#[cfg(all(test, feature = "ssr"))]
pub mod mocks;
pub mod presentation;    // UI: en ambos
```

Consecuencia importante: **el cliente WASM no conoce el dominio**. Por eso los tipos que cruzan la red son DTOs en `infrastructure/adapters/inbound/dto/`, compilados en ambas features. El `TaskStatusDto` duplica a propósito el enum `TaskStatus` del dominio (y hasta replica `has_live_agent`), y la conversión `From<Task> for TaskDto` está dentro de un bloque `#[cfg(feature = "ssr")]`.

### Qué es una *server function*

```rust
#[server]
pub async fn get_task(id: String) -> Result<TaskDto, ServerFnError> { /* cuerpo solo servidor */ }
```

La macro `#[server]` genera **dos** cosas: en el servidor, el endpoint HTTP con el cuerpo real; en el cliente, un *stub* con la misma firma que hace un `fetch` al endpoint. Desde un componente se llama como una función async normal. Es el equivalente a un controlador REST + un cliente generado, sin escribir ninguno. En términos de arquitectura hexagonal, **es el adaptador inbound**.

---

## 4. Estructura del código

```text
src/
├── main.rs            # arranque axum (solo ssr)
├── lib.rs             # entrada WASM (hydrate) + declaración de módulos
├── app.rs             # <App>, shell HTML, rutas del router
├── app_state.rs       # RAÍZ DE COMPOSICIÓN: agrupa contextos y puentes entre features
├── core/              # transversal; NO puede importar de modules/
│   ├── config.rs      # AppConfig (variables de entorno IWF_*)
│   ├── db/            # pool.rs (DbPool, migraciones), schema.rs (generado por Diesel)
│   ├── http/          # límite de cuerpo de las server functions
│   ├── git.rs, env.rs # utilidades sin dominio
│   └── components/    # componentes UI base (modal, toast, select, shell, ...)
└── modules/           # un directorio por feature (vertical slice)
    ├── tasks/         # el núcleo: tareas, worktrees, agentes, eventos
    ├── workspaces/    # repositorios git registrados
    ├── terminal/      # terminales PTY por WebSocket
    ├── agent_settings/# modelo/perfil/runtime/thinking globales
    └── agents_files/  # ficheros AGENTS.md reutilizables
```

### Corte vertical + capas hexagonales

Cada feature contiene **todas** sus capas. Dentro de cada una, las dependencias solo apuntan hacia el centro:

```text
modules/<feature>/
├── domain/            # Rust puro: modelos, errores (thiserror), PUERTOS (traits)
│   ├── models/
│   ├── errors/
│   └── ports/{inbound,outbound}/
├── application/       # casos de uso: un struct por acción con método execute()
│   └── usecases/
├── infrastructure/    # adaptadores técnicos
│   ├── adapters/inbound/{dto/, server_fns.rs [, sse.rs | ws.rs]}
│   ├── adapters/outbound/{persistence/, git/, runtime/, fs/, ...}
│   └── di/<feature>_context.rs     # cablea puertos ↔ adaptadores ↔ casos de uso
├── presentation/      # UI Leptos
│   ├── state/         # signals / Resources / Actions de una pantalla
│   ├── pages/
│   └── components/
└── mocks.rs           # mocks mockall de los puertos (solo #[cfg(test)])
```

```mermaid
flowchart LR
    subgraph Cliente["Navegador (WASM)"]
        PG[pages / components] --> ST["state<br/>Resource · ServerAction · RwSignal"]
    end
    ST -- "HTTP" --> SF
    subgraph Servidor["Servidor (ssr)"]
        subgraph INF_IN["infrastructure — inbound"]
            SF["server_fns.rs<br/>(+ DTOs, SSE, WS)"]
        end
        subgraph APP["application"]
            UC["Casos de uso<br/>struct + execute()"]
        end
        subgraph DOM["domain (Rust puro)"]
            M["models · errors"]
            PO["ports/outbound<br/>(traits)"]
        end
        subgraph INF_OUT["infrastructure — outbound"]
            AD["Adaptadores<br/>Diesel · git CLI · procesos · FS"]
        end
        SF --> UC
        UC --> M
        UC --> PO
        AD -. implementa .-> PO
    end
```

Reglas de dependencia (equivalentes a ArchUnit en Java, pero aquí por convención y revisión):

- `domain` y `application` **no** importan `leptos`, `diesel`, `axum`, ni nada de `infrastructure` / `presentation`.
- Un caso de uso depende de **traits** (`Arc<dyn TaskRepositoryPort>`), nunca de adaptadores concretos.
- Las queries Diesel solo existen en `infrastructure/adapters/outbound/persistence/`.
- `core/` no importa nada de `modules/`. Un feature no importa a otro: se comunican por puertos y puentes en `app_state.rs`.

---

## 5. Arranque y raíz de composición

### `src/main.rs`

Es el equivalente a la clase `@SpringBootApplication` + la configuración de beans. Secuencia, en orden:

```mermaid
sequenceDiagram
    participant M as main()
    participant C as AppConfig
    participant DB as Pool + migraciones
    participant S as AppState
    participant R as axum Router
    M->>C: from_env() (IWF_*)
    M->>DB: build_pool(url) + run_migrations
    M->>S: AppState::new(pool, &config)
    M->>S: tasks.reconcile_tasks.execute()
    Note over S: marca como Interrupted las tareas<br/>que estaban activas antes del reinicio
    M->>R: SSE + WS + rutas Leptos con contexto
    M->>R: axum::serve(... graceful shutdown)
    Note over M,R: Ctrl+C / SIGTERM → plazo de gracia 5s →<br/>flush_events() de la cola (hasta 30s)
```

Detalles útiles para leerlo:

- `#[tokio::main] async fn main()` arranca el runtime asíncrono (en Java sería el contenedor/servidor embebido).
- En `main.rs` sí se permite `expect()`/`panic!` (AGENTS.md lo autoriza solo en el arranque): si la configuración o las migraciones fallan, no tiene sentido continuar.
- Los routers SSE (`task_events_router`) y WebSocket (`terminal_ws_router`) se montan con `.merge(...)` junto al router de Leptos.
- `leptos_routes_with_context(..., move || context_state.provide(), ...)` es el punto donde se **inyectan** los contextos: esa closure se ejecuta por cada petición y llama a `provide_context` para cada feature.
- El cierre ordenado importa: los eventos del agente se encolan en memoria (ver [§7.2](#72-eventos-del-agente-el-pipeline-más-importante)) y `flush_events()` espera a que se persistan antes de salir.

### `src/app_state.rs` — la raíz de composición

```rust
#[derive(Clone)]
pub struct AppState {
    pub workspaces: WorkspacesContext,
    pub tasks: TasksContext,
    pub agent_settings: AgentSettingsContext,
    pub agents_files: AgentsFilesContext,
    pub terminal: TerminalContext,
}

impl AppState {
    pub fn provide(&self) {
        provide_context(self.workspaces.clone());
        provide_context(self.tasks.clone());
        // ...
    }
}
```

**Inyección de dependencias sin framework.** No hay contenedor IoC con reflexión. El patrón es:

1. Cada feature expone un `XxxContext` (struct con sus casos de uso ya construidos como `Arc<UseCase>`), en `infrastructure/di/<feature>_context.rs`.
2. `AppState::new` construye los contextos en el orden adecuado.
3. En cada petición, `provide()` los registra en el **contexto de Leptos** (`provide_context`).
4. Las server functions los recuperan con `use_context::<TasksContext>()`.

Piensa en `provide_context` / `use_context` como un `ApplicationContext` ligero y tipado por clase.

### Los *bridges*: cómo se hablan los features sin acoplarse

Un feature **no puede importar otro**. Cuando `tasks` necesita saber qué workspace tiene una tarea, define un **puerto propio** y `app_state.rs` lo implementa con un adaptador ("bridge") que usa los casos de uso de `workspaces`:

```rust
/// Expone los workspaces al feature de tareas a través de su propio puerto.
pub struct WorkspaceLookupBridge {
    pub get_workspace: Arc<GetWorkspace>,
    pub list_workspaces: Arc<ListWorkspaces>,
}

#[async_trait::async_trait]
impl WorkspaceLookupPort for WorkspaceLookupBridge {   // puerto definido en tasks/
    async fn find(&self, workspace_id: &str) -> Result<Option<WorkspaceRef>, TaskError> {
        let Ok(id) = WorkspaceId::parse(workspace_id) else { return Ok(None); };
        match self.get_workspace.execute(id).await {   // caso de uso de workspaces/
            Ok(workspace) => Ok(Some(to_ref(workspace))),
            Err(WorkspaceError::NotFound(_)) => Ok(None),
            Err(error) => Err(TaskError::Repository(error.to_string())),
        }
    }
    // ...
}
```

Es el patrón *anti-corruption layer*: `tasks` ve un `WorkspaceRef` (su propia vista mínima), no el `Workspace` de otro dominio.

```mermaid
flowchart LR
    subgraph tasks
        TUC[Casos de uso de tasks] --> WLP["WorkspaceLookupPort<br/>LaunchSettingsPort"]
    end
    subgraph workspaces
        WUC[GetWorkspace · ListWorkspaces]
        WTP["WorkspaceTasksPort<br/>AgentsFilesPort"]
    end
    subgraph agent_settings
        PL[PrepareLaunch]
    end
    subgraph agents_files
        AF[ListAgentsFiles · InstallAgentsFile]
    end
    subgraph "app_state.rs (composition root)"
        B1[WorkspaceLookupBridge]
        B2[LaunchSettingsBridge]
        B3[WorkspaceTasksBridge]
        B4[AgentsFilesBridge]
        B5[ProjectLookupBridge]
    end
    WLP -.- B1 --> WUC
    WLP -.- B2 --> PL
    WTP -.- B3 --> TUC
    WTP -.- B4 --> AF
    AF -. ProjectLookupPort .- B5 --> WUC
```

| Bridge | Puerto (lo define…) | Delega en… |
|---|---|---|
| `WorkspaceLookupBridge` | `WorkspaceLookupPort` (tasks) | `workspaces::GetWorkspace`/`ListWorkspaces` |
| `LaunchSettingsBridge` | `LaunchSettingsPort` (tasks) | `agent_settings::PrepareLaunch` |
| `WorkspaceTasksBridge` | `WorkspaceTasksPort` (workspaces) | `tasks::DeleteWorkspaceTasks` |
| `AgentsFilesBridge` | `AgentsFilesPort` (workspaces) | `agents_files::ListAgentsFiles`/`InstallAgentsFile` |
| `ProjectLookupBridge` | `ProjectLookupPort` (agents_files) | `workspaces::GetWorkspace`/`ListWorkspaces` |
| `workspace_path_resolver` (closure) | `PathResolver` (terminal) | `workspaces::GetWorkspace` |

El orden en `AppState::new` no es arbitrario: `workspaces` necesita a `tasks` y `agents_files` (para borrado en cascada e instalar AGENTS.md), pero estos necesitan antes los casos de uso de lectura de workspaces. Por eso se crean primero `get_workspace` / `list_workspaces` con su repositorio y los demás contextos se construyen sobre ellos.

---

## 6. Anatomía de un slice: `tasks`

`src/modules/tasks/` es el feature más grande y el mejor ejemplo. Lo recorremos de dentro hacia fuera.

### 6.1 Dominio (`domain/`)

Rust puro. Sin Leptos, Diesel ni axum.

**Modelos** (`domain/models/`):

| Fichero | Contenido |
|---|---|
| `task.rs` | `Task`, `TaskId` (newtype de `Uuid`), `TaskStatus` (11 estados + reglas de transición, `can_view_diff`), `diff_range: Option<DiffRange>`, `TaskStage` (vista agregada para el tablero), `NewTask`, helpers `branch_name(...)` |
| `agent_event.rs` | `AgentEvent`: eventos normalizados que emite el agente (`RunStarted`, `TextDelta`, `ToolFinished`, `DialogRequested`, `ProcessExited`, …) |
| `agent_command.rs` | `AgentCommand`: órdenes hacia el agente (`Prompt`, `Steer`, `FollowUp`, `Abort`, `AnswerDialog`) |
| `task_event.rs` | `TaskEvent` / `TaskEventKind`: lo que se persiste y se difunde (estado, agente, efímeros) |
| `worktree.rs` | `MergeOutcome` (`Merged` / `Conflict { files }`), `TaskDiff`, `DiffRange` (`base_sha..head_sha`: commits de la tarea, para regenerar su diff tras integrar) |
| `workspace_ref.rs` | Vista mínima de un workspace desde el punto de vista de tareas |

La parte más importante es que `TaskStatus` **encapsula las reglas**:

```rust
pub fn can_finish(&self) -> bool {
    matches!(self, TaskStatus::Running | TaskStatus::Idle | TaskStatus::Interrupted
                 | TaskStatus::Failed  | TaskStatus::Conflict)
}

/// Transición provocada por un evento del agente, si aplica.
pub fn after_agent_event(&self, event: &AgentEvent) -> Option<(TaskStatus, Option<String>)> {
    match event {
        AgentEvent::RunStarted if self.has_live_agent() => Some((TaskStatus::Running, None)),
        AgentEvent::RunSettled if matches!(self, TaskStatus::Starting | TaskStatus::Running)
            => Some((TaskStatus::Idle, None)),
        AgentEvent::ProcessExited { code } if self.has_live_agent() => Some((
            TaskStatus::Interrupted,
            Some(format!("el agente terminó inesperadamente ({})", /* ... */)),
        )),
        _ => None,
    }
    .filter(|(next, _)| next != self)
}
```

Lectura para Java: `matches!(x, A | B)` es un `switch` que devuelve `boolean`; los `if` tras el patrón (`RunStarted if self.has_live_agent()`) son *guards*; devolver `Option<(TaskStatus, Option<String>)>` significa "quizá hay transición, y quizá lleva un mensaje de error". Esta función es **pura** (no toca BD), lo que la hace trivial de testear.

`TaskStage` es una proyección (`Backlog`, `Working`, `Attention`, `Review`, `Done`) calculada a partir del estado, los diálogos pendientes y la actividad de subagentes; el tablero agrupa por ella y `needs_attention()` decide qué resaltar.

**Errores** (`domain/errors/task_error.rs`): un `enum TaskError` con `thiserror`. Es el equivalente a una jerarquía de excepciones, pero cerrada y exhaustiva. Incluye `is_transient()` (solo `Busy`, "SQLite bloqueada"), que usa el decorador de reintentos.

### 6.2 Puertos (`domain/ports/`)

**Outbound** (lo que el dominio *necesita* del mundo exterior) — todos `#[async_trait] pub trait X: Send + Sync`:

| Puerto | Responsabilidad | Adaptador |
|---|---|---|
| `TaskRepositoryPort` | Persistir tareas y eventos; operaciones condicionales atómicas | `TaskRepositoryAdapter` (Diesel) |
| `TaskAttachmentRepositoryPort` | Metadatos de imágenes adjuntas | `TaskAttachmentRepositoryAdapter` (Diesel) |
| `AttachmentStoragePort` | Ficheros de imagen en disco | `FsAttachmentStorageAdapter` |
| `WorktreePort` | `git worktree add`, commit, diff, `pin_range` / `diff_range` (diff de tareas integradas), merge, remove | `GitCliWorktreeAdapter` (ejecuta `git`) |
| `AgentRuntimePort` | Lanzar/parar/hablar con el agente de una tarea, sin presuponer el transporte; declara `AgentCapabilities` y rechaza con `TaskError::Unsupported` lo que no soporta; `subagents(task_id)` devuelve la actividad (`SubagentTask`) de los subagentes, vacía si el runtime no la ofrece (el adaptador pi decodifica `gentle-agents.activity/v1` y guarda en memoria la última instantánea; la UI la abre desde las acciones de la tarea) | `AgentRuntimeRouter` → `PiRpcRuntimeAdapter` (registrado como `gentle-shell` y `pi`) |
| `TaskEventPublisherPort` | Difundir eventos en vivo | `BroadcastTaskEventBus` (tokio broadcast) |
| `WorkspaceLookupPort` | Consultar workspaces | `WorkspaceLookupBridge` (en `app_state.rs`) |
| `LaunchSettingsPort` | Modelo/thinking a usar al lanzar | `LaunchSettingsBridge` |

**Inbound** (`ports/inbound/agent_event_sink.rs`): el único puerto inbound. Existe porque hay **dos** drivers: el runtime del agente empuja eventos hacia el dominio.

```rust
#[async_trait::async_trait]
pub trait AgentEventSink: Send + Sync {
    async fn handle(&self, task_id: TaskId, event: AgentEvent);
}
```

Las server functions *no* tienen puerto inbound: llaman directamente al struct del caso de uso (AGENTS.md lo permite cuando hay un único consumidor).

Ejemplo de puerto con contrato documentado (`agent_runtime_port.rs`):

```rust
#[async_trait::async_trait]
pub trait AgentRuntimePort: Send + Sync {
    async fn start(&self, task_id: TaskId, cwd: &Path, resume: bool,
                   options: &LaunchOptions, sink: Arc<dyn AgentEventSink>) -> Result<(), TaskError>;
    async fn send(&self, task_id: TaskId, command: AgentCommand) -> Result<(), TaskError>;
    async fn stop(&self, task_id: TaskId) -> Result<(), TaskError>;
    async fn is_running(&self, task_id: TaskId) -> bool;
}
```

Fíjate en que `start` recibe un `sink`: el runtime no conoce la BD, solo "alguien que quiere los eventos".

### 6.3 Casos de uso (`application/`)

Un caso de uso = un `struct` con sus puertos como campos `Arc<dyn …>`, un `new(...)` y un `execute(...)`. Es el equivalente a un `@Service` con un único método público.

Hay 20 en `application/usecases/`: `create_task`, `start_task`, `update_task`, `delete_task`, `send_message`, `abort_task`, `answer_dialog`, `finish_task`, `resume_task`, `merge_task`, `discard_task`, `reset_task`, `get_task`, `get_task_diff`, `get_task_events`, `list_tasks`, `list_task_attachments`, `list_workspace_options`, `reconcile_tasks`, `delete_workspace_tasks`, más `handle_agent_event` (implementa `AgentEventSink`).

**`TaskLifecycle`** (`application/services/task_lifecycle.rs`) es el colaborador compartido por casi todos: carga tareas, cambia estado, registra eventos y los difunde.

```rust
pub async fn transition(&self, id: TaskId, status: TaskStatus, error: Option<String>)
    -> Result<(), TaskError>
{
    self.repository.update_status(id, status, error.clone()).await?;
    if !status.has_live_agent() {
        self.repository.clear_pending_dialogs(id).await?;
    }
    self.emit(id, TaskEventKind::status(status, error)).await
}

async fn emit(&self, id: TaskId, kind: TaskEventKind) -> Result<(), TaskError> {
    let event: TaskEvent = if kind.is_persistent() {
        self.repository.append_event(id, kind).await?   // se guarda y recibe su `seq`
    } else {
        TaskEvent::ephemeral(id, kind)                   // solo en vivo
    };
    self.publisher.publish(event);                       // difusión a SSE
    Ok(())
}
```

Otros servicios: `AttachmentCleanup` (borra imágenes al terminar la tarea, mejor esfuerzo) y `QueuedAgentEventSink` (cola, ver §7.2).

**Ejemplo comentado: `FinishTask`** (resumido):

```rust
pub async fn execute(&self, id: TaskId) -> Result<(), TaskError> {
    let task: Task = self.lifecycle.load(id).await?;         // `?` = si falla, retorna el Err
    if !task.status.can_finish() {                            // regla de dominio
        return Err(invalid_state("finalizar", task.status));
    }
    self.lifecycle.transition(id, TaskStatus::Finishing, None).await?;

    if let Err(error) = self.consolidate(&task).await {       // parar agente + commit_all
        self.lifecycle.transition(id, TaskStatus::Failed, Some(error.to_string())).await?;
        return Err(error);
    }
    self.lifecycle.transition(id, TaskStatus::AwaitingReview, None).await?;
    self.cleanup.cleanup_task(id).await;                      // limpieza de imágenes
    Ok(())
}
```

Observa el patrón: **validar con el dominio → transicionar → operar con puertos → compensar si falla**. Casi todos los casos de uso siguen esta forma.

### 6.4 Infraestructura (`infrastructure/`)

**Adaptadores outbound** (`adapters/outbound/`):

| Carpeta | Qué hace |
|---|---|
| `persistence/` | `TaskRepositoryAdapter` (Diesel), `TaskAttachmentRepositoryAdapter`, `RetryingTaskRepository` (decorador), `models/*_row.rs` (structs `Queryable`/`Insertable`) |
| `git/` | `GitCliWorktreeAdapter`: invoca el binario `git` (worktree add, commit, merge `--no-ff`, merge en worktree temporal si la rama base no está checkouteada, …) |
| `runtime/` | `AgentRuntimeRouter` (delegación por `RuntimeId`); `pi/`: `PiRpcRuntimeAdapter` + `rpc_protocol.rs` (JSONL ↔ `AgentEvent`/`AgentCommand`). Añadir un agente = nuevo adaptador de `AgentRuntimePort` registrado en `tasks_context.rs` (y, si procede, su binario en el catálogo de modelos de `agent_settings`) |
| `events/` | `BroadcastTaskEventBus`: `tokio::sync::broadcast` con capacidad 4096 |
| `storage/` | `FsAttachmentStorageAdapter`: imágenes en `~/.iworkflow/attachments/` |

**Adaptadores inbound** (`adapters/inbound/`): `server_fns.rs`, `sse.rs` y `dto/`.

**DI** (`di/tasks_context.rs`): `TasksContext::with_runtime(...)` es el "fichero de configuración de beans" del feature. Construye adaptadores, los envuelve en decoradores y los inyecta en los casos de uso. Fragmento esencial:

```rust
let base_repository: Arc<dyn TaskRepositoryPort> = Arc::new(TaskRepositoryAdapter::new(pool));
// Dos decoradores sobre el MISMO repositorio, con políticas de reintento distintas:
let repository: Arc<dyn TaskRepositoryPort> =
    Arc::new(RetryingTaskRepository::new(base_repository.clone(), RetryPolicy::bounded()));      // 30 s
let events_repository: Arc<dyn TaskRepositoryPort> =
    Arc::new(RetryingTaskRepository::new(base_repository, RetryPolicy::persistent()));           // sin límite

let lifecycle        = Arc::new(TaskLifecycle::new(repository.clone(), publisher.clone(), workspaces.clone()));
let events_lifecycle = Arc::new(TaskLifecycle::new(events_repository, publisher, workspaces.clone()));
let handler: Arc<dyn AgentEventSink> = Arc::new(HandleAgentEvent::new(events_lifecycle));
let event_queue = Arc::new(QueuedAgentEventSink::new(handler, DEFAULT_QUEUE_CAPACITY));
```

La idea: las peticiones de usuario fallan tras 30 s de reintentos (el usuario espera respuesta), pero los eventos del agente **reintentan indefinidamente** porque perderlos corrompería el historial.

### 6.5 DTOs y server functions

Los DTOs (`dto/task_dto.rs`, …) son structs `Serialize`/`Deserialize` compilados en cliente y servidor. Los structs de Diesel nunca salen del servidor. Una server function típica no tiene lógica:

```rust
#[server]
pub async fn get_task(id: String) -> Result<TaskDto, ServerFnError> {
    use crate::modules::tasks::domain::models::task::Task;

    let task: Task = support::context()?          // use_context::<TasksContext>()
        .get_task                                  // Arc<GetTask>
        .execute(support::task_id(&id)?)           // parsea el id, errores → ServerFnError
        .await
        .map_err(support::to_server_error)?;       // TaskError → ServerFnError
    Ok(TaskDto::from(task))                        // dominio → DTO
}
```

Los `use` del dominio van *dentro* de la función: así el stub del cliente WASM no intenta resolverlos.

**SSE** (`sse.rs`): dos endpoints axum (no server functions, porque son streams):

- `GET /api/tasks/{id}/events`: historial persistido + stream en vivo de una tarea. Protocolo: evento `reset`, historial, y eventos en vivo filtrados por `seq` para no duplicar.
- `GET /api/tasks/events`: stream global del tablero; emite `changed` con el id solo para eventos relevantes (`is_board_relevant`).

Si un suscriptor se retrasa más que la capacidad del canal, recibe `lagged` y el cliente debe reconectar y refetchear.

### 6.6 Presentación (`presentation/`)

```text
presentation/
├── pages/        board_page.rs · task_detail_page.rs · task_edit_page.rs
├── state/        board_state.rs · task_detail_state.rs · task_edit_state.rs · task_events.rs · ...
└── components/   task_card.rs · board_lane.rs · composer.rs · diff_view.rs · transcript_view.rs · ...
```

La página crea su estado, y los componentes pintan. Detalle completo en [§9](#9-frontend-leptos-y-signals).

`state/diff_report_state.rs` gestiona el informe imprimible (PDF) del diff, simple o con explicaciones del agente (`ReportPhase`: `Preparing` → `Plain`/`Waiting` → `Ready`). Mientras está ocupado muestra el `ProgressDialog` cancelable de `core/components/`; el render pesado se retrasa un instante (`PREPARE_DELAY`) para que el navegador pinte antes el diálogo.

---

## 7. Flujos de extremo a extremo

### 7.1 Arrancar una tarea

```mermaid
sequenceDiagram
    actor U as Usuario
    participant UI as BoardPage / BoardState
    participant SF as start_task (server fn)
    participant UC as StartTask
    participant R as TaskRepositoryPort
    participant W as WorktreePort
    participant RT as AgentRuntimePort
    U->>UI: pulsa "Iniciar"
    UI->>SF: ServerAction StartTask
    SF->>UC: execute(id)
    UC->>R: find(id) + can_start()?
    UC->>R: claim_for_start(branch, path, base)
    Note over R: UPDATE ... WHERE id=? AND status='backlog'<br/>(atómico: dos clics = un solo ganador)
    UC->>W: create(repo, base_branch, branch, path)
    alt falla create
        UC->>R: revert_start_claim (→ Backlog)
    end
    UC->>R: notify_status(Starting)
    UC->>RT: start(task_id, worktree, sink)
    UC->>RT: send(Prompt{mensaje, imágenes})
    alt falla el lanzamiento
        UC->>RT: stop (si corre)
        UC->>R: transition(Failed)
    end
    UC-->>SF: Task
    SF-->>UI: TaskDto
```

Puntos de diseño a notar en `src/modules/tasks/application/usecases/start_task.rs`:

- **Reclamo atómico (optimistic claim)**: `claim_for_start` es un único `UPDATE … WHERE status = 'backlog'`; devuelve `false` si alguien ya la reclamó. Sustituye a un lock o a `SELECT … FOR UPDATE`.
- **Compensación explícita**: si `worktrees.create` falla se revierte el reclamo; si `launch` falla se para el agente y se marca `Failed` conservando el worktree para inspección.
- `branch_name(&title, &uuid)` y `worktree_dir_name(...)` generan nombres únicos (`pi/<slug>-<id8>`).

### 7.2 Eventos del agente: el pipeline más importante

El agente es un proceso hijo que habla **JSONL por stdin/stdout** (`gentle-shell --mode rpc`). Cada línea de stdout se convierte en un `AgentEvent`, se persiste (si procede), provoca transiciones de estado y se difunde al navegador.

```mermaid
flowchart TB
    subgraph proc["Proceso hijo: gentle-shell --mode rpc"]
        OUT[stdout JSONL]
        IN[stdin JSONL]
    end
    subgraph rt["PiRpcRuntimeAdapter (una tarea tokio 'supervise' por agente)"]
        RE["read_events()<br/>decode(línea) → AgentEvent"]
        WC["write_commands()<br/>AgentCommand → encode → línea"]
    end
    OUT --> RE
    WC --> IN
    RE -- "sink.handle(task_id, event)" --> Q
    subgraph app["application"]
        Q["QueuedAgentEventSink<br/>mpsc (4096) + 1 worker FIFO"]
        H["HandleAgentEvent.execute()"]
        L["TaskLifecycle<br/>record() · transition()"]
        Q --> H --> L
    end
    L -- "append_event (si persistente)" --> DB[(SQLite)]
    L -- "publish" --> BUS["BroadcastTaskEventBus"]
    BUS --> SSE["sse.rs → EventSource"]
    SSE --> UI[Navegador: transcript / tablero]
```

**Paso 1 — el runtime** (`runtime/gentle_shell_runtime_adapter.rs`). `start()` lanza el proceso (vía la shell de login con `exec`, entorno limpio salvo variables de sesión, `kill_on_drop`) y crea una tarea tokio `supervise` por agente. Mantiene un registro `HashMap<TaskId, AgentProcess>` protegido por `Mutex`. `supervise` usa `tokio::select!` para esperar a la vez: lectura de stdout, escritura a stdin y señal de kill. Al terminar el proceso emite `AgentEvent::ProcessExited { code }`.

```rust
loop {
    tokio::select! {
        _ = &mut reader => break,                          // stdout cerrado: el agente terminó
        _ = &mut writer, if !writer_done => writer_done = true,
        signal = &mut kill_rx, if kill_armed => { /* start_kill() */ }
    }
}
```

`select!` es como esperar a "el primero que ocurra" entre varios `Future`s (parecido a `CompletableFuture.anyOf`, pero cancelando los demás). `stop()` cierra stdin (señal de apagado ordenado), espera un timeout y, si no basta, mata el proceso.

**Paso 2 — traducción** (`rpc_protocol.rs`): `encode(AgentCommand) -> String` y `decode(&str) -> Vec<AgentEvent>`. Aquí vive todo el conocimiento del protocolo de pi; el dominio no sabe nada de JSON-RPC.

**Paso 3 — desacoplar con una cola** (`QueuedAgentEventSink`). Si el lector de stdout escribiera en SQLite directamente, una base de datos bloqueada pararía la lectura y el pipe del agente se llenaría. En su lugar, `handle()` solo encola (con *backpressure*: si la cola se llena, espera; nada se descarta) y **un único worker** consume en orden FIFO, preservando el orden por tarea. Cada evento se procesa en su propio `tokio::spawn` para que un `panic` solo pierda ese evento.

**Paso 4 — `HandleAgentEvent`**:

```rust
pub async fn execute(&self, id: TaskId, event: AgentEvent) -> Result<(), TaskError> {
    let next: Option<(TaskStatus, Option<String>)> = if event.is_persistent() {
        let task: Task = self.lifecycle.load(id).await?;
        task.status.after_agent_event(&event)       // regla pura del dominio
    } else { None };
    self.lifecycle.record(id, event).await?;         // persistir (si procede) + publicar
    if let Some((status, error)) = next {
        self.lifecycle.transition(id, status, error).await?;
    }
    Ok(())
}
```

`AgentEvent::is_persistent()` separa los eventos **efímeros** (`TextDelta`, `ThinkingDelta`, `ToolUpdated`, …: se difunden pero no se guardan) de los **persistentes** (mensajes completos, herramientas terminadas, diálogos, salida del proceso).

**Paso 5 — difusión y cliente**: `TaskLifecycle::emit` publica en el bus; `sse.rs` filtra y serializa; en el navegador un `EventSource` actualiza signals ([§9](#9-frontend-leptos-y-signals)).

### 7.3 Finalizar, integrar y descartar

```mermaid
flowchart LR
    A[Idle/Running] -- FinishTask --> B["Finishing<br/>stop agente + commit_all"]
    B --> C[AwaitingReview]
    C -- "GetTaskDiff (git diff base..rama)" --> D[Usuario revisa]
    D -- MergeTask --> E{¿conflicto?}
    E -- no --> F["Merged<br/>worktree + rama eliminados"]
    E -- sí --> G["Conflict<br/>merge --abort, worktree conservado"]
    G -- ResumeTask --> A
    D -- DiscardTask --> H[Discarded]
```

`MergeTask` (resumen): antes de integrar fija el rango de commits con `worktrees.pin_range(...)` (debe ser *antes*: tras el merge el merge-base coincide con la punta) y crea la referencia `refs/iworkflow/tasks/<id>` para que los commits sobrevivan al borrado de la rama. Tras un merge correcto guarda el rango con `record_diff_range` (columnas `base_sha`/`head_sha` de `tasks`). Fijar y guardar son *best-effort*: un fallo no impide integrar, solo deja una nota en el error de la tarea. Así, `GetTaskDiff` en una tarea `Merged` regenera el diff con `worktrees.diff_range(...)` en lugar de `diff(base, rama)`. Luego llama a `worktrees.merge(...)`, que devuelve un `MergeOutcome`; con `match` se decide entre pasar a `Merged` (intentando limpiar el worktree) o `Conflict` con la lista de ficheros. Las reglas de `GitCliWorktreeAdapter` (rechazar si la rama base tiene cambios sin commitear, merge en worktree temporal si la rama base no está checkouteada, abortar ante conflictos) están en el adaptador, no en el caso de uso: son detalles de git.

### 7.4 Reinicio del servidor

`ReconcileTasks` (llamado desde `main`) busca tareas en `Starting`, `Running`, `Idle` o `Finishing` —que no pueden tener agente vivo tras un reinicio— y las pasa a `Interrupted` con un mensaje (avisando si el worktree ya no existe). Desde ahí el usuario puede **reanudar** (`ResumeTask` relanza el agente con `--continue`).

---

## 8. Persistencia

### Pool y conexión (`src/core/db/pool.rs`)

```rust
pub type DbConnectionRaw = SyncConnectionWrapper<SqliteConnection>;
pub type DbPool = Pool<DbConnectionRaw>;                 // deadpool

const CONNECTION_PRAGMAS: &str =
    "PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;";

pub const MIGRATIONS: EmbeddedMigrations = embed_migrations!("migrations");
```

- SQLite es síncrono; `SyncConnectionWrapper` lo envuelve para usarlo con `async`/`await`.
- WAL + `busy_timeout` reducen los bloqueos, pero con varios escritores aún pueden aparecer `database is locked`: de ahí `TaskError::Busy` y el decorador de reintentos.
- Las migraciones se compilan **dentro** del binario y se aplican en `main` antes de aceptar tráfico.

### Esquema y migraciones

`src/core/db/schema.rs` lo genera `diesel print-schema` (no se edita a mano). Tablas actuales: `workspaces`, `tasks`, `task_events`, `task_attachments`, `agent_settings`, `agents_files`. Cada migración en `migrations/<fecha>_<nombre>/` tiene `up.sql` y `down.sql`.

### Adaptador Diesel: patrón

Tres piezas por tabla:

1. **Row struct** (`persistence/models/task_row.rs`): representación en BD, con `#[derive(Queryable, Selectable, Insertable)]`. Solo tipos primitivos (`String`, `NaiveDateTime`, `i32`).
2. **Conversiones** `From<&Task> for TaskRow` y `TryFrom<TaskRow> for Task` (aquí `TryFrom` porque parsear un estado o un JSON puede fallar). Es el *mapper* entre dominio y persistencia.
3. **Adaptador** que implementa el puerto:

```rust
async fn claim_for_start(&self, id: TaskId, branch: String, worktree_path: PathBuf,
                         base_branch: String) -> Result<bool, TaskError> {
    let mut connection: DbConnection = self.connection().await?;     // saca del pool
    let updated: usize = diesel::update(
        tasks::table
            .filter(tasks::id.eq(id.to_string()))
            .filter(tasks::status.eq(TaskStatus::Backlog.as_str())),
    )
    .set((tasks::branch.eq(branch), tasks::status.eq(TaskStatus::Starting.as_str()), /* ... */))
    .execute(&mut connection)
    .await
    .map_err(db_error)?;                                              // DieselError → TaskError
    Ok(updated > 0)
}
```

Diesel es un *query builder* tipado en tiempo de compilación (a medio camino entre JOOQ y JPA Criteria): si `tasks::status` no existe o el tipo no encaja, **no compila**. No hay SQL en texto salvo los `PRAGMA`.

### Decorador de reintentos (`retrying_task_repository.rs`)

`RetryingTaskRepository` implementa `TaskRepositoryPort` **envolviendo** a otro `TaskRepositoryPort` (patrón Decorator). Cada método llama a `self.retry("nombre", || self.inner.método(...))`, que reintenta con backoff exponencial (25 ms → 1 s) mientras el error cumpla `is_transient()`. Se reintenta cada llamada por separado porque cada método del puerto es atómico (una sola sentencia o transacción).

---

## 9. Frontend: Leptos y Signals

### Modelo mental

Leptos es reactivo de grano fino (más parecido a Solid.js que a React): **no hay *virtual DOM* ni re-render del componente**. Una función `#[component]` se ejecuta **una vez**; lo que cambia son los nodos del DOM suscritos a signals concretos.

| Primitiva | Para qué | Analogía |
|---|---|---|
| `RwSignal<T>` / `signal()` | Estado local de UI | `useState` / `ObservableValue` |
| `Memo<T>` | Valor derivado y cacheado | `computed` |
| `Resource` | Lectura asíncrona vía server function (con SSR + Suspense) | `useQuery` |
| `ServerAction<F>` | Mutación vía server function; expone `pending()`, `value()`, `version()` | `useMutation` |
| `Effect` | Efecto secundario de UI (no para orquestar llamadas al backend) | `useEffect` |
| `provide_context` / `use_context` | Compartir estado con descendientes | `Context` |

`view! { ... }` es una macro con sintaxis tipo JSX. `<Suspense>`, `<Transition>` y `<ErrorBoundary>` gestionan carga y errores.

### Patrón *Page → State → Server fn*

```mermaid
flowchart LR
    P["board_page.rs<br/>BoardPage()"] -- "BoardState::new()" --> S["board_state.rs<br/>struct BoardState (Copy)"]
    S -- "Resource::new(.., list_tasks)" --> SF1[list_tasks]
    S -- "ServerAction&lt;StartTask&gt;" --> SF2[start_task]
    S -- "tick: RwSignal&lt;u32&gt;" --> R[(re-fetch)]
    SSE["EventSource /api/tasks/events"] -- "bump tick" --> S
    P --> C["BoardLane → TaskCard (componentes tontos)"]
```

`BoardState` (`presentation/state/board_state.rs`) es un struct `Copy` de signals (los signals de Leptos son *handles* baratos de copiar). Su `Resource` se vuelve a ejecutar cuando cambia su "clave": una tupla de contadores de versión de cada acción más `tick`.

```rust
let tasks: Resource<Result<Vec<TaskDto>, ServerFnError>> = Resource::new(
    move || (tick.get(), create.version().get(), delete.version().get(),
             start.version().get(), finish.version().get(), merge.version().get()),
    |_| list_tasks(None),
);
```

Lectura: "recarga la lista cada vez que una acción termine **o** `tick` cambie". `tick` lo incrementa el `EventSource` SSE cuando el servidor avisa de un cambio. Así el flujo es: acción del usuario / evento del servidor → cambia una dependencia → el `Resource` refetchea → la UI se actualiza solo donde cambió.

### Lógica fuera de los componentes

Los componentes (`task_card.rs`, `board_lane.rs`, …) solo pintan y reciben `Callback`s. Las reglas de presentación **puras** se extraen a funciones testeables en `state/`, por ejemplo `build_lanes(tasks, workspaces, stage_filter, show_done) -> Vec<LaneView>` (agrupa por workspace, filtra, ordena por atención/prioridad/antigüedad) y `card_key(&task)` (la clave del `<For>` que decide qué tarjetas reconstruir). Al ser funciones sin I/O, se prueban con `#[test]` normales sin navegador.

### Código solo de navegador

El `EventSource` (`task_events.rs`, `task_detail_state.rs`) usa `web_sys` y solo existe bajo `#[cfg(feature = "hydrate")]`; en SSR hay una versión vacía:

```rust
#[cfg(not(feature = "hydrate"))]
pub fn subscribe_task_events(_tick: RwSignal<u32>) {}

#[cfg(feature = "hydrate")]
pub fn subscribe_task_events(tick: RwSignal<u32>) { /* EventSource + reconexión + sondeo de respaldo 60 s */ }
```

### Shell, rutas y tema

`src/app.rs` define `shell()` (el HTML del documento con `HydrationScripts`) y `App()` con `<Router>` y las rutas:

| Ruta | Página |
|---|---|
| `/` | `BoardPage` (tablero kanban por workspace) |
| `/workspaces` | `WorkspacesPage` |
| `/tasks/:id` · `/tasks/:id/edit` | `TaskDetailPage` · `TaskEditPage` |
| `/terminals` · `/terminals/:session_id` · `/workspaces/:id/terminal` | Terminales |
| `/agents-files` | `AgentsFilesPage` |

`lib.rs` expone `hydrate()` (`#[wasm_bindgen]`), que monta la app sobre el HTML ya renderizado por el servidor.

---

## 10. Resto de módulos

Todos siguen la misma estructura que `tasks`, con menos piezas.

### `workspaces` — repositorios registrados

Un *workspace* es una carpeta con un repo git donde se lanzan tareas.

- Casos de uso: `RegisterWorkspace` (normaliza la ruta a la raíz del repo; si no existe/no es git y `initialize` es `true`, crea e inicializa; si no tiene commits, crea uno inicial vacío para poder crear worktrees), `ListWorkspaces`, `GetWorkspace`, `DeleteWorkspace` (borra tareas en cascada vía `WorkspaceTasksPort`), `BrowseDirectories` / `CreateDirectory` (selector de carpetas), `ListAgentsFileOptions`.
- Adaptadores: `WorkspaceRepositoryAdapter` (Diesel), `GitCliRepositoryAdapter` (`git`), `FsDirectoryBrowserAdapter`.
- Dependencias entre features: **inbound** de `tasks` y `agents_files` (consultan workspaces vía bridges); **outbound** hacia ambos (puertos `WorkspaceTasksPort`, `AgentsFilesPort`).

### `terminal` — terminales en el navegador

- Dominio: `TerminalSession`, `TerminalSize`, `TerminalSessionId`. Puertos: `PtyPort` (spawn, attach, write, resize, close), `WorkspacePathPort`.
- Adaptador: `PortablePtyAdapter` (crate `portable-pty`).
- **WebSocket** (`ws.rs`): frames binarios = bytes del PTY en ambas direcciones; frames de texto = JSON de redimensionado. La sesión **sobrevive** al socket (permite reconectar); solo termina con `close_terminal` o cuando acaba la shell. Valida que `Origin` coincida con `Host` (`is_same_origin`) para evitar secuestro de la shell desde otra web.
- No usa base de datos.

### `agent_settings` — cómo se lanza el agente

- Guarda en SQLite (una fila) el **runtime** (`gentle-shell` o `pi`), **modelo**, **perfil** y **nivel de thinking**.
- `PrepareLaunch` lee los ajustes, aplica el *pin* de perfil de la tarea (`ProfilePinPort`; `GitProfilePinAdapter` escribe `<git-common-dir>/gentle-ai/profile-pin.json`) y devuelve las `LaunchPreferences` que `tasks` usa al lanzar (a través de `LaunchSettingsBridge`).
- Adaptadores: `AgentSettingsRepositoryAdapter`, `AgentCliModelCatalogAdapter` (lista modelos ejecutando el CLI), `ProfilesFileCatalogAdapter` (lee `profiles.json`), `GitProfilePinAdapter`.

### `agents_files` — biblioteca de AGENTS.md

- Permite subir ficheros `AGENTS.md`, guardarlos (metadatos en SQLite, contenido en `~/.iworkflow/agents-files/`) e instalarlos en un workspace (`InstallAgentsFile`, con control de "ya existe").
- Adaptadores: `AgentsFileRepositoryAdapter` (Diesel), `FsAgentsFileStorageAdapter`, `GitAgentsFileInstallerAdapter`.
- Obtiene los "proyectos" de `workspaces` a través de `ProjectLookupBridge`.

---

## 11. Estrategia de tests

| Qué | Dónde | Cómo |
|---|---|---|
| Reglas de dominio | `#[cfg(test)] mod tests` junto al código | `#[test]` puros (`TaskStatus`, `after_agent_event`, `build_lanes`) |
| Casos de uso | `#[cfg(test)] mod tests` en cada `usecases/*.rs` | `#[tokio::test]` + mocks `mockall` de los puertos; `application/test_support.rs` ofrece una `Recorded` para inspeccionar estados y eventos emitidos |
| Mocks | `src/modules/<feature>/mocks.rs` | `mock! { pub TaskRepository {} #[async_trait] impl TaskRepositoryPort for TaskRepository { ... } }` — el equivalente a `@Mock` de Mockito generado a partir del trait |
| Adaptadores Diesel / FS / git / runtime | `tests/modules/<feature>/*_test.rs` | Integración contra SQLite temporal (`tempfile`) con migraciones reales aplicadas; `tests/common/mod.rs` provee `test_db()` y `git_repo()` |
| Runtime de agente | `tests/modules/tasks/gentle_shell_runtime_adapter_test.rs` | Procesos falsos en `tests/fixtures/` (`fake_agent.py`, `stubborn_agent.py`) |
| Resiliencia de la cola | `tests/modules/tasks/event_queue_resilience_test.rs` | Simula bloqueos de SQLite |
| UI | `end2end/` | Playwright (`run_smoke.sh`) con agente falso |

Ejemplo de test de caso de uso (patrón general; ver `finish_task.rs`):

```rust
#[tokio::test]
async fn stops_agent_commits_and_awaits_review() {
    let task: Task = test_support::task(TaskStatus::Idle);
    let recorded: Recorded = Recorded::default();
    let mut worktrees: MockWorktree = MockWorktree::new();
    worktrees.expect_commit_all()
        .withf(|path: &Path, message: &str| path == Path::new(WORKTREE) && message == "pi: Arregla el login")
        .times(1)
        .returning(|_, _| Ok(true));

    FinishTask::new(test_support::lifecycle(Some(task), &recorded), Arc::new(worktrees), /* ... */)
        .execute(id).await.expect("finalizada");

    assert_eq!(recorded.statuses(), vec![TaskStatus::Finishing, TaskStatus::AwaitingReview]);
}
```

Comandos:

```bash
cargo test --features ssr                                    # todo
cargo test --features ssr modules::tasks                     # un feature
cargo test --features ssr --test task_repository_adapter_test  # un test de integración
cargo fmt --all -- --check
cargo clippy --features ssr --all-targets -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown --lib -- -D warnings
cargo leptos watch                                           # desarrollo en http://127.0.0.1:3000
```

Los `[[test]]` de `Cargo.toml` declaran cada test de integración con `required-features = ["ssr"]` porque dependen del código solo-servidor.

---

## 12. Ruta de lectura recomendada

Orden sugerido para entender el código con el mínimo de saltos:

1. **`src/lib.rs`, `src/app.rs`** — qué se compila y qué rutas existen (5 min).
2. **`src/main.rs`** — arranque, cierre ordenado, dónde se inyectan los contextos.
3. **`src/app_state.rs`** — composición y *bridges*; lee solo `AppState::new` y un bridge.
4. **`src/core/db/pool.rs`** — pool, pragmas, migraciones.
5. **`modules/tasks/domain/models/task.rs`** — `TaskId`, `TaskStatus`, reglas de transición. Es el corazón del dominio.
6. **`modules/tasks/domain/ports/outbound/`** — lee los traits como si fueran las interfaces de un servicio Java.
7. **`modules/tasks/application/services/task_lifecycle.rs`** y luego `usecases/start_task.rs`, `finish_task.rs`, `merge_task.rs`.
8. **`modules/tasks/infrastructure/di/tasks_context.rs`** — ve cómo se enchufa todo.
9. **`modules/tasks/infrastructure/adapters/inbound/server_fns.rs`** + `dto/task_dto.rs`.
10. **`modules/tasks/infrastructure/adapters/outbound/persistence/task_repository_adapter.rs`** (y `retrying_task_repository.rs`).
11. **El pipeline de eventos**: `runtime/gentle_shell_runtime_adapter.rs` → `rpc_protocol.rs` → `queued_agent_event_sink.rs` → `handle_agent_event.rs` → `sse.rs`.
12. **`modules/tasks/presentation/state/board_state.rs`**, `pages/board_page.rs` y `state/task_events.rs`.
13. Después, `workspaces` y `terminal` como segundas lecturas: ya reconocerás el patrón y solo cambiarán los detalles.

### Checklist para añadir una funcionalidad (patrón de trabajo)

1. Modelo / regla en `domain/models` (con tests).
2. Si necesita algo externo, un trait en `domain/ports/outbound` + su mock en `mocks.rs`.
3. Caso de uso en `application/usecases` con tests `mockall`.
4. Adaptador en `infrastructure/adapters/outbound` + test de integración en `tests/modules/<feature>/`; si toca BD, migración con `up.sql`/`down.sql` y regenerar `schema.rs`.
5. Registrar el caso de uso en `<feature>_context.rs`.
6. DTO + server function (sin lógica) + test del mapeo y del error.
7. Estado en `presentation/state`, página/componente (sin lógica de negocio).

---

### Glosario rápido

| Término | Significado en este repo |
|---|---|
| **Slice / feature** | Directorio bajo `modules/` con sus propias capas |
| **Puerto** | Trait que define lo que el dominio necesita (outbound) u ofrece (inbound) |
| **Adaptador** | Implementación concreta de un puerto (Diesel, `git`, proceso, FS) o punto de entrada (server fn, SSE, WS) |
| **Caso de uso** | `struct` con `execute()`; una acción de usuario o de sistema |
| **Contexto (`XxxContext`)** | Struct con los casos de uso de un feature, inyectado con `provide_context` |
| **Bridge** | Adaptador en `app_state.rs` que conecta dos features sin que se importen entre sí |
| **Worktree** | Copia de trabajo git aislada por tarea, en `~/.iworkflow/worktrees` |
| **Sink** | Receptor de eventos del agente (`AgentEventSink`) |
| **SSR / hidratación** | El servidor renderiza HTML; el WASM lo "activa" en el navegador sin volver a pintarlo |
| **DTO** | Tipo serializable que cruza cliente ↔ servidor; el dominio nunca viaja |
