use leptos::prelude::*;
use leptos_router::hooks::use_location;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    list_tasks, list_workspace_options,
};
use crate::modules::tasks::presentation::state::task_events::{
    TaskStreamEvent, subscribe_task_events,
};
use crate::modules::tasks::presentation::state::task_merge::apply_task_removal;

/// Prefijo de las rutas que crean/editan/borran workspaces: al entrar, salir o moverse por
/// ellas se recargan las opciones de workspace de la barra lateral y el tablero.
const WORKSPACES_AREA_PREFIX: &str = "/workspaces";

/// Clave de recarga de los workspaces según la ruta: solo cambia dentro/fuera del área de
/// workspaces, de modo que navegar entre tareas o el tablero no vuelve a pedirlos.
pub fn workspaces_area_key(pathname: &str) -> String {
    if pathname.starts_with(WORKSPACES_AREA_PREFIX) {
        pathname.to_string()
    } else {
        String::new()
    }
}

/// Una carga completa quedó obsoleta si, tras empezar, llegó un parche (`changed`/`removed`)
/// y ya había datos: aplicarla pisaría el parche más reciente, así que se descarta y se
/// vuelve a pedir. La primera carga siempre se aplica (los parches previos no tenían base).
pub fn reload_is_stale(started_gen: u32, current_gen: u32, loaded: bool) -> bool {
    loaded && started_gen != current_gen
}

/// Almacén compartido de tareas activas (y finalizadas visibles) y de opciones de
/// workspace. Se crea una sola vez en el shell de la app, junto con la única suscripción
/// SSE, y se consume por contexto desde la barra lateral y el tablero.
///
/// Carga completa (`reload`): arranque, `reset`/`lagged` del stream y sondeo de respaldo.
/// Un `changed` con id pide solo esa tarea (`get_task`) y la aplica con `apply_task_change`;
/// un `removed` la descarta sin pedir nada.
#[derive(Clone, Copy)]
pub struct TaskStore {
    reload: RwSignal<u32>,
    /// Parches aplicados hasta ahora (se incrementa en cada `changed`/`removed`).
    patch_gen: RwSignal<u32>,
    /// Valor de `patch_gen` cuando empezó la última carga completa.
    load_started_gen: RwSignal<u32>,
    /// Lectura inicial (SSR/hidratación) y recargas completas de las tareas activas.
    pub tasks: Resource<Result<Vec<TaskDto>, ServerFnError>>,
    pub workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>>,
    /// Tareas activas al día (la carga completa y los parches de SSE).
    pub active: RwSignal<Vec<TaskDto>>,
    /// Tareas finalizadas cargadas por el tablero; solo se parchean con `show_done`.
    pub done: RwSignal<Vec<TaskDto>>,
    /// `true` mientras el tablero muestra las finalizadas.
    pub show_done: RwSignal<bool>,
    /// Ya llegó la primera carga completa de `active`.
    pub loaded: RwSignal<bool>,
}

impl TaskStore {
    /// Crea el almacén y abre la suscripción SSE. Debe llamarse una vez, dentro del
    /// `<Router>` y por encima de la barra lateral y las páginas, y proveerse por contexto.
    pub fn new() -> Self {
        let pathname: Memo<String> = use_location().pathname;
        let reload: RwSignal<u32> = RwSignal::new(0);
        let patch_gen: RwSignal<u32> = RwSignal::new(0);
        let load_started_gen: RwSignal<u32> = RwSignal::new(0);
        let tasks: Resource<Result<Vec<TaskDto>, ServerFnError>> = Resource::new(
            move || reload.get(),
            move |_| {
                load_started_gen.set(patch_gen.get_untracked());
                list_tasks(None)
            },
        );
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> = Resource::new(
            move || {
                (
                    reload.get(),
                    pathname.with(|p: &String| workspaces_area_key(p)),
                )
            },
            |_| list_workspace_options(),
        );
        let store: Self = Self {
            reload,
            patch_gen,
            load_started_gen,
            tasks,
            workspaces,
            active: RwSignal::new(Vec::new()),
            done: RwSignal::new(Vec::new()),
            show_done: RwSignal::new(false),
            loaded: RwSignal::new(false),
        };
        // Copia la carga completa a `active` (solo cliente, tras hidratar). Es el único
        // `Effect`: no coordina llamadas al backend, solo vuelca el resultado del `Resource`
        // en la señal que también modifican los parches de SSE.
        Effect::new(move |_| {
            if let Some(Ok(loaded)) = store.tasks.get() {
                if reload_is_stale(
                    store.load_started_gen.get_untracked(),
                    store.patch_gen.get_untracked(),
                    store.loaded.get_untracked(),
                ) {
                    // Un parche llegó durante la carga: se descarta y se repite.
                    store.reload_all();
                    return;
                }
                store.active.set(loaded);
                store.loaded.set(true);
            }
        });
        subscribe_task_events(move |event: TaskStreamEvent| store.apply_event(event));
        store
    }

    /// Contador de recargas completas (clave de los `Resource` que dependen de ellas).
    pub fn reload_count(&self) -> u32 {
        self.reload.get()
    }

    /// Fuerza una carga completa de tareas activas y workspaces.
    pub fn reload_all(&self) {
        self.reload.update(|n: &mut u32| *n = n.wrapping_add(1));
    }

    /// Aplica un evento del stream: recarga completa, parche de una tarea o baja local.
    pub fn apply_event(&self, event: TaskStreamEvent) {
        match event {
            TaskStreamEvent::Reload => self.reload_all(),
            TaskStreamEvent::Removed(id) => self.remove_local(&id),
            TaskStreamEvent::Changed(id) => self.refresh_one(id),
        }
    }

    fn bump_patch_gen(&self) {
        self.patch_gen.update(|n: &mut u32| *n = n.wrapping_add(1));
    }

    fn remove_local(&self, id: &str) {
        self.bump_patch_gen();
        let active: RwSignal<Vec<TaskDto>> = self.active;
        let done: RwSignal<Vec<TaskDto>> = self.done;
        let mut next_active: Vec<TaskDto> = active.get_untracked();
        let mut next_done: Vec<TaskDto> = done.get_untracked();
        apply_task_removal(&mut next_active, &mut next_done, id);
        active.set(next_active);
        done.set(next_done);
    }

    #[cfg(feature = "hydrate")]
    fn patch_local(&self, task: TaskDto) {
        use crate::modules::tasks::presentation::state::task_merge::apply_task_change;

        self.bump_patch_gen();
        let mut next_active: Vec<TaskDto> = self.active.get_untracked();
        let mut next_done: Vec<TaskDto> = self.done.get_untracked();
        apply_task_change(
            &mut next_active,
            &mut next_done,
            task,
            self.show_done.get_untracked(),
        );
        self.active.set(next_active);
        self.done.set(next_done);
    }

    /// Pide solo la tarea `id` y la aplica; si falla (p.ej. ya no existe) recarga todo.
    /// Solo se invoca desde los eventos del navegador, por eso solo existe con `hydrate`.
    #[cfg(feature = "hydrate")]
    fn refresh_one(&self, id: String) {
        use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::get_task;

        let store: Self = *self;
        leptos::task::spawn_local(async move {
            match get_task(id.clone()).await {
                Ok(task) => store.patch_local(task),
                Err(error) => {
                    leptos::logging::error!("no se pudo refrescar la tarea {id}: {error}");
                    store.reload_all();
                }
            }
        });
    }

    #[cfg(not(feature = "hydrate"))]
    fn refresh_one(&self, _id: String) {
        self.reload_all();
    }
}

impl Default for TaskStore {
    fn default() -> Self {
        Self::new()
    }
}

/// Obtiene el almacén provisto por el shell.
pub fn use_task_store() -> TaskStore {
    expect_context::<TaskStore>()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reload_is_stale_only_when_patched_after_first_load() {
        assert!(reload_is_stale(1, 2, true));
        assert!(!reload_is_stale(2, 2, true));
        assert!(!reload_is_stale(0, 5, false));
        assert!(reload_is_stale(u32::MAX, 0, true));
    }

    #[test]
    fn workspaces_key_is_stable_outside_the_workspaces_area() {
        assert_eq!(workspaces_area_key("/"), "");
        assert_eq!(workspaces_area_key("/tasks/abc"), "");
        assert_eq!(workspaces_area_key("/workspaces"), "/workspaces");
        assert_eq!(
            workspaces_area_key("/workspaces/w1/edit"),
            "/workspaces/w1/edit"
        );
    }
}
