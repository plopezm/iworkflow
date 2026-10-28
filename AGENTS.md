# Agent Context & Instructions: Go (templ + htmx) Hexagonal Vertical Slice

## 1. Stack tecnológico
- Lenguaje: Go (última versión estable; `go.mod` fija la versión mínima)
- Web: `net/http` + `chi` (router), render SSR con `templ` e interactividad con `htmx`. JS propio solo donde htmx no llegue (terminal, follow-bottom, diffs) y siempre aislado en `web/static/`
- Tiempo real: SSE y WebSocket (`nhooyr.io/websocket` / `coder/websocket`) para streams de tareas, terminal y refrescos
- Acceso a datos: `sqlc` (queries tipadas generadas) sobre `database/sql`, con migraciones `goose` embebidas (`embed.FS`) que se aplican al arrancar
- Base de datos: SQLite con `modernc.org/sqlite` (puro Go, sin CGO) en `~/.iworkflow/workspaces.db` (override con `IWF_DATABASE_URL`)
- PTY: `creack/pty`. HTTP externo: `net/http` con timeouts explícitos
- Búsqueda código: usa codegraph, mantén actualizados los grafos en cada feature
- Diseño: Tailwind CSS (CLI standalone, salida en `web/static/`)
- Lint: `golangci-lint`. Mocks: `mockery` (o mocks manuales en `mocks_test.go`). Aserciones: `testify`

## 2. Orquestación

Utiliza siempre la metodología ODD con multiagente. La migración desde Rust se coordina con `odd/tasks/go-migration.md`: cada especificación `S#` es un criterio de paridad con el comportamiento Rust existente (tests y e2e actuales).

## 3. Arquitectura (Hexagonal + Vertical Slicing)

1. **Vertical Slicing**: cada feature es un paquete propio bajo `internal/modules/`.
2. **Arquitectura Hexagonal**: `domain` y `application` están en el centro, aislados de la infraestructura (SQL, HTTP, templ, htmx, `chi`).
3. **Use Case Driven**: cada acción del usuario o del sistema es un único caso de uso (struct con un método `Execute(ctx, input)`).
4. **Presentación desacoplada**: reutiliza los componentes de `presentation/components/` y de `internal/core/components/` antes de duplicar UI.

**Puertos:**
- **Outbound** (`domain/ports/outbound/`): siempre. Son `interface`s definidas en el dominio; el caso de uso depende de la interfaz y el adaptador de persistencia la implementa en `infrastructure`.
- **Inbound** (`domain/ports/inbound/`): solo si hace falta un contrato explícito (varios drivers, tests contra la interfaz). Si el handler HTTP es el único consumidor basta el struct del caso de uso.

Las interfaces son pequeñas y se definen donde se consumen. Todo método de I/O recibe `context.Context` como primer argumento.

### 3.1 Dependencias entre capas (se verifica con `depguard` / `go-arch-lint`)

- `domain` no importa nada del proyecto salvo `domain` y paquetes de la stdlib sin I/O.
- `application` solo importa `domain`.
- `infrastructure` importa `domain` y `application`. `presentation` solo habla con `application` a través de los handlers/DTOs de `infrastructure/adapters/inbound`.
- Prohibido en `domain` y `application`: `database/sql`, `net/http`, `chi`, `templ`, `sqlc` y cualquier paquete generado.
- `internal/core` no puede importar nada de `internal/modules/`.

### 3.2 Stack obligatorio: composition root (DI) + htmx (estado)

Es la **única forma autorizada** de inyección de dependencias y de estado de UI. No uses alternativas (variables globales, `init()` con efectos, singletons, frameworks de DI como wire/fx/dig, `context.Value` para servicios) salvo petición explícita.

### 3.3 Inyección de dependencias (servidor)

- **Mecanismo:** DI manual por constructores. El pool se crea en el arranque (`cmd/iworkflow/main.go`) y se inyecta en los módulos; no hay registro global.
- **Dónde:** el wiring de cada feature vive en `infrastructure/di/<feature>_module.go`, que expone un struct `Module` con los casos de uso y los handlers ya construidos y un método `Routes(r chi.Router)`. La composición global está en `internal/app/app.go`, que solo agrupa los módulos.
- **Qué registrar:** adaptadores outbound (dentro de los casos de uso) y casos de uso. Nunca la `*sql.DB` "a pelo" a un handler.
- **Prohibido:** instanciar adaptadores o pools dentro de handlers, componentes o casos de uso; ejecutar queries fuera de `infrastructure/adapters/outbound/persistence/`.

*Ejemplo de registro por feature:*

```go
// internal/modules/portfolio/infrastructure/di/portfolio_module.go
type Module struct {
	CreateProduct *application.CreateProduct
	handlers      *inbound.Handlers
}

func NewModule(db *sql.DB) *Module {
	repo := persistence.NewPortfolioRepository(db) // implementa outbound.PortfolioRepositoryPort
	create := application.NewCreateProduct(repo)
	return &Module{CreateProduct: create, handlers: inbound.NewHandlers(create)}
}

func (m *Module) Routes(r chi.Router) { m.handlers.Mount(r) }
```

```go
// cmd/iworkflow/main.go (extracto)
db, err := database.Open(ctx, databaseURL)
if err != nil { log.Fatal(err) }
application := app.New(db) // construye todos los Module
srv := &http.Server{Addr: addr, Handler: application.Router()}
```

### 3.4 Handlers HTTP — adaptador inbound

- **Dónde:** `infrastructure/adapters/inbound/handlers.go` (y ficheros por recurso).
- **Responsabilidad:** decodificar la petición (formulario, JSON o parámetros), invocar el caso de uso, mapear errores de dominio a código HTTP/fragmento de error y renderizar. **Sin lógica de negocio.**
- **Tipos de entrada/salida:** DTOs en `infrastructure/adapters/inbound/dto/`. No se exponen los structs generados por `sqlc` ni se serializa el dominio directamente.
- **Fragmentos htmx:** los handlers devuelven fragmentos templ cuando `HX-Request` está presente y la página completa en otro caso.

```go
func (h *Handlers) createProduct(w http.ResponseWriter, r *http.Request) {
	input, err := dto.DecodeCreateProduct(r)
	if err != nil { h.renderError(w, r, err); return }
	product, err := h.create.Execute(r.Context(), input.ToDomain())
	if err != nil { h.renderError(w, r, err); return }
	h.render(w, r, components.ProductRow(dto.FromProduct(product)))
}
```

### 3.5 Estado de UI (htmx + templ)

- **El servidor es la fuente de verdad.** El estado de la pantalla se deriva de los datos del caso de uso y se actualiza con `hx-get`, `hx-post`, `hx-swap`, `hx-target` y eventos `HX-Trigger`.
- **Dónde:** el estado de una pantalla se agrupa en `presentation/state/<flow>_state.go` (view-models inmutables que construye el handler), y las vistas en `presentation/pages/` y `presentation/components/` (`.templ`).
- **Cliente:** el JS local se limita a comportamiento puramente de UI (foco, scroll, terminal). Para estado reactivo ligero usa atributos `data-*` y módulos pequeños en `web/static/js/`; no se duplica lógica de negocio en el cliente.
- **Tiempo real:** SSE/WebSocket los expone el módulo y los consume htmx (`hx-ext="sse"`) o un módulo JS dedicado.
- **Prohibido:** lógica de negocio en `.templ`, llamadas a SQL desde presentación y estado de dominio compartido entre features.

*Flujo típico:*

```text
[templ page/component] ──> [htmx request] ──> [HTTP handler (inbound adapter)]
                                                        │
                                               [Use Case (application)]
                                                        │
[sqlc/SQL adapter (persistence)] <── [Outbound Port (interface)] <──┘
         ▲
         └── construido en <feature>_module.go y compuesto en internal/app/app.go
```

## 4. Estructura de Carpetas (Estrategia de Slicing)

Toda nueva funcionalidad se crea dentro de su propio módulo/feature. **Prohibido** crear estados globales, repositorios compartidos o servicios genéricos que crucen features sin pasar por el dominio.

**Código compartido:** lo transversal (pool, migraciones, shell/layout, router base, tema, componentes base, errores genéricos, utilidades sin dominio) vive en `internal/core/`. Si un feature necesita algo de otro, se expone vía puerto de dominio o se mapea en la composition root (`internal/app/app.go`), nunca con un import directo entre features.

```text
go.mod / go.sum
sqlc.yaml                             # genera internal/core/db/gen (o por feature)
Makefile                              # build, generate (templ, sqlc, tailwind), test, lint
.golangci.yml
migrations/                           # goose: NNNN_nombre.sql (-- +goose Up / Down)
cmd/
└── iworkflow/
    └── main.go                       # Arranque: config, pool, migraciones, servidor
internal/
├── app/
│   └── app.go                        # Composición de módulos y router
├── core/                             # Transversal (sin imports de modules/)
│   ├── db/                           # Open, pragmas, migraciones embebidas, tx helper
│   ├── config/                       # config, env
│   ├── i18n/
│   ├── web/                          # shell, layout, middlewares, helpers htmx/SSE
│   ├── components/                   # Componentes templ base reutilizables
│   └── ...                           # git, process, markdown, etc. sin dominio de feature
└── modules/
    └── <feature_name>/               # p. ej. workspaces, tasks, jira
        ├── mocks_test.go             # mocks de los puertos del feature (package <feature>_test o interno)
        ├── domain/                   # Núcleo de negocio (Go puro)
        │   ├── models/               # Entidades y value objects
        │   ├── errors/               # Errores de negocio (sentinelas y tipos)
        │   └── ports/
        │       ├── inbound/          # Interfaces de casos de uso (si hace falta)
        │       └── outbound/         # Interfaces hacia infraestructura
        ├── application/              # Un fichero por caso de uso (+ _test.go)
        ├── infrastructure/
        │   ├── adapters/
        │   │   ├── inbound/
        │   │   │   ├── dto/
        │   │   │   └── handlers.go
        │   │   └── outbound/
        │   │       ├── api/          # Clientes HTTP externos
        │   │       └── persistence/  # sqlc/SQL: queries/*.sql, repositorios, mapeos fila↔dominio
        │   └── di/
        │       └── <feature>_module.go
        └── presentation/             # templ
            ├── state/
            ├── pages/
            └── components/
web/
├── static/                           # CSS compilado, JS, assets
└── tailwind.css
tests/                                # Integración (SQLite temporal) por feature
└── modules/<feature_name>/
end2end/                              # Playwright (se reutilizan los e2e actuales como criterio de paridad)
```

## 5. Testing: Cobertura de Tests Obligatoria (Test-Driven Mindset)
- **Regla:** cada *feature*, caso de uso, adaptador, handler o estado de pantalla nuevo incluye sus pruebas.
- **Estándar:**
  - Casos de uso: tests unitarios en `*_test.go` junto al código, table-driven, con mocks de los puertos outbound.
  - Adaptadores SQL: tests de integración en `tests/modules/<feature>/` contra un SQLite temporal por test (`t.TempDir()`), con las migraciones embebidas aplicadas.
  - Handlers y DTOs: tests de mapeo con `httptest` (DTO ↔ dominio, error de dominio → código HTTP y fragmento).
  - Componentes templ: tests de render (`templ` + comparación de HTML) cuando aporten valor.
  - La UI necesita tests Playwright por feature en `end2end/` para asegurar el caso de uso en navegador.
- **Paridad:** un módulo migrado no se da por terminado hasta que su e2e actual pasa contra la versión Go y sus tests Rust equivalentes tienen contraparte Go.
- Cada feature mantiene sus mocks en `mocks_test.go` (o `mocks/` generado con `mockery`).

## 6. Reglas Estrictas de Código (Strict Rules)
- Código formateado con `gofmt`/`goimports`; sin warnings de `golangci-lint`.
- Inmutabilidad por defecto: entidades y value objects se construyen con constructores (`NewX`) que validan invariantes; los campos de dominio no se exportan si pueden romper invariantes.
- Valores constantes con `const` (o variables de paquete no exportadas e inmutables); sin globales mutables.
- Idiomático: se permite `:=`; los tipos explícitos son obligatorios en firmas, campos y APIs exportadas (adaptación de la regla Rust de anotar cada `let`).
- Los componentes templ no contienen lógica de negocio; se crean lo más genéricos posible (parámetros tipados, `templ.Component` hijos, slots).
- Todo error se propaga con `fmt.Errorf("...: %w", err)` y se comprueba con `errors.Is/As`; prohibido ignorar errores (`_ =`) salvo justificación comentada.
- Prohibido `panic` fuera de `main.go` y de tests; prohibido `log.Fatal` fuera de `main.go`.
- Toda goroutine tiene dueño: usa `errgroup`/`context` y espera su finalización; prohibidas las goroutines fire-and-forget. Respeta la cancelación de `context.Context`.
- Prohibido `unsafe` y `cgo` salvo petición explícita.
- Nada de SQL concatenado: SQL solo en `queries/*.sql` (sqlc) con parámetros; SQL crudo únicamente para pragmas/configuración.
- Tiempos y IDs se inyectan (`Clock`, `IDGenerator`) para poder testear.

## 7. Comandos

Ejecutar desde la raíz (donde está `go.mod`). Requiere `go`, `templ`, `sqlc`, `goose`, `golangci-lint` y Tailwind CLI.

```bash
# Generación (deben ejecutarse tras tocar .templ, queries SQL o CSS)
make generate        # templ generate && sqlc generate && tailwind build

# Desarrollo
make dev             # servidor con recarga

# Migraciones
goose -dir migrations sqlite3 "$IWF_DATABASE_URL" up
goose -dir migrations sqlite3 "$IWF_DATABASE_URL" redo   # verifica que Down revierte

# Tests
go test -p 2 ./internal/modules/<feature_name>/...          # unitarios de un feature
go test -p 2 ./tests/modules/<feature_name>/...             # integración de un feature
go test -p 2 -race ./...                                    # suite completa

# E2E (si existen)
make e2e

# Formato y análisis estático (deben terminar sin errores ni warnings)
gofmt -l .
go vet ./...
golangci-lint run
```

### 7.1 Límites de memoria y comandos largos

- Todo comando `go` (build, test, lint) se ejecuta con `-p 2` (o `GOMAXPROCS`/`GOFLAGS=-p=2`) para limitar la memoria.
- Los tests de integración se lanzan por feature, nunca todos a la vez en iteración.
- Comandos largos (compilación inicial, lint, suites): en segundo plano con `nohup ... > /tmp/<nombre>.log 2>&1 &` y consulta del log con `tail`; nunca en primer plano sin timeout.
- Esto aplica también a los subagentes: inclúyelo en sus instrucciones.

## 8. Finalización (Definition of Done)

No dar una tarea por terminada hasta cumplir todo lo aplicable:

1. El código nuevo sigue la estructura de `internal/modules/<feature_name>/` y el stack composition root (DI) + htmx/templ + handlers + sqlc.
2. Existen tests (unitarios, de integración y handlers) para los casos de uso, adaptadores y handlers tocados, y mocks del feature si aplica.
3. No hay imports de `infrastructure` ni `presentation`, ni de `database/sql`, `net/http`, `chi` o `templ`, dentro de `domain` ni `application`.
4. Las migraciones nuevas tienen `Up` y `Down`, `goose redo` funciona y el código `sqlc` está regenerado.
5. Los ficheros generados (`*_templ.go`, `sqlc`) están al día (`make generate` sin diff).
6. Se han ejecutado y pasado: `gofmt -l .` (vacío), `go vet ./...`, `golangci-lint run` y `go test` de los paquetes afectados (y su e2e si existe).
7. Para módulos migrados: paridad con el comportamiento Rust verificada (e2e y specs `S#` de `odd/tasks/go-migration.md`).
8. Al reportar el cierre: indicar comandos ejecutados, resultado (ok/fallo) y qué quedó sin verificar.
9. Al mezclar a la rama principal, verifica que las migraciones no se solapan.
