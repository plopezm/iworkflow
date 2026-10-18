use leptos::ev::PointerEvent;
use leptos::prelude::*;

/// Etiqueta del nivel de zoom en porcentaje (`1.0` → `100%`).
pub fn zoom_percent_label(zoom: f64) -> String {
    format!("{}%", (zoom * 100.0).round() as i64)
}

/// Controles flotantes de un lienzo con zoom: acercar, alejar y nivel actual. Genérico:
/// los textos accesibles los aporta quien lo usa.
#[component]
pub fn ZoomControls(
    #[prop(into)] zoom: Signal<f64>,
    #[prop(into)] zoom_in_label: String,
    #[prop(into)] zoom_out_label: String,
    on_zoom_in: Callback<()>,
    on_zoom_out: Callback<()>,
) -> impl IntoView {
    view! {
        <div
            class="zoom-controls"
            on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
        >
            <button
                type="button"
                class="zoom-control"
                title=zoom_out_label.clone()
                aria-label=zoom_out_label
                on:click=move |_| on_zoom_out.run(())
            >
                "−"
            </button>
            <span class="zoom-readout" aria-live="polite">
                {move || zoom_percent_label(zoom.get())}
            </span>
            <button
                type="button"
                class="zoom-control"
                title=zoom_in_label.clone()
                aria-label=zoom_in_label
                on:click=move |_| on_zoom_in.run(())
            >
                "+"
            </button>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn zoom_is_shown_as_a_rounded_percentage() {
        assert_eq!(zoom_percent_label(1.0), "100%");
        assert_eq!(zoom_percent_label(0.4623), "46%");
        assert_eq!(zoom_percent_label(2.0), "200%");
    }
}
