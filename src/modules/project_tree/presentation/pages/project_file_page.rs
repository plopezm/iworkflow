use leptos::prelude::*;

use crate::core::components::code::CodeEditor;
use crate::core::components::layout::ErrorText;
use crate::core::dock::context::use_tab_params;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::ProjectFileDto;
use crate::modules::project_tree::presentation::state::project_file_state::{
    ProjectFileState, is_dirty,
};

/// Visor/editor de un fichero del proyecto. Se reconstruye por cada (workspace, ruta).
#[component]
pub fn ProjectFilePage() -> impl IntoView {
    let params = use_tab_params();
    let target: Memo<(String, String)> = Memo::new(move |_| {
        params.with(|p| {
            (
                p.get("id").unwrap_or_default(),
                p.get("path").unwrap_or_default(),
            )
        })
    });

    view! {
        {move || {
            let (workspace_id, path): (String, String) = target.get();
            view! { <ProjectFileContent workspace_id path /> }
        }}
    }
}

#[component]
fn ProjectFileContent(workspace_id: String, path: String) -> impl IntoView {
    let state: ProjectFileState = ProjectFileState::new(workspace_id, path.clone());
    let title: String = path.clone();
    let tooltip: String = path.clone();

    view! {
        <div class="flex h-full min-h-0 flex-col bg-surface">
            <div class="tree-panel-header">
                <h1 class="min-w-0 flex-1 truncate font-mono text-xs font-semibold" title=tooltip>
                    {title}
                </h1>
            </div>
            <Transition fallback=|| {
                view! { <p class="px-3 py-2 text-xs text-muted">"Cargando fichero…"</p> }
            }>
                {move || {
                    let path: String = path.clone();
                    Suspend::new(async move {
                        match state.file.await {
                            Ok(file) => file_body(file, state).into_any(),
                            Err(e) => {
                                view! {
                                    <div class="px-3 py-2">
                                        <ErrorText message=e.to_string() />
                                        <p class="mt-1 font-mono text-xs text-muted">{path}</p>
                                    </div>
                                }
                                    .into_any()
                            }
                        }
                    })
                }}
            </Transition>
        </div>
    }
}

fn file_body(file: ProjectFileDto, state: ProjectFileState) -> impl IntoView {
    let ProjectFileDto { name, content, .. } = file;
    state.draft.set(content.clone());
    let loaded: StoredValue<String> = StoredValue::new(content);
    let saving: Memo<bool> = state.saving();
    let dirty: Signal<bool> = Signal::derive(move || {
        state
            .draft
            .with(|d: &String| loaded.with_value(|l: &String| is_dirty(d, l)))
    });
    let save: Callback<()> = Callback::new(move |_| {
        if dirty.get_untracked() {
            state.submit();
        }
    });

    view! {
        <div class="flex items-center gap-2 border-b border-border px-3 py-1.5">
            <button
                type="button"
                class="btn-primary"
                disabled=move || saving.get() || !dirty.get()
                on:click=move |_| save.run(())
            >
                {move || if saving.get() { "Guardando…" } else { "Guardar" }}
            </button>
            <button
                type="button"
                class="btn-ghost"
                disabled=move || saving.get() || !dirty.get()
                on:click=move |_| state.draft.set(loaded.get_value())
            >
                "Descartar"
            </button>
            <button type="button" class="icon-btn" title="Recargar desde disco" on:click=move |_| state.refresh()>
                "↻"
            </button>
            <Show when=move || dirty.get()>
                <span class="text-xs text-muted">"Cambios sin guardar"</span>
            </Show>
        </div>
        {move || state.save_error().map(|message: String| view! { <div class="px-3 py-1"><ErrorText message /></div> })}
        <CodeEditor value=state.draft filename=name.clone() disabled=saving on_save=save marks=state.marks />
    }
}
