//! Componentes del dock: árbol de divisiones, grupos con barra de pestañas y arrastre.

use leptos::prelude::*;
use leptos_router::params::ParamsMap;

use crate::core::components::shell::DropdownMenu;
use crate::core::dock::context::{DockContext, GroupContext, TabContext, use_dock};
use crate::core::dock::js;
use crate::core::dock::layout::{
    DockLayout, DropZone, GroupId, LayoutNode, SplitDir, TabEntry, TabId,
};
use crate::core::i18n::Msg;

const CLOSE_OTHERS: Msg = Msg {
    en: "Close others",
    es: "Cerrar las demás",
    zh: "关闭其他",
    de: "Andere schließen",
    fr: "Fermer les autres",
};
const CLOSE_SPLIT: Msg = Msg {
    en: "Close split",
    es: "Cerrar división",
    zh: "关闭分屏",
    de: "Teilung schließen",
    fr: "Fermer la division",
};
const CLOSE_TAB: Msg = Msg {
    en: "Close tab",
    es: "Cerrar pestaña",
    zh: "关闭标签页",
    de: "Tab schließen",
    fr: "Fermer l'onglet",
};
const MOVE_TAB_DOWN: Msg = Msg {
    en: "Move tab down",
    es: "Mover pestaña abajo",
    zh: "将标签页移到下方",
    de: "Tab nach unten verschieben",
    fr: "Déplacer l'onglet vers le bas",
};
const MOVE_TAB_RIGHT: Msg = Msg {
    en: "Move tab to the right",
    es: "Mover pestaña a la derecha",
    zh: "将标签页移到右侧",
    de: "Tab nach rechts verschieben",
    fr: "Déplacer l'onglet vers la droite",
};
const NEW_TAB: Msg = Msg {
    en: "New tab",
    es: "Nueva pestaña",
    zh: "新建标签页",
    de: "Neuer Tab",
    fr: "Nouvel onglet",
};

/// Funciones que la aplicación aporta al dock: contenido, parámetros e icono de cada ruta.
#[derive(Clone, Copy)]
pub struct DockHooks {
    pub render: Callback<String, AnyView>,
    pub params_for: fn(&str) -> ParamsMap,
    pub icon_for: fn(&str) -> &'static str,
}

/// Raíz del dock: reconstruye el árbol solo cuando cambia su forma (no con pestañas ni
/// al arrastrar un divisor), de modo que las pestañas montadas conservan su estado.
#[component]
pub fn DockHost(
    hooks: DockHooks,
    #[prop(into)] launcher: ViewFn,
    #[prop(into)] empty: ViewFn,
) -> impl IntoView {
    let dock: DockContext = use_dock();
    let shape: Memo<LayoutNode> =
        Memo::new(move |_| dock.layout.with(|l: &DockLayout| l.root.shape()));
    view! {
        <div class="flex h-full min-h-0 w-full min-w-0">
            {move || {
                view! {
                    <DockNode
                        node=shape.get()
                        hooks=hooks
                        launcher=launcher.clone()
                        empty=empty.clone()
                    />
                }
            }}
        </div>
    }
}

#[component]
fn DockNode(node: LayoutNode, hooks: DockHooks, launcher: ViewFn, empty: ViewFn) -> AnyView {
    match node {
        LayoutNode::Group(group_id) => view! {
            <DockGroup group_id=group_id hooks=hooks launcher=launcher empty=empty />
        }
        .into_any(),
        LayoutNode::Split {
            id,
            dir,
            first,
            second,
            ..
        } => {
            let dock: DockContext = use_dock();
            let vertical: bool = dir == SplitDir::Column;
            let ratio: Signal<f32> = Signal::derive(move || {
                dock.layout
                    .with(|l: &DockLayout| l.root.ratio_of(id))
                    .unwrap_or(0.5)
            });
            let container_class: &str = if vertical {
                "flex min-h-0 min-w-0 flex-1 flex-col"
            } else {
                "flex min-h-0 min-w-0 flex-1 flex-row"
            };
            let divider_class: &str = if vertical {
                "dock-divider dock-divider-col"
            } else {
                "dock-divider dock-divider-row"
            };
            let on_down = move |ev: leptos::ev::PointerEvent| {
                js::start_resize(ev, vertical, move |value: f64| {
                    dock.set_ratio(id, value as f32);
                });
            };
            view! {
                <div class=container_class>
                    <div
                        class="flex min-h-0 min-w-0 overflow-hidden"
                        style=move || format!("flex: 0 0 {}%", ratio.get() * 100.0)
                    >
                        <DockNode
                            node=*first
                            hooks=hooks
                            launcher=launcher.clone()
                            empty=empty.clone()
                        />
                    </div>
                    <div
                        class=divider_class
                        data-dock-divider=""
                        role="separator"
                        aria-orientation=if vertical { "horizontal" } else { "vertical" }
                        on:pointerdown=on_down
                    ></div>
                    <div class="flex min-h-0 min-w-0 flex-1 overflow-hidden">
                        <DockNode node=*second hooks=hooks launcher=launcher empty=empty />
                    </div>
                </div>
            }
            .into_any()
        }
    }
}

/// Zonas de suelta sobre el cuerpo de un grupo: clase de posición por zona.
const DROP_ZONES: [(DropZone, &str); 5] = [
    (DropZone::Left, "left-0 top-0 bottom-0 w-1/4"),
    (DropZone::Right, "right-0 top-0 bottom-0 w-1/4"),
    (DropZone::Top, "left-1/4 right-1/4 top-0 h-1/4"),
    (DropZone::Bottom, "left-1/4 right-1/4 bottom-0 h-1/4"),
    (DropZone::Center, "left-1/4 right-1/4 top-1/4 bottom-1/4"),
];

/// Superposición con las cinco zonas de suelta mientras se arrastra una pestaña.
#[component]
fn DropOverlay(group_id: GroupId) -> impl IntoView {
    let dock: DockContext = use_dock();
    let hover: RwSignal<Option<DropZone>> = RwSignal::new(None);
    view! {
        <div class="absolute inset-0 z-30">
            {DROP_ZONES
                .into_iter()
                .map(|(zone, position)| {
                    view! {
                        <div
                            class=move || {
                                let state: &str = if hover.get() == Some(zone) {
                                    "dock-drop-zone dock-drop-zone-hot"
                                } else {
                                    "dock-drop-zone"
                                };
                                format!("absolute {position} {state}")
                            }
                            on:dragover=move |ev: leptos::ev::DragEvent| {
                                ev.prevent_default();
                                hover.set(Some(zone));
                            }
                            on:dragleave=move |_| hover.set(None)
                            on:drop=move |ev: leptos::ev::DragEvent| {
                                ev.prevent_default();
                                if let Some(tab) = dock.dragging.get_untracked() {
                                    dock.move_tab(tab, group_id, zone);
                                }
                                dock.dragging.set(None);
                            }
                        ></div>
                    }
                })
                .collect_view()}
        </div>
    }
}

/// Pestaña de la barra: clic activa, × (o clic central) cierra, arrastrable para reordenar y entre grupos.
#[component]
fn TabButton(
    tab_id: TabId,
    tabs: Memo<Vec<TabEntry>>,
    active: Memo<Option<TabId>>,
    hooks: DockHooks,
) -> impl IntoView {
    let dock: DockContext = use_dock();
    let title: Memo<String> = Memo::new(move |_| {
        tabs.with(|list: &Vec<TabEntry>| {
            list.iter()
                .find(|t: &&TabEntry| t.id == tab_id)
                .map(|t: &TabEntry| t.title.clone())
                .unwrap_or_default()
        })
    });
    let icon: Memo<&'static str> = Memo::new(move |_| {
        tabs.with(|list: &Vec<TabEntry>| {
            list.iter()
                .find(|t: &&TabEntry| t.id == tab_id)
                .map_or("", |t: &TabEntry| (hooks.icon_for)(&t.path))
        })
    });
    let node: NodeRef<leptos::html::Div> = NodeRef::new();
    Effect::new(move |_| {
        if active.get() == Some(tab_id)
            && let Some(element) = node.get()
        {
            js::reveal_tab(element);
        }
    });
    view! {
        <div
            node_ref=node
            role="tab"
            draggable="true"
            class=move || {
                if active.get() == Some(tab_id) { "group dock-tab dock-tab-active" } else { "group dock-tab" }
            }
            aria-selected=move || (active.get() == Some(tab_id)).to_string()
            title=move || title.get()
            on:click=move |_| dock.activate(tab_id)
            on:auxclick=move |ev: leptos::ev::MouseEvent| {
                if ev.button() == 1 {
                    dock.close_tab(tab_id);
                }
            }
            on:dragstart=move |ev: leptos::ev::DragEvent| {
                js::start_drag(ev, &tab_id.to_string());
                dock.dragging.set(Some(tab_id));
            }
            on:dragend=move |_| dock.dragging.set(None)
            on:dragover=move |ev: leptos::ev::DragEvent| ev.prevent_default()
            on:drop=move |ev: leptos::ev::DragEvent| {
                ev.prevent_default();
                ev.stop_propagation();
                if let Some(dragged) = dock.dragging.get_untracked() {
                    dock.reorder_tab(dragged, tab_id, js::drop_after(ev));
                }
                dock.dragging.set(None);
            }
        >
            <span class="dock-tab-icon" aria-hidden="true">
                {move || icon.get()}
            </span>
            <span class="min-w-0 flex-1 truncate">{move || title.get()}</span>
            <button
                class="dock-tab-close"
                aria-label=CLOSE_TAB.get()
                on:click=move |ev: leptos::ev::MouseEvent| {
                    ev.stop_propagation();
                    dock.close_tab(tab_id);
                }
            >
                "×"
            </button>
        </div>
    }
}

/// Contenido de una pestaña: se monta la primera vez que se muestra y se conserva oculto.
#[component]
fn TabPane(
    tab_id: TabId,
    group_id: GroupId,
    path: String,
    active: Memo<Option<TabId>>,
    hooks: DockHooks,
) -> impl IntoView {
    let seen: RwSignal<bool> = RwSignal::new(false);
    let is_active: Memo<bool> = Memo::new(move |_| active.get() == Some(tab_id));
    Effect::new(move |_| {
        if is_active.get() {
            seen.set(true);
        }
    });
    provide_context(TabContext {
        tab_id,
        group_id,
        params: (hooks.params_for)(&path),
        path: path.clone(),
    });
    view! {
        <div class=move || if is_active.get() { "absolute inset-0 overflow-hidden" } else { "hidden" }>
            {move || seen.get().then(|| hooks.render.run(path.clone()))}
        </div>
    }
}

/// Grupo de pestañas: barra (pestañas, `+` y `⋯`) y cuerpo con las pestañas montadas.
#[component]
fn DockGroup(
    group_id: GroupId,
    hooks: DockHooks,
    launcher: ViewFn,
    empty: ViewFn,
) -> impl IntoView {
    let dock: DockContext = use_dock();
    provide_context(GroupContext(group_id));
    let tabs: Memo<Vec<TabEntry>> = Memo::new(move |_| {
        dock.layout.with(|l: &DockLayout| {
            l.group(group_id)
                .map(|g| g.tabs.clone())
                .unwrap_or_default()
        })
    });
    let active: Memo<Option<TabId>> = Memo::new(move |_| {
        dock.layout
            .with(|l: &DockLayout| l.group(group_id).and_then(|g| g.active))
    });
    let focused: Memo<bool> =
        Memo::new(move |_| dock.layout.with(|l: &DockLayout| l.focused == group_id));
    let is_empty: Memo<bool> = Memo::new(move |_| tabs.with(Vec::is_empty));
    let is_only_group: Memo<bool> =
        Memo::new(move |_| dock.layout.with(|l: &DockLayout| l.groups.len() <= 1));
    let dragging: Memo<bool> = Memo::new(move |_| dock.dragging.with(Option::is_some));
    let with_active = move |f: fn(&DockContext, TabId)| {
        if let Some(tab) = active.get_untracked() {
            f(&dock, tab);
        }
    };

    view! {
        <section
            class=move || {
                if focused.get() { "dock-group dock-group-focused" } else { "dock-group" }
            }
            on:pointerdown=move |_| dock.focus(group_id)
        >
            <div
                class="dock-tabbar"
                on:dragover=move |ev: leptos::ev::DragEvent| ev.prevent_default()
                on:drop=move |ev: leptos::ev::DragEvent| {
                    ev.prevent_default();
                    if let Some(tab) = dock.dragging.get_untracked() {
                        dock.move_tab(tab, group_id, DropZone::Center);
                    }
                    dock.dragging.set(None);
                }
            >
                <div class="dock-tabs" role="tablist">
                    <For
                        each=move || tabs.get()
                        key=|tab: &TabEntry| tab.id
                        children=move |tab: TabEntry| {
                            view! { <TabButton tab_id=tab.id tabs=tabs active=active hooks=hooks /> }
                        }
                    />
                </div>
                <DropdownMenu
                    label="+"
                    aria_label=NEW_TAB.get()
                    trigger_class="dock-icon-btn"
                    menu_class="min-w-52"
                >
                    {launcher.run()}
                </DropdownMenu>
                <DropdownMenu
                    label="⋯"
                    aria_label="Opciones del grupo"
                    trigger_class="dock-icon-btn"
                    menu_class="min-w-52"
                >
                    <button
                        class="menu-item"
                        on:click=move |_| dock.split_group(group_id, SplitDir::Row)
                    >
                        "Dividir a la derecha"
                    </button>
                    <button
                        class="menu-item"
                        on:click=move |_| dock.split_group(group_id, SplitDir::Column)
                    >
                        "Dividir hacia abajo"
                    </button>
                    <button
                        class="menu-item"
                        disabled=move || is_empty.get()
                        on:click=move |_| {
                            with_active(|d: &DockContext, tab: TabId| {
                                d.move_to_new_split(tab, SplitDir::Row)
                            })
                        }
                    >
                        {MOVE_TAB_RIGHT.get()}
                    </button>
                    <button
                        class="menu-item"
                        disabled=move || is_empty.get()
                        on:click=move |_| {
                            with_active(|d: &DockContext, tab: TabId| {
                                d.move_to_new_split(tab, SplitDir::Column)
                            })
                        }
                    >
                        {MOVE_TAB_DOWN.get()}
                    </button>
                    <button
                        class="menu-item"
                        disabled=move || tabs.with(|t: &Vec<TabEntry>| t.len() < 2)
                        on:click=move |_| {
                            with_active(|d: &DockContext, tab: TabId| d.close_others(tab))
                        }
                    >
                        {CLOSE_OTHERS.get()}
                    </button>
                    <button
                        class="menu-item"
                        disabled=move || is_only_group.get()
                        on:click=move |_| dock.close_group(group_id)
                    >
                        {CLOSE_SPLIT.get()}
                    </button>
                </DropdownMenu>
            </div>
            <div class="relative min-h-0 flex-1 bg-bg">
                <For
                    each=move || tabs.get()
                    key=|tab: &TabEntry| (tab.id, tab.path.clone())
                    children=move |tab: TabEntry| {
                        view! {
                            <TabPane
                                tab_id=tab.id
                                group_id=group_id
                                path=tab.path
                                active=active
                                hooks=hooks
                            />
                        }
                    }
                />
                <Show when=move || is_empty.get()>
                    {empty.run()}
                    <Show when=move || !is_only_group.get()>
                        <button
                            class="btn-secondary absolute right-3 bottom-3 min-h-8 px-3 py-1 text-xs"
                            on:click=move |_| dock.close_group(group_id)
                        >
                            {CLOSE_SPLIT.get()}
                        </button>
                    </Show>
                </Show>
                <Show when=move || dragging.get()>
                    <DropOverlay group_id=group_id />
                </Show>
            </div>
        </section>
    }
}
