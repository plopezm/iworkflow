use leptos::prelude::*;

/// Tarjeta genérica con título opcional.
#[component]
pub fn Card(#[prop(into, optional)] title: Option<String>, children: Children) -> impl IntoView {
    view! {
        <section class="card">
            {title.map(|t: String| view! { <h2 class="card-title">{t}</h2> })}
            {children()}
        </section>
    }
}

/// Etiqueta de estado con variante de color (`ok`, `warn`, `error`, `info`, `muted`).
#[component]
pub fn Badge(#[prop(into)] text: String, #[prop(into)] variant: String) -> impl IntoView {
    view! { <span class=format!("badge badge-{variant}")>{text}</span> }
}

/// Mensaje de error estándar.
#[component]
pub fn ErrorText(#[prop(into)] message: String) -> impl IntoView {
    view! { <p class="error-text">{message}</p> }
}
