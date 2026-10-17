# Rust para un desarrollador Java

> Parte de la [guía de arquitectura de iworkflow](../ARCHITECTURE.es.md). 🇬🇧 [English version](rust-for-java.md)

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
| Spring `@Transactional` / pool | `deadpool` + Diesel | Ver [persistencia](../ARCHITECTURE.es.md#8-persistencia). |
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
