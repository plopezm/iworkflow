use leptos::prelude::*;

use crate::core::components::form::TextField;
use crate::core::components::modal::Modal;
use crate::core::i18n::Msg;
use crate::modules::agent_profiles::presentation::state::name_rules::slug_hint;

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

/// Diálogo genérico que pide un nombre slug (nuevo, duplicado, renombrado…).
#[component]
pub fn NameDialog(
    #[prop(into)] title: Signal<String>,
    #[prop(into)] label: String,
    #[prop(into)] confirm_label: String,
    #[prop(into)] open: Signal<bool>,
    value: RwSignal<String>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
    on_close: Callback<()>,
) -> impl IntoView {
    let hint: Memo<Option<&'static str>> = Memo::new(move |_| slug_hint(value.get().trim()));
    let label: StoredValue<String> = StoredValue::new(label);
    let confirm_label: StoredValue<String> = StoredValue::new(confirm_label);

    view! {
        <Modal title open on_close>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    if hint.get_untracked().is_none() {
                        on_submit.run(());
                    }
                }
            >
                <TextField label=label.get_value() value required=true />
                {move || {
                    hint.get()
                        .filter(|_| !value.get().is_empty())
                        .map(|text: &'static str| view! { <p class="text-sm text-error">{text}</p> })
                }}
                <div class="row gap-2">
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || hint.get().is_some()
                        class:is-busy=move || pending.get()
                    >
                        {move || confirm_label.get_value()}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        {CANCEL.get()}
                    </button>
                </div>
            </form>
        </Modal>
    }
}
