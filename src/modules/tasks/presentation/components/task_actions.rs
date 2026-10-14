use leptos::prelude::*;

use crate::core::browser::confirm;
use crate::core::components::layout::Badge;
use crate::core::dock::link::TabLink;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskActionsDto;

/// Callbacks de las acciones sobre una tarea.
#[derive(Clone, Copy)]
pub struct TaskActionHandlers {
    pub abort: Callback<()>,
    pub finish: Callback<()>,
    pub resume: Callback<()>,
    pub merge: Callback<()>,
    pub discard: Callback<()>,
    pub reset: Callback<()>,
    pub open_diff: Callback<()>,
    pub show_subagents: Callback<()>,
}

/// Variante visual de una fila de acción.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ActionTone {
    Normal,
    Primary,
    Danger,
}

impl ActionTone {
    fn class(self) -> &'static str {
        match self {
            ActionTone::Normal => "action-row",
            ActionTone::Primary => "action-row action-row-primary",
            ActionTone::Danger => "action-row action-row-danger",
        }
    }
}

/// Grupos de acciones que hay que pintar según lo que permite la tarea (pure).
pub fn visible_sections(actions: &TaskActionsDto) -> [bool; 4] {
    [
        actions.can_abort || actions.can_resume || actions.can_finish,
        actions.can_view_diff || actions.can_merge,
        actions.can_open_terminal,
        actions.can_discard || actions.can_reset,
    ]
}

/// Fila de acción: icono, texto y, opcionalmente, un indicador a la derecha.
#[component]
fn ActionRow(
    icon: &'static str,
    #[prop(default = ActionTone::Normal)] tone: ActionTone,
    #[prop(into, default = Signal::stored(false))] disabled: Signal<bool>,
    on_click: Callback<leptos::ev::MouseEvent>,
    children: Children,
) -> impl IntoView {
    view! {
        <button
            class=tone.class()
            disabled=move || disabled.get()
            on:click=move |ev: leptos::ev::MouseEvent| on_click.run(ev)
        >
            <span class="action-icon" aria-hidden="true">
                {icon}
            </span>
            {children()}
        </button>
    }
}

/// Menú de acciones contextual: lista vertical agrupada según lo que permite el estado.
#[component]
pub fn TaskActions(
    actions: TaskActionsDto,
    handlers: TaskActionHandlers,
    #[prop(into)] busy: Signal<bool>,
    #[prop(into)] subagents_active: Signal<u32>,
    /// Ruta que abre una terminal en el worktree de la tarea.
    #[prop(into)]
    terminal_href: String,
) -> impl IntoView {
    let [_agent, review, worktree, danger] = visible_sections(&actions);
    let plain =
        |action: Callback<()>| Callback::new(move |_: leptos::ev::MouseEvent| action.run(()));
    let ask = |message: &'static str, action: Callback<()>| {
        Callback::new(move |_: leptos::ev::MouseEvent| {
            if confirm(message) {
                action.run(());
            }
        })
    };
    view! {
        <div class="action-list -mx-3">
            <div class="action-section">"Agente"</div>
            {actions
                .can_finish
                .then(|| {
                    view! {
                        <ActionRow
                            icon="✓"
                            tone=ActionTone::Primary
                            disabled=busy
                            on_click=ask(
                                "¿Finalizar la tarea? Se cerrará el agente y se consolidarán los cambios para revisión.",
                                handlers.finish,
                            )
                        >
                            "Finalizar y revisar"
                        </ActionRow>
                    }
                })}
            {actions
                .can_resume
                .then(|| {
                    view! {
                        <ActionRow icon="▶" disabled=busy on_click=plain(handlers.resume)>
                            "Reanudar agente"
                        </ActionRow>
                    }
                })}
            {actions
                .can_abort
                .then(|| {
                    view! {
                        <ActionRow icon="■" disabled=busy on_click=plain(handlers.abort)>
                            "Abortar ejecución"
                        </ActionRow>
                    }
                })}
            <ActionRow icon="◈" on_click=plain(handlers.show_subagents)>
                "Subagentes"
                {move || {
                    let active: u32 = subagents_active.get();
                    (active > 0)
                        .then(|| view! { <Badge text=active.to_string() variant="info" /> })
                }}
            </ActionRow>
            {review
                .then(|| {
                    view! {
                        <div class="action-section">"Revisión"</div>
                        {actions
                            .can_view_diff
                            .then(|| {
                                view! {
                                    <ActionRow icon="±" on_click=plain(handlers.open_diff)>
                                        "Ver diff"
                                    </ActionRow>
                                }
                            })}
                        {actions
                            .can_merge
                            .then(|| {
                                view! {
                                    <ActionRow
                                        icon="⇄"
                                        tone=ActionTone::Primary
                                        disabled=busy
                                        on_click=ask(
                                            "¿Integrar la rama en la base? Se eliminará el worktree.",
                                            handlers.merge,
                                        )
                                    >
                                        "Merge"
                                    </ActionRow>
                                }
                            })}
                    }
                })}
            {worktree
                .then(|| {
                    view! {
                        <div class="action-section">"Worktree"</div>
                        <TabLink href=terminal_href class="action-row">
                            <span class="action-icon" aria-hidden="true">
                                "❯"
                            </span>
                            "Abrir terminal"
                        </TabLink>
                    }
                })}
            {danger
                .then(|| {
                    view! {
                        <div class="action-section">"Zona de peligro"</div>
                        {actions
                            .can_reset
                            .then(|| {
                                view! {
                                    <ActionRow
                                        icon="↺"
                                        tone=ActionTone::Danger
                                        disabled=busy
                                        on_click=ask(
                                            "¿Restablecer la tarea? Se detendrá el agente, se borrarán el worktree, la rama y el historial, y volverá a Backlog.",
                                            handlers.reset,
                                        )
                                    >
                                        "Restablecer"
                                    </ActionRow>
                                }
                            })}
                        {actions
                            .can_discard
                            .then(|| {
                                view! {
                                    <ActionRow
                                        icon="✕"
                                        tone=ActionTone::Danger
                                        disabled=busy
                                        on_click=ask(
                                            "¿Descartar la tarea? Se borrarán el worktree y la rama sin integrar.",
                                            handlers.discard,
                                        )
                                    >
                                        "Descartar"
                                    </ActionRow>
                                }
                            })}
                    }
                })}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sections_follow_the_allowed_actions() {
        let none: TaskActionsDto = TaskActionsDto::default();
        assert_eq!(visible_sections(&none), [false, false, false, false]);
        let review: TaskActionsDto = TaskActionsDto {
            can_view_diff: true,
            can_discard: true,
            ..Default::default()
        };
        assert_eq!(visible_sections(&review), [false, true, false, true]);
        let running: TaskActionsDto = TaskActionsDto {
            can_abort: true,
            can_open_terminal: true,
            ..Default::default()
        };
        assert_eq!(visible_sections(&running), [true, false, true, false]);
    }

    #[test]
    fn tone_maps_to_distinct_classes() {
        assert_eq!(ActionTone::Normal.class(), "action-row");
        assert!(ActionTone::Danger.class().contains("action-row-danger"));
        assert!(ActionTone::Primary.class().contains("action-row-primary"));
    }
}
