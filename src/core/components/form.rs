use leptos::prelude::*;

/// Campo de texto de una línea enlazado a una señal.
#[component]
pub fn TextField(
    #[prop(into)] label: String,
    value: RwSignal<String>,
    #[prop(into, optional)] placeholder: String,
    #[prop(optional)] required: bool,
) -> impl IntoView {
    view! {
        <label class="stack">
            <span>{label}</span>
            <input
                type="text"
                placeholder=placeholder
                required=required
                prop:value=move || value.get()
                on:input=move |ev| value.set(event_target_value(&ev))
            />
        </label>
    }
}

/// Área de texto multilínea enlazada a una señal.
#[component]
pub fn TextArea(
    #[prop(into)] label: String,
    value: RwSignal<String>,
    #[prop(into, optional)] placeholder: String,
    #[prop(optional)] required: bool,
) -> impl IntoView {
    view! {
        <label class="stack">
            <span>{label}</span>
            <textarea
                placeholder=placeholder
                required=required
                prop:value=move || value.get()
                on:input=move |ev| value.set(event_target_value(&ev))
            ></textarea>
        </label>
    }
}

/// Opción de un desplegable: valor y texto visible.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SelectOption {
    pub value: String,
    pub label: String,
}

/// Desplegable enlazado a una señal. `placeholder` añade una opción vacía inicial.
#[component]
pub fn SelectField(
    #[prop(into)] label: String,
    value: RwSignal<String>,
    options: Vec<SelectOption>,
    #[prop(into, optional)] placeholder: Option<String>,
    #[prop(optional)] required: bool,
) -> impl IntoView {
    view! {
        <label class="stack">
            <span>{label}</span>
            <select
                required=required
                prop:value=move || value.get()
                on:change=move |ev| value.set(event_target_value(&ev))
            >
                {placeholder.map(|text: String| view! { <option value="">{text}</option> })}
                {options
                    .into_iter()
                    .map(|o: SelectOption| {
                        let selected: String = o.value.clone();
                        view! {
                            <option value=o.value selected=move || value.get() == selected>
                                {o.label}
                            </option>
                        }
                    })
                    .collect_view()}
            </select>
        </label>
    }
}

/// Muestra el error de la última ejecución de una acción, si lo hubo.
#[component]
pub fn ActionError<T>(result: Signal<Option<Result<T, ServerFnError>>>) -> impl IntoView
where
    T: Send + Sync + 'static,
{
    move || {
        result.with(|r: &Option<Result<T, ServerFnError>>| match r {
            Some(Err(e)) => Some(view! { <p class="error-text">{e.to_string()}</p> }),
            _ => None,
        })
    }
}
