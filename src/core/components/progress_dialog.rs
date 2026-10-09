use leptos::prelude::*;

use crate::core::components::modal::Modal;

/// Cancelable dialog with an indeterminate progress bar for long-running work.
///
/// Closing it by any means (button, backdrop, Escape, header close) runs `on_cancel`.
#[component]
pub fn ProgressDialog(
    #[prop(into)] title: Signal<String>,
    #[prop(into)] message: Signal<String>,
    #[prop(into)] open: Signal<bool>,
    on_cancel: Callback<()>,
) -> impl IntoView {
    view! {
        <Modal title open on_close=on_cancel>
            <p class="text-sm text-muted">{move || message.get()}</p>
            <div
                class="progress-track"
                role="progressbar"
                aria-busy="true"
                aria-valuetext="En curso"
            >
                <div class="progress-bar"></div>
            </div>
            <div class="row justify-end">
                <button type="button" class="btn-secondary" on:click=move |_| on_cancel.run(())>
                    "Cancelar"
                </button>
            </div>
        </Modal>
    }
}
