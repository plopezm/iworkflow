# Agent Context & Instructions: Rust (Leptos + Diesel) Hexagonal Vertical Slice

## 1. Stack tecnológico
- Lenguaje: Rust (edición 2021 o superior)
- Framework: Leptos full-stack (SSR + hidratación, reactividad con Signals) sobre `axum`, gestionado con `cargo-leptos`
- ORM: Diesel (`diesel` + `diesel-async` con `SyncConnectionWrapper<SqliteConnection>` y pool `deadpool`), migraciones con `diesel_cli` y embebidas (`embed_migrations!`) que se aplican al arrancar
- Base de datos: SQLite (`libsqlite3-sys` bundled) en `~/.pi-dashboard/workspaces.db` (override con `DATABASE_URL`)
- Búsqueda código: Usa codegraph, mantén actualizado los grafos en cada feature
- Diseño: Usa tailwind css

## 2. Orquestación

Utiliza siempre la metodología ODD con multiagente para el desarrollo de las features.


## 3. Arquitectura (Hexagonal + Vertical Slicing)

Combinamos los conceptos clave para mantener el sistema desacoplado, testeable y escalable:
1. **Vertical Slicing**.
2. **Arquitectura Hexagonal**: Dentro de cada corte vertical, el **Dominio** y los **Casos de Uso** están en el centro, completamente aislados de la infraestructura externa (Diesel, SQLite, HTTP, Leptos, axum).
3. **Use Case Driven (Orientado a Casos de Uso)**: Cada acción del usuario o sistema está representada por un único Caso de Uso (struct con un método `execute`) que orquesta la lógica.
4. **Presentación desacoplada**: Reutiliza componentes dentro de `presentation/components/` antes de duplicar UI.

**Puertos:**
- **Outbound** (`domain/ports/outbound/`): siempre. Son `trait`s; el caso de uso depende del trait (`Arc<dyn OrderRepositoryPort>`) y el adaptador Diesel lo implementa en `infrastructure`.
- **Inbound** (`domain/ports/inbound/`): solo si hace falta un contrato explícito (varios drivers, tests contra el trait, o el proyecto ya lo usa). Si la server function es el único consumidor, basta el struct del use case en `application/usecases/` sin trait inbound.

**Traits asíncronos:** los puertos que se usen como `dyn Trait` se declaran con `#[async_trait::async_trait]` y con bound `Send + Sync`.

### 3.1 Compilación dual: `ssr` / `hydrate`

Leptos compila el mismo crate dos veces: servidor (feature `ssr`) y cliente WASM (feature `hydrate`).

- `domain` y `application` solo se compilan en el servidor salvo los modelos que viajen al cliente (ver 3.3).
- Todo lo que dependa de Diesel, `diesel-async`, `deadpool`, `axum` o `tokio` va detrás de `#[cfg(feature = "ssr")]` y sus dependencias en `Cargo.toml` son `optional = true` activadas solo por `ssr`.
- Nunca debe acabar Diesel ni código de acceso a datos en el bundle WASM.

### 3.2 Stack obligatorio: Contexto de Leptos/axum (DI) + Signals (estado)

Estas herramientas son **la única forma autorizada** de inyección de dependencias y gestión de estado. No uses alternativas (singletons con `lazy_static`/`OnceCell` para servicios, `static mut`, stores globales de terceros, llamadas a Diesel desde componentes, etc.) salvo que el usuario lo pida explícitamente.

### 3.3 Inyección de dependencias (servidor)

- **Mecanismo:** el pool de SQLite y los casos de uso se construyen en el arranque (`main.rs`) y se inyectan en las server functions mediante `provide_context` usando `leptos_routes_with_context` / `handle_server_fns_with_context` de `leptos_axum`.
- **Dónde:** el wiring de cada feature vive en `infrastructure/di/[feature]_context.rs`, que expone un struct `[Feature]Context` con los casos de uso ya construidos. La composición global se hace en `src/app_state.rs`, que solo agrupa los contextos de features.
- **Qué registrar:** adaptadores outbound (dentro de los casos de uso) y casos de uso. Nunca el pool "a pelo" para que una server function haga queries.
- **Cómo consumir:** en la server function con `expect_context::<OrderContext>()` (o `use_context` devolviendo `ServerFnError` si falta).
- **Prohibido:** instanciar adaptadores o pools dentro de componentes, server functions o casos de uso; ejecutar queries Diesel fuera de `infrastructure/adapters/outbound/persistence/`.

*Ejemplo de registro por feature:*

```rust
// src/modules/portfolio/infrastructure/di/portfolio_context.rs
#[cfg(feature = "ssr")]
#[derive(Clone)]
pub struct PortfolioContext {
    pub create_product: Arc<CreateProduct>,
}

#[cfg(feature = "ssr")]
impl PortfolioContext {
    pub fn new(pool: DbPool) -> Self {
        let repository: Arc<dyn PortfolioRepositoryPort> =
            Arc::new(PortfolioRepositoryAdapter::new(pool));
        Self {
            create_product: Arc::new(CreateProduct::new(repository)),
        }
    }
}
```

```rust
// src/main.rs (extracto)
let pool: DbPool = build_pool(&database_url).await?;
let portfolio: PortfolioContext = PortfolioContext::new(pool.clone());

let router: Router = Router::new()
    .leptos_routes_with_context(
        &leptos_options,
        routes,
        move || provide_context(portfolio.clone()),
        { let options = leptos_options.clone(); move || shell(options.clone()) },
    );
```

### 3.4 Server functions — adaptador inbound

- **Dónde:** `infrastructure/adapters/inbound/server_fns.rs`, anotadas con `#[server]`.
- **Responsabilidad:** deserializar la entrada, obtener el caso de uso del contexto, invocarlo y mapear errores de dominio a `ServerFnError`. **Sin lógica de negocio.**
- **Tipos que viajan cliente↔servidor:** DTOs con `Serialize`/`Deserialize` en `infrastructure/adapters/inbound/dto/` (compilados en ambas features). No se exponen los structs de Diesel.

```rust
#[server]
pub async fn create_product(input: CreateProductDto) -> Result<ProductDto, ServerFnError> {
    let ctx: PortfolioContext = expect_context::<PortfolioContext>();
    let product: Product = ctx
        .create_product
        .execute(input.into())
        .await
        .map_err(|e: PortfolioError| ServerFnError::new(e.to_string()))?;
    Ok(ProductDto::from(product))
}
```

### 3.5 Signals — gestión de estado de UI

- **Primitivas:** `signal` / `RwSignal` para estado local de UI, `Memo` para derivados, `Resource` para lecturas asíncronas vía server functions, `ServerAction` / `Action` (o `ActionForm`) para mutaciones.
- **Dónde:** el estado de una pantalla se agrupa en `presentation/state/[flow]_state.rs` (struct con signals/resources/actions y un constructor `new()`), que se crea en la página y se comparte con sus hijos vía `provide_context` si hace falta.
- **Responsabilidad del estado:** traducir interacciones en llamadas a server functions, exponer carga/éxito/error y datos listos para pintar. **Sin lógica de negocio duplicada:** la orquestación vive en `application/usecases/`.
- **UI:** páginas y componentes usan `<Suspense>`, `<Transition>`, `<ErrorBoundary>` y leen signals; no llaman a Diesel, ni a adaptadores, ni a casos de uso directamente.
- **Prohibido:** estado de dominio en signals globales compartidos entre features, `Effect` para coordinar llamadas al backend (usa `Resource`/`Action`), o contextos que sustituyan al estado de pantalla.

*Flujos típicos:*

```text
[Page / Component] ──> [Signals / Resource / Action] ──> [#[server] fn (inbound adapter)]
                                                                   │
                                                          [Use Case (application)]
                                                                   │
[Diesel Adapter (persistence)] <── [Outbound Port (trait)] <───────┘
         ▲
         └── construido en [feature]_context.rs y provisto vía provide_context
```

## 4. Estructura de Carpetas (Estrategia de Slicing)

Toda nueva funcionalidad debe crearse dentro de su propio módulo/feature. Queda **estrictamente prohibido** crear estados globales, repositorios compartidos o servicios genéricos que crucen features sin pasar por el dominio.

**Código compartido:** lo transversal (pool de SQLite, `schema.rs` de Diesel, shell/app, router, tema, componentes base, errores genéricos, utilidades sin dominio de un feature) vive en `src/core/` (o `src/shared/` si el proyecto ya usa ese nombre). `core`/`shared` **no** puede importar nada de `modules/`. Si un feature necesita algo de otro, se expone vía puerto de dominio o se mapea en la composition root (`app_state.rs` / `main.rs`) — no se importa el otro feature a pelo.

Sigue este patrón de estructura de archivos:

```text
Cargo.toml                            # features: ssr / hydrate; [package.metadata.leptos]
diesel.toml                           # print_schema -> src/core/db/schema.rs
migrations/                           # Migraciones Diesel (up.sql / down.sql)
src/
├── main.rs                           # Arranque axum + leptos_axum (solo ssr)
├── lib.rs                            # Entrada hydrate
├── app.rs                            # Shell, <Router>, rutas
├── app_state.rs                      # Composición de contextos de features (ssr)
├── core/                             # Transversal (sin imports de modules/)
│   ├── db/
│   │   ├── pool.rs                   # DbPool (deadpool + SyncConnectionWrapper<SqliteConnection>)
│   │   └── schema.rs                 # Generado por diesel print-schema
│   └── components/                   # Componentes base reutilizables
└── modules/
    └── [feature_name]/               # Ejemplo: portfolio, taxation
        ├── mod.rs
        ├── mocks.rs                  # #[cfg(test)] mocks mockall del feature
        ├── domain/                   # Núcleo de negocio (Rust puro: sin Leptos, Diesel ni axum)
        │   ├── models/               # Entidades y value objects (order.rs, product_id.rs)
        │   ├── errors/               # Errores de negocio con thiserror (order_error.rs)
        │   └── ports/
        │       ├── inbound/          # Traits de casos de uso (create_order_use_case.rs)
        │       └── outbound/         # Traits hacia infraestructura (order_repository_port.rs)
        │
        ├── application/              # Orquestación de la lógica de negocio
        │   └── usecases/
        │       ├── create_order.rs
        │       └── cancel_order.rs
        │
        ├── infrastructure/           # Adaptadores técnicos y DI del slice
        │   ├── adapters/
        │   │   ├── inbound/
        │   │   │   ├── dto/          # DTOs serde cliente <-> servidor (order_dto.rs)
        │   │   │   └── server_fns.rs # #[server] functions
        │   │   └── outbound/
        │   │       ├── api/          # Clientes HTTP externos (reqwest), si aplica
        │   │       └── persistence/  # Diesel (solo ssr)
        │   │           ├── models/   # Structs Queryable / Insertable / AsChangeset (order_row.rs)
        │   │           └── order_repository_adapter.rs
        │   └── di/
        │       └── [feature]_context.rs
        │
        └── presentation/             # UI Leptos: componentes, páginas y estado
            ├── state/
            │   └── order_list_state.rs
            ├── pages/
            │   └── order_list_page.rs
            └── components/
                └── order_list_item.rs

tests/                                # Tests de integración (SQLite temporal)
└── modules/
    └── [feature_name]/
        └── order_repository_adapter_test.rs
end2end/                              # Tests e2e (Playwright vía cargo-leptos), solo si aportan valor
```

## 5. Testing: Cobertura de Tests Obligatoria (Test-Driven Mindset)
- **Regla:** Cada *feature*, caso de uso, adaptador o estado de pantalla nuevo debe incluir obligatoriamente sus pruebas correspondientes.
- **Estándar:**
  - Casos de uso: tests unitarios en `#[cfg(test)] mod tests` junto al código, con `#[tokio::test]` y `mockall` sobre los puertos outbound.
  - Adaptadores Diesel: tests de integración en `tests/modules/[feature_name]/` contra un fichero SQLite temporal (`tempfile`) por test, con las migraciones embebidas aplicadas.
  - Server functions y DTOs: tests de mapeo (DTO ↔ dominio, error de dominio → `ServerFnError`).
  - Estado de UI / componentes: `wasm-bindgen-test` o e2e con Playwright (`end2end/`) solo cuando aporten valor real.
- Se deberá crear un fichero `mocks.rs` para cada feature (`src/modules/[feature_name]/mocks.rs`, bajo `#[cfg(test)]`), donde se definirán los mocks con `mockall::mock!`.

## 6. Reglas Estrictas de Código (Strict Rules)
- Inmutabilidad por defecto: solo se usa `mut` cuando la variable deba reasignarse o mutarse.
- Los valores constantes se declaran con `const` (o `static` si requieren dirección fija), en `SCREAMING_SNAKE_CASE`.
- Todo binding `let` indica su tipo explícitamente (`let total: Decimal = ...`). Excepción: patrones de desestructuración y argumentos de closures donde la anotación no sea posible o sea redundante con la firma.
- Los componentes no contendrán lógica de negocio en ninguno de los casos.
- Los componentes se crearán de la forma más genérica posible (props tipadas, `Children`, `#[prop(into)]`) para su reutilización.
- Siempre hay que hacer `.await` de los futures; nunca se descartan.
- Evitar tareas fire-and-forget (`tokio::spawn`, `spawn_local`) sin gestionar su resultado o `JoinHandle`.
- Prohibido `unwrap()` / `expect()` / `panic!` fuera de tests y del arranque en `main.rs`; los errores se propagan con `Result` y `?`, definidos con `thiserror`.
- Prohibido `unsafe` salvo petición explícita del usuario.
- Nada de SQL crudo (`sql_query`) salvo que el query builder de Diesel no lo permita, y siempre con parámetros `bind`.

## 7. Comandos

Ejecutar desde la raíz del proyecto (donde está el `Cargo.toml`). Requiere `cargo-leptos`, `diesel_cli` (con feature `sqlite-bundled`) y el target `wasm32-unknown-unknown`. Comandos útiles según el alcance del cambio:

```bash
# Descarga/compilación de dependencias
cargo fetch
cargo leptos build

# Desarrollo con recarga
cargo leptos watch

# Base de datos (usa DATABASE_URL=ruta al .db; la app aplica migraciones al arrancar)
diesel setup
diesel migration generate [nombre]
diesel migration run
diesel migration redo          # Verifica que down.sql revierte correctamente

# Testear la aplicación
cargo leptos test
# o directamente la parte servidor
cargo test --features ssr

# Tests de un feature concreto
cargo test --features ssr modules::[feature_name]

# Un test de integración concreto
cargo test --features ssr --test order_repository_adapter_test

# E2E (si existen)
cargo leptos end-to-end

# Formato y análisis estático (deben terminar sin errores ni warnings)
cargo fmt --all -- --check
cargo clippy --features ssr -- -D warnings
cargo clippy --features hydrate --target wasm32-unknown-unknown -- -D warnings
```

## 8. Finalización (Definition of Done)

No dar una tarea por terminada hasta cumplir todo lo aplicable:

1. El código nuevo sigue la estructura de `src/modules/[feature_name]/` y el stack contexto (DI) + Signals + server functions + Diesel.
2. Existen tests (unitarios en el módulo y/o de integración en `tests/modules/[feature_name]/`) para los casos de uso, adaptadores y server functions tocados y, si aplica, `mocks.rs` del feature.
3. No hay `use` de `infrastructure` ni `presentation`, ni de `leptos`, `diesel` o `axum`, dentro de `domain` ni `application`.
4. Las migraciones nuevas tienen `up.sql` y `down.sql`, `diesel migration redo` funciona y `schema.rs` está regenerado.
5. Se han ejecutado y han pasado:
   - `cargo fmt --all -- --check`
   - `cargo clippy` para `ssr` y `hydrate` con `-D warnings`
   - `cargo leptos test` / `cargo test --features ssr` (o al menos los tests del feature/archivos afectados)
6. Comprobar que se ha hecho todo lo indicado en [Auto-Invoke Skills](#2-auto-invoke-skills).
7. Al reportar el cierre: indicar comandos ejecutados, resultado (ok/fallo) y qué quedó sin verificar.
8. Al mezclar a la rama principal, verifica que las migraciones no se solapan.
