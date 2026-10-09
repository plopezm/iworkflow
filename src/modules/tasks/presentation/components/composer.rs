use leptos::ev::KeyboardEvent;
use leptos::prelude::*;

use crate::modules::tasks::presentation::components::image_picker::{
    ImageAttachButton, ImagePreviews, paste_callback,
};
use crate::modules::tasks::presentation::state::image_attachments::ImageAttachments;

/// Caja de mensajes para el agente. Ctrl/Cmd+Enter envía; los mensajes son siempre `Steer`.
#[component]
pub fn Composer(
    message: RwSignal<String>,
    attachments: ImageAttachments,
    #[prop(into)] pending: Signal<bool>,
    on_send: Callback<()>,
) -> impl IntoView {
    let on_keydown = move |ev: KeyboardEvent| {
        if ev.key() == "Enter" && (ev.ctrl_key() || ev.meta_key()) {
            ev.prevent_default();
            on_send.run(());
        }
    };

    let on_paste: Callback<leptos::ev::ClipboardEvent> = paste_callback(attachments);

    view! {
        <div class="stack composer">
            <textarea
                placeholder="Escribe un mensaje para el agente… (Ctrl+Enter para enviar)"
                prop:value=move || message.get()
                on:input=move |ev| message.set(event_target_value(&ev))
                on:keydown=on_keydown
                on:paste=move |ev| on_paste.run(ev)
            ></textarea>
            <ImagePreviews attachments />
            <div class="flex items-center justify-between gap-2">
                <button
                    class="btn-primary"
                    disabled=move || {
                        pending.get()
                            || (message.with(|m: &String| m.trim().is_empty())
                                && attachments.is_empty())
                    }
                    on:click=move |_| on_send.run(())
                >
                    "Enviar"
                </button>
                <ImageAttachButton attachments />
            </div>
        </div>
    }
}
