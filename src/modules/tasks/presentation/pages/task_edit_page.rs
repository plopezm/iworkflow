use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::NavigateOptions;
use leptos_router::hooks::{use_navigate, use_params_map};

use crate::core::browser::history_back;
use crate::core::components::form::{ActionError, SelectField, TextArea, TextField};
use crate::core::components::layout::ErrorText;
use crate::modules::tasks::presentation::components::image_picker::{ImagePicker, paste_callback};
use crate::modules::tasks::presentation::components::task_modal::priority_select_options;
use crate::modules::tasks::presentation::state::task_edit_state::TaskEditState;

/// Página de edición. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn TaskEditPage() -> impl IntoView {
    let params = use_params_map();
    let task_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <TaskEditContent task_id=task_id.get() /> }} }
}

#[component]
fn TaskEditContent(task_id: String) -> impl IntoView {
    let state: TaskEditState = TaskEditState::new(task_id);
    let navigate = use_navigate();
    let leave: Callback<()> = Callback::new(move |_| {
        if !history_back() {
            navigate("/", NavigateOptions::default());
        }
    });

    // Precarga una sola vez, cuando llega la tarea.
    let prefilled: RwSignal<bool> = RwSignal::new(false);
    Effect::new(move |_| {
        if let Some(Ok(task)) = state.task.get()
            && !prefilled.get_untracked()
        {
            state.prefill(&task);
            prefilled.set(true);
        }
    });
    Effect::new(move |_| {
        if state.saved() {
            leave.run(());
        }
    });

    view! {
        <Title text="Editar tarea · iworkflow" />
        <div class="mx-auto w-full max-w-5xl p-4 md:p-6">
            <Transition fallback=|| view! { <p class="muted">"Cargando tarea…"</p> }>
                {move || Suspend::new(async move {
                    match state.task.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(task) if !task.actions.can_edit => {
                            view! { <ErrorText message="Esta tarea ya no se puede editar." /> }
                                .into_any()
                        }
                        Ok(_) => {
                            view! {
                                <form
                                    class="card stack gap-4 p-5 md:p-6"
                                    on:submit=move |ev: leptos::ev::SubmitEvent| {
                                        ev.prevent_default();
                                        state.submit();
                                    }
                                >
                                    <h1 class="page-title">"Editar tarea"</h1>
                                    <TextField
                                        label="Título (opcional)"
                                        value=state.form_title
                                        placeholder="Se deriva del prompt"
                                    />
                                    <SelectField
                                        label="Prioridad"
                                        value=state.form_priority
                                        options=priority_select_options()
                                    />
                                    <TextArea
                                        label="Prompt"
                                        value=state.form_prompt
                                        placeholder="Describe lo que debe hacer el agente…"
                                        required=true
                                        large=true
                                        on_paste=paste_callback(state.images)
                                    />
                                    <ImagePicker attachments=state.images />
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
    }
}
