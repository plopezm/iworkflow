use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{SelectField, SelectOption, TextField};
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::AgentsFileOptionDto;

const PATH_INPUT_ID: &str = "workspace-path";

/// Formulario de alta de workspace. El envío se delega en `on_submit`.
#[component]
pub fn WorkspaceForm(
    name: RwSignal<String>,
    path: RwSignal<String>,
    base_branch: RwSignal<String>,
    agents_file_id: RwSignal<String>,
    agents_file_options: Resource<Result<Vec<AgentsFileOptionDto>, ServerFnError>>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
    on_browse: Callback<()>,
) -> impl IntoView {
    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        on_submit.run(());
    };

    view! {
        <form class="stack" on:submit=submit>
            <TextField label="Nombre" value=name placeholder="mi-proyecto" required=true />
            <div class="stack gap-1.5">
                <label for=PATH_INPUT_ID class="text-sm font-medium text-fg">
                    "Ruta absoluta (se crea si no existe)"
                </label>
                <div class="row gap-2">
                    <input
                        id=PATH_INPUT_ID
                        type="text"
                        class="input min-w-0 flex-1"
                        placeholder="/home/usuario/proyectos/mi-proyecto"
                        required=true
                        prop:value=move || path.get()
                        on:input=move |ev| path.set(event_target_value(&ev))
                    />
                    <button type="button" class="btn-ghost" on:click=move |_| on_browse.run(())>
                        "Elegir…"
                    </button>
                </div>
            </div>
            <TextField
                label="Rama base (vacío = rama actual)"
                value=base_branch
                placeholder="main"
            />
            <Transition fallback=|| ()>
                {move || Suspend::new(async move {
                    let options: Vec<SelectOption> = agents_file_options
                        .await
                        .unwrap_or_default()
                        .into_iter()
                        .map(|o: AgentsFileOptionDto| SelectOption {
                            value: o.id,
                            label: o.description,
                        })
                        .collect();
                    (!options.is_empty()).then(|| {
                        view! {
                            <SelectField
                                label="AGENTS.md inicial (opcional)"
                                value=agents_file_id
                                options
                                placeholder="Sin AGENTS.md"
                            />
                        }
                    })
                })}
            </Transition>
            <div class="row">
                <button type="submit" class="btn-primary" disabled=move || pending.get()>
                    {move || if pending.get() { "Registrando…" } else { "Registrar" }}
                </button>
            </div>
        </form>
    }
}
