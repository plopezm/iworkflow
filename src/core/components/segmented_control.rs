use leptos::prelude::*;

/// Opción de un control segmentado: valor estable y etiqueta ya traducida.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SegmentOption {
    pub value: String,
    pub label: String,
}

/// Clase de un segmento según esté seleccionado.
pub fn segment_class(selected: bool) -> &'static str {
    if selected {
        "segment segment-selected"
    } else {
        "segment"
    }
}

/// Control segmentado (pestañas excluyentes). Genérico: las opciones y la etiqueta accesible
/// del grupo las aporta quien lo usa; la selección vive en una señal ajena.
#[component]
pub fn SegmentedControl(
    #[prop(into)] options: Signal<Vec<SegmentOption>>,
    #[prop(into)] selected: Signal<String>,
    #[prop(into)] aria_label: String,
    on_select: Callback<String>,
    #[prop(optional, into)] class: String,
) -> impl IntoView {
    view! {
        <div class=format!("segmented {class}") role="tablist" aria-label=aria_label>
            <For
                each=move || options.get()
                key=|o: &SegmentOption| format!("{}|{}", o.value, o.label)
                children=move |option: SegmentOption| {
                    let value: String = option.value.clone();
                    let check: String = option.value.clone();
                    let is_selected: Signal<bool> =
                        Signal::derive(move || selected.get() == check);
                    view! {
                        <button
                            type="button"
                            role="tab"
                            class=move || segment_class(is_selected.get())
                            aria-selected=move || is_selected.get().to_string()
                            on:click=move |_| on_select.run(value.clone())
                        >
                            {option.label}
                        </button>
                    }
                }
            />
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn selected_segment_is_marked() {
        assert_eq!(segment_class(false), "segment");
        assert_eq!(segment_class(true), "segment segment-selected");
    }
}
