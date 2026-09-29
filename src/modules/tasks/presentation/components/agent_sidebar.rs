use leptos::prelude::*;

use crate::modules::tasks::presentation::state::transcript::Transcript;

/// Estado de extensiones, widgets y colas del agente.
#[component]
pub fn AgentSidebar(
    transcript: RwSignal<Transcript>,
    #[prop(into)] connected: Signal<bool>,
    #[prop(into, optional)] class: String,
) -> impl IntoView {
    let container_class: String = if class.is_empty() {
        "stack".to_string()
    } else {
        format!("stack {class}")
    };
    view! {
        <aside class=container_class>
            <section class="card">
                <h3 class="card-title">"Conexión"</h3>
                {move || {
                    if connected.get() {
                        view! { <span class="badge badge-ok">
                            <span class="h-1.5 w-1.5 rounded-full bg-current"></span>
                            "en vivo"
                        </span> }.into_any()
                    } else {
                        view! { <span class="badge badge-muted">"desconectado"</span> }.into_any()
                    }
                }}
            </section>
            {move || {
                transcript
                    .with(|t: &Transcript| {
                        let queued: Vec<String> = t
                            .queued_steering
                            .iter()
                            .map(|m: &String| format!("steer: {m}"))
                            .chain(t.queued_follow_up.iter().map(|m: &String| format!("follow-up: {m}")))
                            .collect();
                        (!queued.is_empty())
                            .then(|| {
                                view! {
                                    <section class="card">
                                        <h3 class="card-title">"En cola"</h3>
                                        <pre class="text-sm">{queued.join("\n")}</pre>
                                    </section>
                                }
                            })
                    })
            }}
            {move || {
                transcript
                    .with(|t: &Transcript| {
                        (!t.statuses.is_empty())
                            .then(|| {
                                let lines: String = t
                                    .statuses
                                    .iter()
                                    .map(|(k, v)| format!("{k}: {v}"))
                                    .collect::<Vec<String>>()
                                    .join("\n");
                                view! {
                                    <section class="card">
                                        <h3 class="card-title">"Estado"</h3>
                                        <pre class="text-sm">{lines}</pre>
                                    </section>
                                }
                            })
                    })
            }}
            {move || {
                transcript
                    .with(|t: &Transcript| {
                        t.widgets
                            .iter()
                            .map(|(key, lines)| {
                                view! {
                                    <section class="card">
                                        <h3 class="card-title">{key.clone()}</h3>
                                        <pre class="text-sm">{lines.join("\n")}</pre>
                                    </section>
                                }
                            })
                            .collect_view()
                    })
            }}
        </aside>
    }
}
