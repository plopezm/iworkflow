use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::NavigateOptions;
use leptos_router::hooks::{use_navigate, use_params_map};

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextField};
use crate::core::components::layout::ErrorText;
use crate::modules::workspaces::presentation::state::workspace_edit_state::{
    WorkspaceEditState, split_agents_options,
};

const WORKSPACES_HREF: &str = "/workspaces";

/// Página de edición. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn WorkspaceEditPage() -> impl IntoView {
    let params = use_params_map();
    let workspace_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <WorkspaceEditContent workspace_id=workspace_id.get() /> }} }
}

#[component]
fn WorkspaceEditContent(workspace_id: String) -> impl IntoView {
    let state: WorkspaceEditState = WorkspaceEditState::new(workspace_id);
    let navigate = use_navigate();
    let leave: Callback<()> = Callback::new(move |_| {
        navigate(WORKSPACES_HREF, NavigateOptions::default());
    });

    // Precarga una sola vez, cuando llega el workspace.
    let prefilled: RwSignal<bool> = RwSignal::new(false);
    Effect::new(move |_| {
        if let Some(Ok(workspace)) = state.workspace.get()
            && !prefilled.get_untracked()
        {
            state.prefill(&workspace);
            prefilled.set(true);
        }
    });
    Effect::new(move |_| {
        if state.saved() {
            leave.run(());
        }
    });

    view! {
        <Title text="Editar workspace · iworkflow" />
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-3xl p-4 pb-10 md:p-6 md:pb-12">
                <Transition fallback=|| view! { <p class="muted">"Cargando workspace…"</p> }>
                    {move || Suspend::new(async move {
                        match state.workspace.await {
                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                            Ok(workspace) => {
                                let (options, options_error): (
                                    Vec<SelectOption>,
                                    Option<String>,
                                ) = split_agents_options(state.agents_file_options.await);
                                view! {
                                    <form
                                        class="card stack gap-4 p-5 md:p-6"
                                        on:submit=move |ev: leptos::ev::SubmitEvent| {
                                            ev.prevent_default();
                                            state.submit();
                                        }
                                    >
                                        <h1 class="page-title">"Editar workspace"</h1>
                                        <TextField
                                            label="Nombre"
                                            value=state.form_name
                                            placeholder="mi-proyecto"
                                            required=true
                                        />
                                        <div class="stack gap-1.5">
                                            <span class="text-sm font-medium text-fg">"Ruta"</span>
                                            <code class="truncate font-mono text-xs text-muted">
                                                {workspace.path}
                                            </code>
                                        </div>
                                        <TextField
                                            label="Rama base"
                                            value=state.form_base_branch
                                            placeholder="main"
                                        />
                                        {options_error
                                            .map(|message: String| {
                                                view! { <ErrorText message /> }
                                            })}
                                        {(!options.is_empty())
                                            .then(|| {
                                                view! {
                                                    <SelectField
                                                        label="Sustituir AGENTS.md (opcional)"
                                                        value=state.form_agents_file_id
                                                        options
                                                        placeholder="Mantener el actual"
                                                    />
                                                }
                                            })}
                                        <ActionError result=state.update.value().into() />
                                        <div class="row justify-end gap-2">
                                            <button
                                                type="button"
                                                class="btn-secondary"
                                                on:click=move |_| leave.run(())
                                            >
                                                "Cancelar"
                                            </button>
                                            <button
                                                type="submit"
                                                class="btn-primary"
                                                disabled=move || state.update.pending().get()
                                            >
                                                {move || {
                                                    if state.update.pending().get() {
                                                        "Guardando…"
                                                    } else {
                                                        "Guardar cambios"
                                                    }
                                                }}
                                            </button>
                                        </div>
                                    </form>
                                }
                                    .into_any()
                            }
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}
