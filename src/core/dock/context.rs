//! Estado reactivo del dock y hooks para las pestañas (sin dominio de ningún feature).

use leptos::prelude::*;
use leptos_router::hooks::use_params_map;
use leptos_router::params::ParamsMap;

use crate::core::browser;
use crate::core::dock::layout::{DockLayout, DropZone, GroupId, SplitDir, TabId};

/// Clave de `localStorage` del layout.
const LAYOUT_STORAGE_KEY: &str = "iwf-dock-layout";

/// Estado del espacio de trabajo: layout, arrastre en curso y título por defecto por ruta.
#[derive(Clone, Copy)]
pub struct DockContext {
    pub layout: RwSignal<DockLayout>,
    /// Pestaña que se está arrastrando.
    pub dragging: RwSignal<Option<TabId>>,
    /// `true` cuando ya se intentó restaurar el layout guardado (solo cliente).
    pub restored: RwSignal<bool>,
    default_title: StoredValue<fn(&str) -> String>,
}

impl DockContext {
    /// Crea el estado, lo provee por contexto y sincroniza el layout con `localStorage`.
    /// `default_title` da el título de una pestaña a partir de su ruta.
    pub fn provide(default_title: fn(&str) -> String) -> Self {
        let dock: Self = Self {
            layout: RwSignal::new(DockLayout::new()),
            dragging: RwSignal::new(None),
            restored: RwSignal::new(false),
            default_title: StoredValue::new(default_title),
        };
        // Solo cliente, tras hidratar: el SSR siempre pinta el layout vacío por defecto.
        Effect::new(move |_| {
            if let Some(saved) = browser::read_local_storage(LAYOUT_STORAGE_KEY)
                && let Some(layout) = DockLayout::from_json(&saved)
            {
                dock.layout.set(layout);
            }
            dock.restored.set(true);
        });
        Effect::new(move |_| {
            let json: String = dock.layout.with(DockLayout::to_json);
            if dock.restored.get() {
                browser::write_local_storage(LAYOUT_STORAGE_KEY, &json);
            }
        });
        provide_context(dock);
        dock
    }

    fn title_for(&self, path: &str) -> String {
        (self.default_title.get_value())(path)
    }

    /// Abre `path` (activa la pestaña si ya existe). `group` fija el grupo destino.
    pub fn open(&self, path: &str, title: Option<&str>, group: Option<GroupId>) {
        let title: String = title.map_or_else(|| self.title_for(path), str::to_string);
        self.layout
            .update(|l: &mut DockLayout| {
            let _tab: TabId = l.open(path, &title, group);
        });
    }

    pub fn activate(&self, tab: TabId) {
        self.layout.update(|l: &mut DockLayout| l.activate(tab));
    }

    pub fn focus(&self, group: GroupId) {
        if self.layout.with_untracked(|l: &DockLayout| l.focused) != group {
            self.layout.update(|l: &mut DockLayout| l.focus(group));
        }
    }

    pub fn close_tab(&self, tab: TabId) {
        self.layout.update(|l: &mut DockLayout| l.close_tab(tab));
    }

    pub fn close_others(&self, tab: TabId) {
        self.layout.update(|l: &mut DockLayout| l.close_others(tab));
    }

    pub fn split_group(&self, group: GroupId, dir: SplitDir) {
        self.layout
            .update(|l: &mut DockLayout| {
            let _group: GroupId = l.split_group(group, dir);
        });
    }

    pub fn move_tab(&self, tab: TabId, group: GroupId, zone: DropZone) {
        self.layout
            .update(|l: &mut DockLayout| l.move_tab(tab, group, zone));
    }

    pub fn move_to_new_split(&self, tab: TabId, dir: SplitDir) {
        self.layout
            .update(|l: &mut DockLayout| l.move_to_new_split(tab, dir));
    }

    pub fn set_ratio(&self, split: u64, ratio: f32) {
        self.layout
            .update(|l: &mut DockLayout| l.set_ratio(split, ratio));
    }

    pub fn set_title(&self, path: &str, title: &str) {
        let changed: bool = self.layout.with_untracked(|l: &DockLayout| {
            l.groups
                .iter()
                .flat_map(|g| g.tabs.iter())
                .any(|t| t.path == path && t.title != title)
        });
        if changed {
            self.layout
                .update(|l: &mut DockLayout| l.set_title(path, title));
        }
    }

    pub fn is_path_visible(&self, path: &str) -> bool {
        self.layout
            .with(|l: &DockLayout| l.is_path_visible(path))
    }
}

/// Contexto de una pestaña concreta, provisto a su contenido.
#[derive(Clone, Debug)]
pub struct TabContext {
    pub tab_id: TabId,
    pub group_id: GroupId,
    pub path: String,
    pub params: ParamsMap,
}

/// Contexto de un grupo (lo ven su barra y su menú `+`).
#[derive(Clone, Copy, Debug)]
pub struct GroupContext(pub GroupId);

pub fn use_dock() -> DockContext {
    expect_context::<DockContext>()
}

/// Parámetros de ruta de la pestaña actual (o del router si se usa fuera del dock).
pub fn use_tab_params() -> Memo<ParamsMap> {
    match use_context::<TabContext>() {
        Some(tab) => Memo::new(move |_| tab.params.clone()),
        None => use_params_map(),
    }
}

/// Acciones de navegación entre pestañas, relativas a la pestaña que las usa.
#[derive(Clone, Copy)]
pub struct TabNav {
    dock: DockContext,
    group: Option<GroupId>,
    tab: Option<TabId>,
}

impl TabNav {
    /// Abre `path` en una pestaña del grupo actual.
    pub fn open(&self, path: &str) {
        self.dock.open(path, None, self.group);
    }

    pub fn open_titled(&self, path: &str, title: &str) {
        self.dock.open(path, Some(title), self.group);
    }

    /// Sustituye la pestaña actual por `path` (p. ej. al volver de una pantalla de edición).
    pub fn replace(&self, path: &str) {
        match self.tab {
            Some(tab) => {
                let title: String = self.dock.title_for(path);
                self.dock
                    .layout
                    .update(|l: &mut DockLayout| l.replace_path(tab, path, &title));
            }
            None => self.open(path),
        }
    }

    /// Cierra la pestaña actual.
    pub fn close(&self) {
        if let Some(tab) = self.tab {
            self.dock.close_tab(tab);
        }
    }

    /// Abre `path` y cierra la pestaña actual (p. ej. tras borrar la tarea que mostraba).
    pub fn close_and_open(&self, path: &str) {
        self.open(path);
        self.close();
    }
}

pub fn use_tab_nav() -> TabNav {
    let dock: DockContext = use_dock();
    let tab: Option<TabContext> = use_context::<TabContext>();
    let group: Option<GroupId> = tab
        .as_ref()
        .map(|t: &TabContext| t.group_id)
        .or_else(|| use_context::<GroupContext>().map(|g: GroupContext| g.0));
    TabNav {
        dock,
        group,
        tab: tab.map(|t: TabContext| t.tab_id),
    }
}
