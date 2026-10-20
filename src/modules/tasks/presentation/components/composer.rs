use std::future::Future;
use std::pin::Pin;

use leptos::ev::KeyboardEvent;
use leptos::html::Textarea;
use leptos::prelude::*;
use leptos::web_sys::HtmlTextAreaElement;

use crate::core::i18n::Msg;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::AgentModeDto;
use crate::modules::tasks::presentation::components::image_picker::{
    ImageAttachButton, ImagePreviews, paste_callback,
};
use crate::modules::tasks::presentation::state::composer_completion::{
    self as completion, SlashCommand, Trigger, TriggerKind,
};
use crate::modules::tasks::presentation::state::image_attachments::ImageAttachments;

const MESSAGE_PLACEHOLDER: Msg = Msg {
    en: "Write a message for the agent… (@ for files, / for commands, Ctrl+Enter to send)",
    es: "Escribe un mensaje para el agente… (@ ficheros, / comandos, Ctrl+Enter para enviar)",
    zh: "给智能体写一条消息…（@ 引用文件，/ 命令，Ctrl+Enter 发送）",
    de: "Schreibe eine Nachricht an den Agent … (@ Dateien, / Befehle, Strg+Enter zum Senden)",
    fr: "Écrivez un message pour l’agent… (@ fichiers, / commandes, Ctrl+Entrée pour envoyer)",
};
const MODE_LABEL: Msg = Msg {
    en: "Agent mode",
    es: "Modo del agente",
    zh: "智能体模式",
    de: "Agent-Modus",
    fr: "Mode de l’agent",
};
const MODE_AGENT: Msg = Msg {
    en: "Agent",
    es: "Agente",
    zh: "智能体",
    de: "Agent",
    fr: "Agent",
};
const MODE_PLAN: Msg = Msg {
    en: "Plan",
    es: "Plan",
    zh: "计划",
    de: "Plan",
    fr: "Plan",
};
const SEND: Msg = Msg {
    en: "Send",
    es: "Enviar",
    zh: "发送",
    de: "Senden",
    fr: "Envoyer",
};
const NO_FILES: Msg = Msg {
    en: "No matching files",
    es: "Ningún fichero coincide",
    zh: "没有匹配的文件",
    de: "Keine passenden Dateien",
    fr: "Aucun fichier correspondant",
};
const SUGGESTIONS_LABEL: Msg = Msg {
    en: "Suggestions",
    es: "Sugerencias",
    zh: "建议",
    de: "Vorschläge",
    fr: "Suggestions",
};
const REMOVE_REFERENCE: Msg = Msg {
    en: "Remove reference {name}",
    es: "Quitar la referencia {name}",
    zh: "移除引用 {name}",
    de: "Verweis {name} entfernen",
    fr: "Retirer la référence {name}",
};

/// Búsqueda de ficheros del proyecto de la tarea: recibe `(id de tarea, texto)` y devuelve rutas.
/// La inyecta quien compone la página para que `tasks` no importe `project_tree`.
pub type FileSearchFn = fn(String, String) -> Pin<Box<dyn Future<Output = Vec<String>>>>;

/// Una opción del menú de autocompletado.
#[derive(Debug, Clone, PartialEq, Eq)]
struct Suggestion {
    /// Texto que sustituye al token (`/plan`, `@src/main.rs`).
    insert: String,
    /// Texto principal de la fila.
    label: String,
    /// Explicación opcional (comandos).
    hint: String,
}

fn command_suggestions(query: &str) -> Vec<Suggestion> {
    completion::matching_commands(query)
        .into_iter()
        .map(|command: SlashCommand| Suggestion {
            insert: format!("/{}", command.name),
            label: format!("/{}", command.name),
            hint: command.description.get().to_string(),
        })
        .collect()
}

fn file_suggestions(paths: Vec<String>) -> Vec<Suggestion> {
    paths
        .into_iter()
        .map(|path: String| Suggestion {
            insert: format!("@{path}"),
            label: path,
            hint: String::new(),
        })
        .collect()
}

/// Caja de mensajes para el agente. Ctrl/Cmd+Enter envía; los mensajes son siempre `Steer`.
/// `/` al inicio abre los comandos y `@` los ficheros del worktree de la tarea (`scope` = id de tarea), si hay `file_search`.
#[component]
pub fn Composer(
    message: RwSignal<String>,
    attachments: ImageAttachments,
    #[prop(into)] pending: Signal<bool>,
    on_send: Callback<()>,
    #[prop(into, default = String::new())] scope: String,
    #[prop(optional_no_strip)] file_search: Option<FileSearchFn>,
    #[prop(optional_no_strip)] mode: Option<Signal<Option<String>>>,
    #[prop(optional_no_strip)] on_mode: Option<Callback<AgentModeDto>>,
) -> impl IntoView {
    let textarea: NodeRef<Textarea> = NodeRef::new();
    // Posición del cursor en unidades UTF-16 (la que da el navegador).
    let caret: RwSignal<usize> = RwSignal::new(0);
    let dismissed: RwSignal<bool> = RwSignal::new(false);
    let active: RwSignal<usize> = RwSignal::new(0);
    let scope: StoredValue<String> = StoredValue::new(scope);

    let trigger: Memo<Option<Trigger>> = Memo::new(move |_| {
        if dismissed.get() {
            return None;
        }
        message.with(|text: &String| {
            completion::detect(text, completion::byte_offset(text, caret.get()))
        })
    });
    let file_query: Memo<Option<String>> = Memo::new(move |_| {
        trigger
            .get()
            .filter(|t: &Trigger| t.kind == TriggerKind::File && file_search.is_some())
            .map(|t: Trigger| t.query)
    });
    let files: LocalResource<Vec<String>> = LocalResource::new(move || async move {
        match (file_query.get(), file_search) {
            (Some(query), Some(search)) => search(scope.get_value(), query).await,
            _ => Vec::new(),
        }
    });
    let suggestions: Memo<Vec<Suggestion>> = Memo::new(move |_| match trigger.get() {
        Some(t) if t.kind == TriggerKind::Command => command_suggestions(&t.query),
        Some(t) if t.kind == TriggerKind::File && file_search.is_some() => {
            file_suggestions(files.get().unwrap_or_default())
        }
        _ => Vec::new(),
    });
    let menu_open: Memo<bool> = Memo::new(move |_| {
        trigger
            .get()
            .is_some_and(|t: Trigger| t.kind == TriggerKind::Command || file_search.is_some())
    });
    let references: Memo<Vec<String>> =
        Memo::new(move |_| message.with(|text: &String| completion::references(text)));

    // El resaltado vuelve al primero cada vez que cambia la lista.
    Effect::new(move |_| {
        suggestions.track();
        active.set(0);
    });

    let sync_caret = move |element: HtmlTextAreaElement| {
        let position: usize = element
            .selection_start()
            .ok()
            .flatten()
            .map_or(0, |p: u32| p as usize);
        caret.set(position);
    };

    let accept = move |index: usize| {
        let (Some(t), Some(choice)) = (
            trigger.get_untracked(),
            suggestions.with_untracked(|items: &Vec<Suggestion>| items.get(index).cloned()),
        ) else {
            return;
        };
        let (text, byte_caret): (String, usize) =
            message.with_untracked(|text: &String| completion::apply(text, &t, &choice.insert));
        let utf16: usize = completion::utf16_offset(&text, byte_caret);
        message.set(text);
        caret.set(utf16);
        request_animation_frame(move || {
            if let Some(element) = textarea.get() {
                let position: u32 = utf16 as u32;
                // El foco y el cursor son cosmética: si el navegador los rechaza no se pierde nada.
                let _ = element.focus();
                let _ = element.set_selection_range(position, position);
            }
        });
    };

    let on_keydown = move |ev: KeyboardEvent| {
        // Enter/Tab confirman la composición de un IME: no son para el menú ni para enviar.
        if ev.is_composing() {
            return;
        }
        let count: usize = suggestions.with_untracked(Vec::len);
        if menu_open.get_untracked() && count > 0 {
            match ev.key().as_str() {
                "ArrowDown" => {
                    ev.prevent_default();
                    active.update(|i: &mut usize| *i = (*i + 1) % count);
                    return;
                }
                "ArrowUp" => {
                    ev.prevent_default();
                    active.update(|i: &mut usize| *i = (*i + count - 1) % count);
                    return;
                }
                "Enter" | "Tab" if !(ev.ctrl_key() || ev.meta_key()) => {
                    ev.prevent_default();
                    accept(active.get_untracked());
                    return;
                }
                _ => {}
            }
        }
        if ev.key() == "Escape" && menu_open.get_untracked() {
            ev.prevent_default();
            dismissed.set(true);
            return;
        }
        if ev.key() == "Enter" && (ev.ctrl_key() || ev.meta_key()) {
            ev.prevent_default();
            on_send.run(());
        }
    };

    let indexed =
        move || -> Vec<(usize, Suggestion)> { suggestions.get().into_iter().enumerate().collect() };
    let on_paste: Callback<leptos::ev::ClipboardEvent> = paste_callback(attachments);

    let current_mode: Memo<Option<String>> =
        Memo::new(move |_| mode.and_then(|m: Signal<Option<String>>| m.get()));
    let mode_button = move |target: AgentModeDto, label: Msg| {
        view! {
            <button
                type="button"
                class="composer-mode-option"
                class:is-active=move || current_mode.get().as_deref() == Some(target.as_str())
                aria-pressed=move || {
                    (current_mode.get().as_deref() == Some(target.as_str())).to_string()
                }
                on:click=move |_| {
                    if let Some(callback) = on_mode {
                        callback.run(target);
                    }
                }
            >
                {label.get()}
            </button>
        }
    };

    view! {
        <div class="stack composer">
            <Show when=move || current_mode.get().is_some()>
                <div class="composer-mode" role="group" aria-label=MODE_LABEL.get()>
                    {mode_button(AgentModeDto::Agent, MODE_AGENT)}
                    {mode_button(AgentModeDto::Plan, MODE_PLAN)}
                </div>
            </Show>
            <Show when=move || !references.get().is_empty()>
                <ul class="composer-refs" aria-label="@">
                    <For
                        each=move || references.get()
                        key=|path: &String| path.clone()
                        children=move |path: String| {
                            let remove_label: String = REMOVE_REFERENCE.fmt(&[("name", &path)]);
                            let to_remove: String = path.clone();
                            view! {
                                <li class="composer-ref">
                                    <span class="truncate">{format!("@{path}")}</span>
                                    <button
                                        type="button"
                                        class="composer-ref-remove"
                                        aria-label=remove_label
                                        on:click=move |_| {
                                            message
                                                .update(|text: &mut String| {
                                                    *text = completion::remove_reference(text, &to_remove);
                                                })
                                        }
                                    >
                                        "×"
                                    </button>
                                </li>
                            }
                        }
                    />
                </ul>
            </Show>
            <div class="composer-field">
                <Show when=move || menu_open.get()>
                    <ul class="composer-menu" role="listbox" aria-label=SUGGESTIONS_LABEL.get()>
                        <Show
                            when=move || !suggestions.with(Vec::is_empty)
                            fallback=move || {
                                view! { <li class="composer-menu-empty">{NO_FILES.get()}</li> }
                            }
                        >
                            <For
                                each=indexed
                                key=|(index, item): &(usize, Suggestion)| (*index, item.insert.clone())
                                children=move |(index, item): (usize, Suggestion)| {
                                    view! {
                                        <li
                                            role="option"
                                            class="composer-menu-item"
                                            class:is-active=move || active.get() == index
                                            aria-selected=move || (active.get() == index).to_string()
                                            on:mousedown=move |ev| {
                                                ev.prevent_default();
                                                accept(index);
                                            }
                                            on:mouseenter=move |_| active.set(index)
                                        >
                                            <span class="truncate">{item.label}</span>
                                            <span class="composer-menu-hint truncate">{item.hint}</span>
                                        </li>
                                    }
                                }
                            />
                        </Show>
                    </ul>
                </Show>
                <textarea
                    node_ref=textarea
                    placeholder=MESSAGE_PLACEHOLDER.get()
                    prop:value=move || message.get()
                    on:input=move |ev| {
                        dismissed.set(false);
                        message.set(event_target_value(&ev));
                        sync_caret(event_target::<HtmlTextAreaElement>(&ev));
                    }
                    on:keydown=on_keydown
                    on:keyup=move |ev| sync_caret(event_target::<HtmlTextAreaElement>(&ev))
                    on:click=move |ev| sync_caret(event_target::<HtmlTextAreaElement>(&ev))
                    on:paste=move |ev| on_paste.run(ev)
                ></textarea>
            </div>
            <ImagePreviews attachments />
            <div class="flex items-center justify-between gap-2">
                <button
                    class="btn-primary"
                    disabled=move || {
                        pending.get()
                            || (message.with(|m: &String| m.trim().is_empty())
                                && attachments.is_empty())
                    }
                    class:is-busy=move || pending.get()
                    on:click=move |_| on_send.run(())
                >
                    {SEND.get()}
                </button>
                <ImageAttachButton attachments />
            </div>
        </div>
    }
}
