use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_file_dto::TaskFileDto;
use crate::modules::tasks::presentation::state::diff_view_state::{
    DiffViewState, effective_content, is_dirty,
};

/// Fewest rows the textarea shows.
const MIN_ROWS: usize = 8;
/// Most rows the textarea grows to before scrolling.
const MAX_ROWS: usize = 28;

/// Visible rows for `content`: fits the text within [`MIN_ROWS`, `MAX_ROWS`] (pure).
pub fn editor_rows(content: &str) -> usize {
    (content.lines().count() + 1).clamp(MIN_ROWS, MAX_ROWS)
}

/// Inline editor of one file: loads its content, saves it (write + commit) or cancels.
#[component]
pub fn DiffFileEditor(#[prop(into)] path: String, state: DiffViewState) -> impl IntoView {
    let pending: Memo<bool> = state.save.pending();
    view! {
        <div class="diff-editor">
            <Suspense fallback=|| view! { <p class="muted p-3">"Cargando fichero…"</p> }>
                {move || {
                    let path: String = path.clone();
                    Suspend::new(async move {
                        match state.file.await {
                            Some(Ok(file)) => editor_body(file, path, state, pending).into_any(),
                            Some(Err(e)) => {
                                view! {
                                    <div class="stack p-3">
                                        <ErrorText message=e.to_string() />
                                        <div>
                                            <button
                                                type="button"
                                                class="btn-ghost"
                                                on:click=move |_| state.cancel_edit()
                                            >
                                                "Cerrar"
                                            </button>
                                        </div>
                                    </div>
                                }
                                    .into_any()
                            }
                            None => ().into_any(),
                        }
                    })
                }}
            </Suspense>
        </div>
    }
}

fn editor_body(
    file: TaskFileDto,
    path: String,
    state: DiffViewState,
    pending: Memo<bool>,
) -> impl IntoView {
    let loaded: StoredValue<String> = StoredValue::new(file.content);
    // Remounting must show what Save would write: keep a draft typed before the remount.
    let initial: String = state.draft.with_untracked(|d: &Option<String>| {
        loaded.with_value(|l: &String| effective_content(d.as_deref(), l))
    });
    let rows: usize = editor_rows(&initial);
    let dirty: Signal<bool> = Signal::derive(move || {
        state
            .draft
            .with(|d: &Option<String>| loaded.with_value(|l: &String| is_dirty(d.as_deref(), l)))
    });
    let save_path: String = path;
    view! {
        <div class="stack p-3">
            <textarea
                class="diff-editor-input"
                rows=rows
                spellcheck="false"
                aria-label="Contenido del fichero"
                prop:value=initial
                prop:disabled=move || pending.get()
                on:input=move |ev| state.draft.set(Some(event_target_value(&ev)))
            ></textarea>
            {move || state.save_error().map(|message: String| view! { <ErrorText message /> })}
            <Show when=move || state.save_unchanged()>
                <p class="notice notice-info animate-fade-in">
                    "Sin cambios: el fichero ya tenía ese contenido, no se creó ningún commit."
                </p>
            </Show>
            <div class="flex flex-wrap items-center gap-2">
                <button
                    type="button"
                    class="btn-primary"
                    disabled=move || pending.get()
                    on:click=move |_| {
                        loaded.with_value(|l: &String| state.submit(&save_path, l));
                    }
                >
                    {move || if pending.get() { "Guardando…" } else { "Guardar" }}
                </button>
                <button
                    type="button"
                    class="btn-ghost"
                    disabled=move || pending.get()
                    on:click=move |_| state.cancel_edit()
                >
                    "Cancelar"
                </button>
                <Show when=move || dirty.get()>
                    <span class="muted text-xs">"Cambios sin guardar"</span>
                </Show>
                <span class="muted ml-auto text-xs">"Guardar crea un commit en la rama de la tarea."</span>
            </div>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rows_are_clamped() {
        assert_eq!(editor_rows(""), 8);
        assert_eq!(editor_rows(&"x\n".repeat(15)), 16);
        assert_eq!(editor_rows(&"x\n".repeat(500)), 28);
    }
}
