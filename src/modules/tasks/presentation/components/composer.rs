use leptos::ev::KeyboardEvent;
use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::MessageModeDto;

/// Caja de mensajes para el agente. Ctrl/Cmd+Enter envía.
#[component]
pub fn Composer(
    message: RwSignal<String>,
    mode: RwSignal<MessageModeDto>,
    #[prop(into)] agent_busy: Signal<bool>,
    #[prop(into)] pending: Signal<bool>,
    on_send: Callback<()>,
) -> impl IntoView {
    let on_keydown = move |ev: KeyboardEvent| {
        if ev.key() == "Enter" && (ev.ctrl_key() || ev.meta_key()) {
            ev.prevent_default();
            on_send.run(());
        }
    };

    view! {
        <div class="stack composer sticky bottom-0 z-10 border-t border-border/40 bg-surface/80 p-3 backdrop-blur-xl sm:static sm:border-0 sm:bg-transparent sm:p-0">
            <textarea
                placeholder="Escribe un mensaje para el agente… (Ctrl+Enter para enviar)"
                prop:value=move || message.get()
                on:input=move |ev| message.set(event_target_value(&ev))
                on:keydown=on_keydown
            ></textarea>
            <div class="row">
                <Show when=move || agent_busy.get()>
                    <label class="row">
                        <input
                            type="radio"
                            name="mode"
                            prop:checked=move || mode.get() == MessageModeDto::Steer
                            on:change=move |_| mode.set(MessageModeDto::Steer)
                        />
                        "Steer (tras las tools en curso)"
                    </label>
                    <label class="row">
                        <input
                            type="radio"
                            name="mode"
                            prop:checked=move || mode.get() == MessageModeDto::FollowUp
                            on:change=move |_| mode.set(MessageModeDto::FollowUp)
                        />
                        "Follow-up (al terminar)"
                    </label>
                </Show>
                <button
                    class="btn-primary"
                    disabled=move || pending.get() || message.with(|m: &String| m.trim().is_empty())
                    on:click=move |_| on_send.run(())
                >
                    "Enviar"
                </button>
            </div>
        </div>
    }
}
