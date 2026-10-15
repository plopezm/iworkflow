use leptos::prelude::*;

/// Campo de texto de una línea enlazado a una señal.
#[component]
pub fn TextField(
    #[prop(into)] label: String,
    value: RwSignal<String>,
    #[prop(into, optional)] placeholder: String,
    #[prop(optional)] required: bool,
    #[prop(into, optional)] name: Option<String>,
) -> impl IntoView {
    view! {
        <label class="stack gap-1.5">
            <span class="text-sm font-medium text-fg">{label}</span>
            <input
                type="text"
                name=name
                class="input"
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
    /// Variante amplia: texto mayor y altura mínima generosa para aprovechar el espacio.
    #[prop(optional)]
    large: bool,
    /// Manejador opcional de pegado (p. ej. para adjuntar imágenes del portapapeles).
    #[prop(optional)]
    on_paste: Option<Callback<leptos::ev::ClipboardEvent>>,
) -> impl IntoView {
    let class: &str = if large {
        "input input-lg min-h-[50vh] resize-y"
    } else {
        "input min-h-24"
    };
    view! {
        <label class="stack gap-1.5">
            <span class="text-sm font-medium text-fg">{label}</span>
            <textarea
                class=class
                placeholder=placeholder
                required=required
                prop:value=move || value.get()
                on:input=move |ev| value.set(event_target_value(&ev))
                on:paste=move |ev| {
                    if let Some(handler) = on_paste {
                        handler.run(ev);
                    }
                }
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
        <label class="stack gap-1.5">
            <span class="text-sm font-medium text-fg">{label}</span>
            <select
                class="input"
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
            Some(Err(e)) => Some(view! { <p class="text-sm text-error">{e.to_string()}</p> }),
            _ => None,
        })
    }
}

/// Campo numérico opcional con unidad (p. ej. "MiB"), controlado por un valor externo:
/// notifica el texto al confirmar el cambio y deja que el llamador decida qué hacer con él.
/// Vacío significa "sin valor".
#[component]
pub fn NumberInput(
    #[prop(into)] label: String,
    #[prop(into)] value: String,
    #[prop(into)] unit: String,
    #[prop(into, optional)] placeholder: String,
    on_change: Callback<String>,
    #[prop(into, default = Signal::stored(false))] disabled: Signal<bool>,
) -> impl IntoView {
    view! {
        <label class="select-pill">
            <span class="text-[11px] text-muted">{label}</span>
            <input
                type="number"
                min="1"
                class="select-pill-input w-20"
                placeholder=placeholder
                prop:value=value
                disabled=move || disabled.get()
                on:change=move |ev| on_change.run(event_target_value(&ev))
            />
            <span class="text-[11px] text-muted">{unit}</span>
        </label>
    }
}
