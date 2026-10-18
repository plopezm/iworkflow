use crate::core::browser;
use crate::core::dock::context::use_dock;
use crate::core::dock::link::TabLink;
use crate::core::i18n::Msg;
use leptos::prelude::*;

const CLOSE_PANEL: Msg = Msg {
    en: "Close panel",
    es: "Cerrar panel",
    zh: "关闭面板",
    de: "Bereich schließen",
    fr: "Fermer le panneau",
};
const COLLAPSE_PANEL: Msg = Msg {
    en: "Collapse panel",
    es: "Colapsar panel",
    zh: "折叠面板",
    de: "Bereich einklappen",
    fr: "Réduire le panneau",
};
const EXPAND_PANEL: Msg = Msg {
    en: "Expand panel",
    es: "Expandir panel",
    zh: "展开面板",
    de: "Bereich ausklappen",
    fr: "Développer le panneau",
};
const HIDE_SIDEBAR: Msg = Msg {
    en: "Hide sidebar",
    es: "Ocultar barra lateral",
    zh: "隐藏侧边栏",
    de: "Seitenleiste ausblenden",
    fr: "Masquer la barre latérale",
};
const MORE_ACTIONS: Msg = Msg {
    en: "More actions",
    es: "Más acciones",
    zh: "更多操作",
    de: "Weitere Aktionen",
    fr: "Plus d'actions",
};
const NEEDS_ATTENTION: Msg = Msg {
    en: "Needs attention",
    es: "Necesita atención",
    zh: "需要关注",
    de: "Erfordert Aufmerksamkeit",
    fr: "Nécessite votre attention",
};
const OPEN_MENU: Msg = Msg {
    en: "Open menu",
    es: "Abrir menú",
    zh: "打开菜单",
    de: "Menü öffnen",
    fr: "Ouvrir le menu",
};
const SHOW_SIDEBAR: Msg = Msg {
    en: "Show sidebar",
    es: "Mostrar barra lateral",
    zh: "显示侧边栏",
    de: "Seitenleiste einblenden",
    fr: "Afficher la barre latérale",
};
const SIDEBAR: Msg = Msg {
    en: "Sidebar",
    es: "Barra lateral",
    zh: "侧边栏",
    de: "Seitenleiste",
    fr: "Barre latérale",
};

/// Borde del layout en el que se ancla un panel colapsable.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PanelSide {
    Left,
    Right,
}

/// Clases del panel: overlay fijo en móvil (`overlay_open`) y columna en escritorio (`open`).
fn panel_class(side: PanelSide, open: bool, overlay_open: bool) -> String {
    let anchor: &str = match side {
        PanelSide::Left => "left-0 border-r",
        PanelSide::Right => "right-0 border-l",
    };
    let mobile: &str = if overlay_open { "flex" } else { "hidden" };
    let desktop: &str = if open { "md:flex" } else { "md:hidden" };
    format!(
        "{mobile} {desktop} fixed inset-y-0 z-40 w-[88vw] max-w-sm min-h-0 flex-col border-border bg-surface md:static md:z-auto md:max-w-none md:shrink-0 {anchor}"
    )
}

/// Clases del tirador de colapso (solo escritorio), a caballo del borde del panel.
fn handle_class(side: PanelSide, open: bool) -> &'static str {
    match (side, open) {
        (PanelSide::Left, true) => {
            "absolute top-1/2 right-0 z-20 hidden h-12 w-4 translate-x-1/2 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md border border-border bg-surface text-xs text-muted opacity-60 transition-opacity hover:opacity-100 hover:text-fg md:flex"
        }
        (PanelSide::Left, false) => {
            "absolute top-1/2 left-0 z-20 hidden h-12 w-4 -translate-y-1/2 cursor-pointer items-center justify-center rounded-r-md border border-border bg-surface text-xs text-muted hover:text-fg md:flex"
        }
        (PanelSide::Right, true) => {
            "absolute top-1/2 left-0 z-20 hidden h-12 w-4 -translate-x-1/2 -translate-y-1/2 cursor-pointer items-center justify-center rounded-md border border-border bg-surface text-xs text-muted hover:text-fg md:flex"
        }
        (PanelSide::Right, false) => {
            "absolute top-1/2 right-0 z-20 hidden h-12 w-4 -translate-y-1/2 cursor-pointer items-center justify-center rounded-l-md border border-border bg-surface text-xs text-muted hover:text-fg md:flex"
        }
    }
}

fn handle_glyph(side: PanelSide, open: bool) -> &'static str {
    match (side, open) {
        (PanelSide::Left, true) | (PanelSide::Right, false) => "‹",
        (PanelSide::Left, false) | (PanelSide::Right, true) => "›",
    }
}

/// Panel lateral colapsable: columna con tirador en escritorio, overlay con fondo en móvil.
/// `width` es la clase de ancho de escritorio (p. ej. `md:w-72`) y puede cambiar en caliente.
#[component]
pub fn CollapsePanel(
    side: PanelSide,
    open: RwSignal<bool>,
    overlay_open: RwSignal<bool>,
    #[prop(into)] width: Signal<String>,
    #[prop(into)] label: String,
    children: Children,
) -> impl IntoView {
    let aside_class: Memo<String> = Memo::new(move |_| {
        format!(
            "{} {}",
            panel_class(side, open.get(), overlay_open.get()),
            width.get()
        )
    });
    view! {
        <div class="relative flex min-h-0 shrink-0">
            <Show when=move || overlay_open.get()>
                <button
                    class="fixed inset-0 z-30 cursor-default bg-black/40 md:hidden"
                    aria-label=CLOSE_PANEL.get()
                    on:click=move |_| overlay_open.set(false)
                ></button>
            </Show>
            <aside class=move || aside_class.get() aria-label=label>
                {children()}
            </aside>
            <button
                class=move || handle_class(side, open.get())
                aria-label=move || if open.get() { COLLAPSE_PANEL.get() } else { EXPAND_PANEL.get() }
                on:click=move |_| open.update(|v: &mut bool| *v = !*v)
            >
                {move || handle_glyph(side, open.get())}
            </button>
        </div>
    }
}

/// Menú desplegable genérico: se cierra al elegir una opción o al pulsar fuera.
#[component]
pub fn DropdownMenu(
    #[prop(into)] label: String,
    #[prop(into)] aria_label: String,
    #[prop(into)] trigger_class: String,
    #[prop(optional, into)] menu_class: String,
    children: ChildrenFn,
) -> impl IntoView {
    let open: RwSignal<bool> = RwSignal::new(false);
    view! {
        <div class="relative">
            <button
                class=trigger_class
                aria-label=aria_label
                aria-haspopup="menu"
                aria-expanded=move || open.get().to_string()
                on:click=move |_| open.update(|v: &mut bool| *v = !*v)
            >
                {label}
            </button>
            <Show when=move || open.get()>
                <button
                    class="fixed inset-0 z-30 cursor-default"
                    tabindex="-1"
                    aria-hidden="true"
                    on:click=move |_| open.set(false)
                ></button>
                <div
                    class=format!(
                        "absolute top-full right-0 z-40 mt-1 flex min-w-44 flex-col rounded-md border border-border bg-surface p-1 shadow-lg {menu_class}",
                    )
                    role="menu"
                    on:click=move |_| open.set(false)
                >
                    {children()}
                </div>
            </Show>
        </div>
    }
}

/// Sección colapsable de la barra lateral con contador y acciones opcionales.
#[component]
pub fn SidebarSection(
    #[prop(into)] title: String,
    #[prop(into)] count: Signal<usize>,
    #[prop(optional, into)] actions: Option<ViewFn>,
    children: Children,
) -> impl IntoView {
    let open: RwSignal<bool> = RwSignal::new(true);
    view! {
        <section class="px-1 pb-1">
            <div class="flex items-center gap-2 px-2.5 py-1.5">
                <button
                    class="flex flex-1 cursor-pointer items-center gap-1.5 text-left text-[11px] font-medium text-muted hover:text-fg"
                    aria-expanded=move || open.get().to_string()
                    on:click=move |_| open.update(|v: &mut bool| *v = !*v)
                >
                    <span aria-hidden="true">{move || if open.get() { "▾" } else { "▸" }}</span>
                    {title}
                </button>
                {actions.map(|actions: ViewFn| actions.run())}
                <span class="text-[10px] text-muted">{move || count.get()}</span>
            </div>
            <div class=move || {
                if open.get() { "flex flex-col gap-0.5 px-1.5 pb-2" } else { "hidden" }
            }>{children()}</div>
        </section>
    }
}

/// Fila de la barra lateral: enlace con título y subtítulo, y menú `…` opcional.
#[component]
pub fn SidebarItem(
    #[prop(into)] href: String,
    #[prop(into)] title: String,
    #[prop(optional, into)] subtitle: String,
    #[prop(into)] selected: Signal<bool>,
    #[prop(optional)] highlight: bool,
    #[prop(optional, into)] menu: Option<ViewFn>,
) -> impl IntoView {
    let row_class = move || {
        if selected.get() {
            "flex items-stretch rounded-md bg-surface-3"
        } else {
            "flex items-stretch rounded-md hover:bg-surface-2"
        }
    };
    view! {
        <div class=row_class>
            <TabLink href=href title=title.clone() class="min-w-0 flex-1 px-2.5 py-1.5">
                <div class="flex items-center gap-1.5">
                    {highlight
                        .then(|| {
                            view! {
                                <span
                                    class="h-1.5 w-1.5 shrink-0 rounded-full bg-error"
                                    title=NEEDS_ATTENTION.get()
                                ></span>
                            }
                        })}
                    <span class="truncate text-[13px] font-medium text-fg">{title}</span>
                </div>
                {(!subtitle.is_empty())
                    .then(|| view! { <div class="truncate text-[11px] text-muted">{subtitle}</div> })}
            </TabLink>
            {menu
                .map(|menu: ViewFn| {
                    view! {
                        <div class="flex">
                            <DropdownMenu
                                label="…"
                                aria_label=MORE_ACTIONS.get()
                                trigger_class="flex h-full cursor-pointer items-center px-2 text-muted hover:text-fg"
                            >
                                {menu.run()}
                            </DropdownMenu>
                        </div>
                    }
                })}
        </div>
    }
}

/// Elemento de un `TabBar`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TabItem {
    pub id: &'static str,
    pub label: &'static str,
}

/// Barra de pestañas controlada: el llamador decide qué cuerpo pintar según `active`.
#[component]
pub fn TabBar(
    tabs: Vec<TabItem>,
    #[prop(into)] active: Signal<String>,
    on_select: Callback<String>,
) -> impl IntoView {
    view! {
        <div class="flex shrink-0 gap-1.5 overflow-x-auto border-b border-border p-2" role="tablist">
            {tabs
                .into_iter()
                .map(|tab: TabItem| {
                    let id: &'static str = tab.id;
                    view! {
                        <button
                            role="tab"
                            class=move || if active.get() == id { "tab tab-selected" } else { "tab" }
                            aria-selected=move || (active.get() == id).to_string()
                            on:click=move |_| on_select.run(id.to_string())
                        >
                            {tab.label}
                        </button>
                    }
                })
                .collect_view()}
        </div>
    }
}

/// Clave de `localStorage` del estado de la barra lateral en escritorio.
const SIDEBAR_STORAGE_KEY: &str = "iwf-sidebar-open";

/// Interpreta el valor guardado: solo `"0"` significa oculta; todo lo demás, abierta (pure).
pub fn parse_sidebar_open(stored: Option<&str>) -> bool {
    stored != Some("0")
}

/// Estado de la barra lateral, provisto por contexto para que su contenido pueda ocultarla.
#[derive(Clone, Copy)]
pub struct SidebarState {
    pub open: RwSignal<bool>,
}

impl SidebarState {
    pub fn toggle(&self) {
        self.open.update(|v: &mut bool| *v = !*v);
    }
}

/// Botón que oculta/muestra la barra lateral en escritorio (no hace nada fuera del `AppShell`).
#[component]
pub fn SidebarToggle(#[prop(optional, into)] class: String) -> impl IntoView {
    let state: Option<SidebarState> = use_context::<SidebarState>();
    let open: Signal<bool> =
        Signal::derive(move || state.is_some_and(|s: SidebarState| s.open.get()));
    view! {
        <button
            class=format!("icon-btn {class}")
            aria-label=move || if open.get() { HIDE_SIDEBAR.get() } else { SHOW_SIDEBAR.get() }
            title=move || if open.get() { HIDE_SIDEBAR.get() } else { SHOW_SIDEBAR.get() }
            on:click=move |_| {
                if let Some(s) = state {
                    s.toggle();
                }
            }
        >
            <svg viewBox="0 0 16 16" fill="none" class="h-4 w-4" aria-hidden="true">
                <rect x="2" y="3" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.3" />
                <path d="M6 3v10" stroke="currentColor" stroke-width="1.3" />
            </svg>
        </button>
    }
}

/// Estructura de tres columnas a pantalla completa: barra lateral colapsable (`sidebar`),
/// y a su derecha la columna central con el contenido enrutado (`children`). Cada columna
/// hace scroll de forma independiente; en móvil la barra es un overlay abierto desde la
/// cabecera.
#[component]
pub fn AppShell(
    #[prop(into)] brand: String,
    #[prop(into)] sidebar: ViewFn,
    children: Children,
) -> impl IntoView {
    let sidebar_open: RwSignal<bool> = RwSignal::new(true);
    let mobile_open: RwSignal<bool> = RwSignal::new(false);
    let layout = use_dock().layout;
    provide_context(SidebarState { open: sidebar_open });
    // Solo cliente, tras hidratar: restaura la preferencia y la guarda en cada cambio.
    let restored: RwSignal<bool> = RwSignal::new(false);
    Effect::new(move |_| {
        let stored: Option<String> = browser::read_local_storage(SIDEBAR_STORAGE_KEY);
        sidebar_open.set(parse_sidebar_open(stored.as_deref()));
        restored.set(true);
    });
    Effect::new(move |_| {
        let open: bool = sidebar_open.get();
        if restored.get() {
            browser::write_local_storage(SIDEBAR_STORAGE_KEY, if open { "1" } else { "0" });
        }
    });

    // Cierra el overlay móvil al abrir o mover pestañas.
    Effect::new(move |_| {
        layout.track();
        mobile_open.set(false);
    });

    view! {
        <div class="flex h-dvh w-full overflow-hidden bg-bg text-fg">
            <CollapsePanel
                side=PanelSide::Left
                open=sidebar_open
                overlay_open=mobile_open
                width="md:w-72"
                label=SIDEBAR.get()
            >
                {sidebar.run()}
            </CollapsePanel>
            <Show when=move || !sidebar_open.get()>
                <div class="hidden w-10 shrink-0 flex-col items-center border-r border-border bg-surface pt-2 md:flex">
                    <SidebarToggle />
                </div>
            </Show>
            <div class="flex min-w-0 flex-1 flex-col">
                <header class="flex shrink-0 items-center gap-2 border-b border-border bg-surface px-3 py-2 md:hidden">
                    <button
                        class="btn-secondary min-h-9 px-2.5 py-1"
                        aria-label=OPEN_MENU.get()
                        on:click=move |_| mobile_open.set(true)
                    >
                        "☰"
                    </button>
                    <TabLink href="/" class="text-sm font-semibold tracking-tight">
                        {brand}
                    </TabLink>
                </header>
                <main class="min-h-0 min-w-0 flex-1 overflow-hidden bg-bg">{children()}</main>
            </div>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sidebar_is_open_unless_explicitly_stored_closed() {
        assert!(parse_sidebar_open(None));
        assert!(parse_sidebar_open(Some("1")));
        assert!(parse_sidebar_open(Some("garbage")));
        assert!(!parse_sidebar_open(Some("0")));
    }

    #[test]
    fn panel_class_combines_mobile_overlay_and_desktop_visibility() {
        let closed: String = panel_class(PanelSide::Left, false, false);
        assert!(closed.starts_with("hidden md:hidden"));
        let desktop_open: String = panel_class(PanelSide::Left, true, false);
        assert!(desktop_open.starts_with("hidden md:flex"));
        let overlay: String = panel_class(PanelSide::Right, false, true);
        assert!(overlay.starts_with("flex md:hidden"));
        assert!(overlay.contains("right-0 border-l"));
    }

    #[test]
    fn handle_glyph_points_toward_the_collapse_direction() {
        assert_eq!(handle_glyph(PanelSide::Left, true), "‹");
        assert_eq!(handle_glyph(PanelSide::Left, false), "›");
        assert_eq!(handle_glyph(PanelSide::Right, true), "›");
        assert_eq!(handle_glyph(PanelSide::Right, false), "‹");
    }
}
