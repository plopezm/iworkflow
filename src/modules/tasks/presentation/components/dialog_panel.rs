use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
    DialogAnswerDto, DialogKindDto, DialogRequestDto,
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
            <div class="row flex-wrap gap-2">
                {options
                    .into_iter()
                    .map(|option: String| {
                        let value: String = option.clone();
                        view! {
                            <button
                                class="btn-secondary min-h-9 rounded-lg px-3 py-1.5 text-xs"
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
                    "Sí"
                </button>
                <button
                    class="btn-secondary"
                    on:click=move |_| {
                        on_answer.run(DialogAnswerDto::Confirmed { confirmed: false })
                    }
                >
                    "No"
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
                    "Enviar"
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
                        "Enviar"
                    </button>
                </div>
            </div>
        }
        .into_any(),
    };

    view! {
        <section class="card dialog-panel animate-slide-up">
            <h3 class="card-title">{format!("❓ {}", request.title)}</h3>
            {body}
            <div class="row">
                <button class="btn-secondary" on:click=move |_| on_answer.run(DialogAnswerDto::Cancelled)>
                    "Cancelar diálogo"
                </button>
            </div>
        </section>
    }
}
