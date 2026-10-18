use leptos::prelude::*;

use crate::core::i18n::Msg;

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

/// Panel de aviso inline con un mensaje y dos acciones (confirmar / cancelar).
#[component]
pub fn ConfirmPanel(
    #[prop(into)] message: Signal<String>,
    #[prop(into)] confirm_label: Signal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_confirm: Callback<()>,
    on_cancel: Callback<()>,
) -> impl IntoView {
    view! {
        <div class="stack gap-2 rounded-xl border border-border/60 bg-surface/80 p-3 backdrop-blur-sm">
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
                    class="btn-secondary"
                    disabled=move || pending.get()
                    on:click=move |_| on_cancel.run(())
                >
                    {CANCEL.get()}
                </button>
            </div>
        </div>
    }
}
