use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::TextField;

/// Formulario de alta de workspace. El envío se delega en `on_submit`.
#[component]
pub fn WorkspaceForm(
    name: RwSignal<String>,
    path: RwSignal<String>,
    base_branch: RwSignal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
) -> impl IntoView {
    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        on_submit.run(());
    };

    view! {
        <form class="stack" on:submit=submit>
            <TextField label="Nombre" value=name placeholder="mi-proyecto" required=true />
            <TextField
                label="Ruta absoluta del repositorio"
                value=path
                placeholder="/home/usuario/proyectos/mi-proyecto"
                required=true
            />
            <TextField
                label="Rama base (vacío = rama actual)"
                value=base_branch
                placeholder="main"
            />
            <div class="row">
                <button type="submit" disabled=move || pending.get()>
                    {move || if pending.get() { "Registrando…" } else { "Registrar" }}
                </button>
            </div>
        </form>
    }
}
