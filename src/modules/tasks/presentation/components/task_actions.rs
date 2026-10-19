use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::browser::confirm;
use crate::core::components::layout::Badge;
use crate::core::dock::link::TabLink;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskActionsDto;

const SECTION_AGENT: Msg = Msg {
    en: "Agent",
    es: "Agente",
    zh: "智能体",
    de: "Agent",
    fr: "Agent",
};

const SECTION_REVIEW: Msg = Msg {
    en: "Review",
    es: "Revisión",
    zh: "审查",
    de: "Überprüfung",
    fr: "Révision",
};

const SECTION_DANGER: Msg = Msg {
    en: "Danger zone",
    es: "Zona de peligro",
    zh: "危险区域",
    de: "Gefahrenzone",
    fr: "Zone dangereuse",
};

const CONFIRM_FINISH: Msg = Msg {
    en: "Finish the task? The agent will be closed and the changes will be consolidated for review.",
    es: "¿Finalizar la tarea? Se cerrará el agente y se consolidarán los cambios para revisión.",
    zh: "要完成该任务吗？智能体将被关闭，更改将合并以供审查。",
    de: "Aufgabe abschließen? Der Agent wird beendet und die Änderungen werden zur Überprüfung zusammengeführt.",
    fr: "Terminer la tâche ? L’agent sera fermé et les modifications seront consolidées pour révision.",
};

const CONFIRM_MERGE: Msg = Msg {
    en: "Merge the branch into the base? The worktree will be removed.",
    es: "¿Integrar la rama en la base? Se eliminará el worktree.",
    zh: "要将分支合并到基础分支吗？worktree 将被删除。",
    de: "Den Branch in die Basis integrieren? Der Worktree wird gelöscht.",
    fr: "Intégrer la branche dans la base ? Le worktree sera supprimé.",
};

const CONFIRM_RESET: Msg = Msg {
    en: "Reset the task? The agent will be stopped, the worktree, the branch and the history will be deleted, and it will go back to Backlog.",
    es: "¿Restablecer la tarea? Se detendrá el agente, se borrarán el worktree, la rama y el historial, y volverá a Backlog.",
    zh: "要重置该任务吗？智能体将被停止，worktree、分支和历史记录将被删除，任务将回到 Backlog。",
    de: "Aufgabe zurücksetzen? Der Agent wird gestoppt, Worktree, Branch und Verlauf werden gelöscht und die Aufgabe kehrt ins Backlog zurück.",
    fr: "Réinitialiser la tâche ? L’agent sera arrêté, le worktree, la branche et l’historique seront supprimés, et la tâche retournera dans le Backlog.",
};

const CONFIRM_DISCARD: Msg = Msg {
    en: "Discard the task? The worktree and the unmerged branch will be deleted.",
    es: "¿Descartar la tarea? Se borrarán el worktree y la rama sin integrar.",
    zh: "要丢弃该任务吗？worktree 和未合并的分支将被删除。",
    de: "Aufgabe verwerfen? Der Worktree und der nicht integrierte Branch werden gelöscht.",
    fr: "Abandonner la tâche ? Le worktree et la branche non intégrée seront supprimés.",
};

const FINISH_AND_REVIEW: Msg = Msg {
    en: "Finish and review",
    es: "Finalizar y revisar",
    zh: "完成并审查",
    de: "Abschließen und überprüfen",
    fr: "Terminer et réviser",
};

const RESUME_AGENT: Msg = Msg {
    en: "Resume agent",
    es: "Reanudar agente",
    zh: "恢复智能体",
    de: "Agent fortsetzen",
    fr: "Reprendre l’agent",
};

const ABORT_RUN: Msg = Msg {
    en: "Abort run",
    es: "Abortar ejecución",
    zh: "中止运行",
    de: "Ausführung abbrechen",
    fr: "Interrompre l’exécution",
};

const SUBAGENTS: Msg = Msg {
    en: "Subagents",
    es: "Subagentes",
    zh: "子智能体",
    de: "Subagents",
    fr: "Sous-agents",
};

const VIEW_DIFF: Msg = Msg {
    en: "View diff",
    es: "Ver diff",
    zh: "查看 diff",
    de: "Diff anzeigen",
    fr: "Voir le diff",
};

const OPEN_TERMINAL: Msg = Msg {
    en: "Open terminal",
    es: "Abrir terminal",
    zh: "打开终端",
    de: "Terminal öffnen",
    fr: "Ouvrir le terminal",
};

const RESET: Msg = Msg {
    en: "Reset",
    es: "Restablecer",
    zh: "重置",
    de: "Zurücksetzen",
    fr: "Réinitialiser",
};

const DISCARD: Msg = Msg {
    en: "Discard",
    es: "Descartar",
    zh: "丢弃",
    de: "Verwerfen",
    fr: "Abandonner",
};

const MERGE: Msg = Msg {
    en: "Merge",
    es: "Merge",
    zh: "合并",
    de: "Mergen",
    fr: "Fusionner",
};

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
    let ask = |message: Msg, action: Callback<()>| {
        Callback::new(move |_: leptos::ev::MouseEvent| {
            if confirm(message.get()) {
                action.run(());
            }
        })
    };
    view! {
        <div class="action-list -mx-3">
            <div class="action-section">{SECTION_AGENT.get()}</div>
            {actions
                .can_finish
                .then(|| {
                    view! {
                        <ActionRow
                            icon="✓"
                            tone=ActionTone::Primary
                            disabled=busy
                            on_click=ask(
                                CONFIRM_FINISH,
                                handlers.finish,
                            )
                        >
                            {FINISH_AND_REVIEW.get()}
                        </ActionRow>
                    }
                })}
            {actions
                .can_resume
                .then(|| {
                    view! {
                        <ActionRow icon="▶" disabled=busy on_click=plain(handlers.resume)>
                            {RESUME_AGENT.get()}
                        </ActionRow>
                    }
                })}
            {actions
                .can_abort
                .then(|| {
                    view! {
                        <ActionRow icon="■" disabled=busy on_click=plain(handlers.abort)>
                            {ABORT_RUN.get()}
                        </ActionRow>
                    }
                })}
            <ActionRow icon="◈" on_click=plain(handlers.show_subagents)>
                {SUBAGENTS.get()}
                {move || {
                    let active: u32 = subagents_active.get();
                    (active > 0)
                        .then(|| view! { <Badge text=active.to_string() variant="info" /> })
                }}
            </ActionRow>
            {review
                .then(|| {
                    view! {
                        <div class="action-section">{SECTION_REVIEW.get()}</div>
                        {actions
                            .can_view_diff
                            .then(|| {
                                view! {
                                    <ActionRow icon="±" on_click=plain(handlers.open_diff)>
                                        {VIEW_DIFF.get()}
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
                                            CONFIRM_MERGE,
                                            handlers.merge,
                                        )
                                    >
                                        {MERGE.get()}
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
                            {OPEN_TERMINAL.get()}
                        </TabLink>
                    }
                })}
            {danger
                .then(|| {
                    view! {
                        <div class="action-section">{SECTION_DANGER.get()}</div>
                        {actions
                            .can_reset
                            .then(|| {
                                view! {
                                    <ActionRow
                                        icon="↺"
                                        tone=ActionTone::Danger
                                        disabled=busy
                                        on_click=ask(
                                            CONFIRM_RESET,
                                            handlers.reset,
                                        )
                                    >
                                        {RESET.get()}
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
                                            CONFIRM_DISCARD,
                                            handlers.discard,
                                        )
                                    >
                                        {DISCARD.get()}
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
            can_edit_files: true,
            ..Default::default()
        };
        assert_eq!(visible_sections(&running), [true, false, true, false]);
        let files_only: TaskActionsDto = TaskActionsDto {
            can_edit_files: true,
            ..Default::default()
        };
        assert_eq!(visible_sections(&files_only), [false, false, false, false]);
    }

    #[test]
    fn tone_maps_to_distinct_classes() {
        assert_eq!(ActionTone::Normal.class(), "action-row");
        assert!(ActionTone::Danger.class().contains("action-row-danger"));
        assert!(ActionTone::Primary.class().contains("action-row-primary"));
    }
}
