use leptos::prelude::*;

/// Panel de aviso inline con un mensaje y dos acciones (confirmar / cancelar).
/// Genérico: no conoce el dominio que lo usa, solo el texto y las etiquetas.
#[component]
pub fn ConfirmPanel(
    #[prop(into)] message: Signal<String>,
    #[prop(into)] confirm_label: Signal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_confirm: Callback<()>,
    on_cancel: Callback<()>,
) -> impl IntoView {
    view! {
        <div class="stack gap-2 rounded-lg border border-border bg-surface p-3">
            <p class="text-sm text-fg">{move || message.get()}</p>
            <div class="row gap-2">
                <button
                    type="button"
                    class="btn-primary"
                    disabled=move || pending.get()
                    on:click=move |_| on_confirm.run(())
                >
                    {move || confirm_label.get()}
                </button>
                <button
                    type="button"
                    class="btn-ghost"
                    disabled=move || pending.get()
                    on:click=move |_| on_cancel.run(())
                >
                    "Cancelar"
                </button>
            </div>
        </div>
    }
}
