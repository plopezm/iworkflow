# Rust for a Java developer

> Part of the [iworkflow architecture guide](../ARCHITECTURE.md). 🇪🇸 [Versión en castellano](rust-for-java.es.md)

An equivalence table you will constantly need while reading this repo.

| Java | Rust in this project | Notes |
|---|---|---|
| `interface Foo` | `trait Foo` | Outbound ports are traits (`TaskRepositoryPort`). |
| Inject `Foo` via constructor | `Arc<dyn Foo>` as a field | `Arc` = shared pointer with atomic reference counting (like a Java reference where you also know when it is released). `dyn` = dynamic dispatch (vtable), which in Java is the norm and in Rust is explicit. |
| `class` with fields + methods | `struct` + `impl` | Data and methods are declared separately. There is no inheritance. |
| Simple `enum` | `enum` | But Rust enums **carry data** (they are *sum types*): `AgentEvent::ProcessExited { code }`. They are equivalent to a `sealed` hierarchy of records. |
| `switch` / pattern matching | `match` | Exhaustive: if you add a variant, the compiler points out every `match` that is missing it. |
| `Optional<T>` / `null` | `Option<T>` | There is no `null`. `Some(x)` / `None`. |
| `throws` / exceptions | `Result<T, E>` | Errors are values. `?` propagates the error to the caller (like an implicit `throw` but visible in the signature). |
| `try/catch` | `match` on `Result`, `if let Err(e)`, `.map_err(...)` | `map_err` converts one error type into another (e.g. from Diesel to `TaskError`). |
| Lombok `@Data`, `equals` | `#[derive(Debug, Clone, PartialEq, ...)]` | `derive` generates trait implementations. |
| Constructor / factory | `fn new(...) -> Self` | A convention, not syntax. |
| `final` | Immutable by default | `let x` is immutable, `let mut x` is mutable. AGENTS.md requires `mut` only when needed. |
| `package` / `import` | `mod` / `use` | The module hierarchy follows the folder tree; each folder has a `mod.rs`. |
| `public` / package-private | `pub` / module-private | Everything is private to its module by default. |
| Generic types | Generics with *trait bounds* | `fn retry<T, F, Fut>(...) where F: FnMut() -> Fut` |
| Lambdas | Closures `\|x\| x + 1` | `move \|\| ...` captures by value (needed when passing closures to another thread/task). |
| `CompletableFuture` | `async fn` + `.await` | Rust futures are **lazy**: they do nothing until awaited with `.await` (or launched with `tokio::spawn`). |
| Spring `@Transactional` / pool | `deadpool` + Diesel | See [persistence](../ARCHITECTURE.md#8-persistence). |
| Mockito | `mockall` | `mock! { ... }` generates doubles for traits. |
| `Thread` / `ExecutorService` | `tokio::spawn` | Lightweight async tasks over a thread pool. |
| `BlockingQueue` | `tokio::sync::mpsc` | Multi-producer / single-consumer channel. |
| `synchronized` / `ReentrantLock` | `Mutex<T>` | The mutex **owns** the data: it is only reachable through the guard. |

### Concepts with no direct equivalent

**Ownership and borrowing.** Every value has a single owner; passing a value *moves* it. To read without taking ownership you borrow: `&T` (read, many at once) or `&mut T` (write, only one). You will see `&self` (method that only reads), `&mut self` (mutates) and `self` (consumes the object). This replaces the garbage collector: there is no GC, the compiler decides when each thing is freed.

**`Arc<T>` and `.clone()`.** Cloning an `Arc` does **not** copy the data, it only increments a counter. That is why the contexts (`TasksContext`, etc.) are `#[derive(Clone)]` and are cloned freely: each field is an `Arc<UseCase>`.

**`Send + Sync`.** Markers saying "this type can be moved/shared across threads". Async traits are declared `trait X: Send + Sync` because the tokio runtime may move tasks between threads.

**`#[async_trait::async_trait]`.** Rust still does not comfortably allow `async fn` in traits used as `dyn Trait`; this macro solves it. It appears on every port and adapter.

**Macros (`!` and `#[...]`).** `view! { ... }`, `mock! { ... }`, `#[server]`, `#[derive(...)]`, `#[component]` generate code at compile time. Think of them as a far more powerful Java annotation processor.

**`#[cfg(feature = "ssr")]` attributes.** Conditional compilation. The same crate is compiled twice with different sets of code (see next section). It is the central mechanism of this project.

**Newtypes.** `pub struct TaskId(Uuid);` is a different type from `Uuid` even though it wraps one: it prevents mixing up identifiers. Equivalent to a Java *value object*.

**Enums with state and their own `impl`.** `TaskStatus` is an enum with methods (`can_finish()`, `after_agent_event()`): the domain is **rich**, not anemic.
