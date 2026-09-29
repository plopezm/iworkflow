use leptos::prelude::*;

/// One `<option>`, optionally grouped under an `<optgroup>` by `group`.
#[derive(Debug, Clone)]
pub struct SelectOption {
    pub value: String,
    pub label: String,
    pub group: Option<String>,
}

impl SelectOption {
    /// Opción sin agrupar.
    pub fn new(value: impl Into<String>, label: impl Into<String>) -> Self {
        Self {
            value: value.into(),
            label: label.into(),
            group: None,
        }
    }

    /// Opción agrupada bajo un `<optgroup label=group>`.
    pub fn grouped(
        group: impl Into<String>,
        value: impl Into<String>,
        label: impl Into<String>,
    ) -> Self {
        Self {
            value: value.into(),
            label: label.into(),
            group: Some(group.into()),
        }
    }
}

/// Agrupa las opciones por `group`, preservando el orden de primera aparición de cada
/// grupo (y de las opciones dentro de él) tal como llegan en `options`.
fn group_options(options: Vec<SelectOption>) -> Vec<(Option<String>, Vec<SelectOption>)> {
    let mut groups: Vec<(Option<String>, Vec<SelectOption>)> = Vec::new();
    for option in options {
        match groups.iter_mut().find(|(group, _)| *group == option.group) {
            Some((_, items)) => items.push(option),
            None => groups.push((option.group.clone(), vec![option])),
        }
    }
    groups
}

/// Desplegable compacto en forma de píldora (mismo lenguaje visual que `.nav-pill`/`.chip`),
/// controlado por un valor externo (no una señal enlazada): notifica cada cambio vía
/// `on_change` en lugar de escribir en una señal propia, para que el llamador decida qué
/// hacer con la selección (p. ej. despachar una acción). Incluye siempre una primera opción
/// vacía (`default_label`, p. ej. "Por defecto"). `label` no se muestra (evita el texto
/// apilado que desbordaba la barra) pero sigue siendo el nombre accesible del control vía
/// asociación implícita `<label>`. `icon` es un prefijo visual opcional (p. ej. un `<svg>`).
#[component]
pub fn OptionSelect(
    #[prop(into)] label: String,
    #[prop(into)] value: String,
    options: Vec<SelectOption>,
    #[prop(into)] default_label: String,
    on_change: Callback<String>,
    #[prop(into)] disabled: Signal<bool>,
    #[prop(optional, into)] icon: Option<ViewFn>,
) -> impl IntoView {
    let is_default: bool = value.is_empty();
    let current_label: String = options
        .iter()
        .find(|option: &&SelectOption| option.value == value)
        .map(|option: &SelectOption| option.label.clone())
        .unwrap_or_else(|| default_label.clone());

    view! {
        <label class="select-pill" title=current_label>
            {icon
                .map(|icon: ViewFn| {
                    view! {
                        <span class="select-pill-icon" aria-hidden="true">
                            {icon.run()}
                        </span>
                    }
                })}
            <span class="sr-only">{label}</span>
            <select
                class="select-pill-input"
                disabled=move || disabled.get()
                on:change=move |ev| on_change.run(event_target_value(&ev))
            >
                <option value="" selected=is_default>
                    {default_label}
                </option>
                {group_options(options)
                    .into_iter()
                    .map(|(group, items): (Option<String>, Vec<SelectOption>)| {
                        let items_view = items
                            .into_iter()
                            .map(|option: SelectOption| {
                                let is_selected: bool = option.value == value;
                                view! {
                                    <option value=option.value.clone() selected=is_selected>
                                        {option.label}
                                    </option>
                                }
                            })
                            .collect_view();
                        match group {
                            Some(name) => {
                                view! { <optgroup label=name>{items_view}</optgroup> }.into_any()
                            }
                            None => items_view.into_any(),
                        }
                    })
                    .collect_view()}
            </select>
            <svg
                class="select-pill-chevron"
                viewBox="0 0 20 20"
                fill="none"
                aria-hidden="true"
            >
                <path
                    d="M5.5 8L10 12.5L14.5 8"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
        </label>
    }
}
