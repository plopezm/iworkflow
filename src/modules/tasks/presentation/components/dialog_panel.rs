use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
    DialogAnswerDto, DialogKindDto, DialogRequestDto,
};

const YES: Msg = Msg {
    en: "Yes",
    es: "Sí",
    zh: "是",
    de: "Ja",
    fr: "Oui",
};
const NO: Msg = Msg {
    en: "No",
    es: "No",
    zh: "否",
    de: "Nein",
    fr: "Non",
};
const SEND: Msg = Msg {
    en: "Send",
    es: "Enviar",
    zh: "发送",
    de: "Senden",
    fr: "Envoyer",
};
const CANCEL_DIALOG: Msg = Msg {
    en: "Cancel dialog",
    es: "Cancelar diálogo",
    zh: "取消对话框",
    de: "Dialog abbrechen",
    fr: "Annuler la boîte de dialogue",
};

/// Panel para responder a un diálogo abierto por el agente.
#[component]
pub fn DialogPanel(
    request: DialogRequestDto,
    on_answer: Callback<DialogAnswerDto>,
) -> impl IntoView {
    let initial: String = match &request.kind {
        DialogKindDto::Editor { prefill } => prefill.clone().unwrap_or_default(),
        _ => String::new(),
    };
    let text: RwSignal<String> = RwSignal::new(initial);
    let submit_text = move |_| {
        on_answer.run(DialogAnswerDto::Value {
            value: text.get_untracked(),
        })
    };

    let body: AnyView = match request.kind {
        DialogKindDto::Select { options } => view! {
            <div class="stack gap-2">
                {options
                    .into_iter()
                    .map(|option: String| {
                        let value: String = option.clone();
                        view! {
                            <button
                                class="btn-secondary min-h-9 w-full rounded-lg px-3 py-2 text-left text-sm whitespace-normal break-words"
                                on:click=move |_| {
                                    on_answer.run(DialogAnswerDto::Value { value: value.clone() })
                                }
                            >
                                {option}
                            </button>
                        }
                    })
                    .collect_view()}
            </div>
        }
        .into_any(),
        DialogKindDto::Confirm { message } => view! {
            {message.map(|m: String| view! { <p>{m}</p> })}
            <div class="row gap-2">
                <button
                    class="btn-primary"
                    on:click=move |_| { on_answer.run(DialogAnswerDto::Confirmed { confirmed: true }) }
                >
                    {YES.get()}
                </button>
                <button
                    class="btn-secondary"
                    on:click=move |_| {
                        on_answer.run(DialogAnswerDto::Confirmed { confirmed: false })
                    }
                >
                    {NO.get()}
                </button>
            </div>
        }
        .into_any(),
        DialogKindDto::Input { placeholder } => view! {
            <div class="row gap-2">
                <input
                    type="text"
                    class="input min-w-0 flex-1"
                    placeholder=placeholder.unwrap_or_default()
                    prop:value=move || text.get()
                    on:input=move |ev| text.set(event_target_value(&ev))
                />
                <button class="btn-primary" on:click=submit_text>
                    {SEND.get()}
                </button>
            </div>
        }
        .into_any(),
        DialogKindDto::Editor { .. } => view! {
            <div class="stack">
                <textarea
                    class="input"
                    prop:value=move || text.get()
                    on:input=move |ev| text.set(event_target_value(&ev))
                ></textarea>
                <div class="row">
                    <button class="btn-primary" on:click=submit_text>
                        {SEND.get()}
                    </button>
                </div>
            </div>
        }
        .into_any(),
    };

    view! {
        <section class="card dialog-panel max-h-[50vh] animate-slide-up overflow-y-auto">
            <h3 class="card-title whitespace-pre-wrap break-words">{format!("❓ {}", request.title)}</h3>
            {body}
            <div class="row">
                <button class="btn-secondary" on:click=move |_| on_answer.run(DialogAnswerDto::Cancelled)>
                    {CANCEL_DIALOG.get()}
                </button>
            </div>
        </section>
    }
}
