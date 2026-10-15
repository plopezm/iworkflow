//! Visor y editor de código con resaltado de sintaxis (highlight.js, cargado bajo demanda).

use leptos::html;
use leptos::prelude::*;

#[cfg(feature = "hydrate")]
mod js {
    use wasm_bindgen::JsValue;
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(inline_js = r#"
const ASSETS = ['/vendor/highlightjs/highlight.min.js', '/code-view.js'];
let loading = null;

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.dataset.iwfSrc = src;
    script.src = src;
    script.onload = resolve;
    script.onerror = () => reject(new Error('no se pudo cargar ' + src));
    document.head.appendChild(script);
  });
}

function ready() {
  if (window.iwfCode) return Promise.resolve();
  if (!loading) {
    loading = (async () => {
      for (const src of ASSETS) await loadScript(src);
    })().catch((e) => {
      loading = null;
      throw e;
    });
  }
  return loading;
}

export function iwf_code_render(el, text, filename) {
  const draw = () => window.iwfCode.render(el, text, filename);
  if (window.iwfCode) return draw();
  ready().then(draw).catch((e) => {
    console.error(e);
    el.textContent = text;
  });
}

export function iwf_code_attach_editor(textarea, layer, gutter) {
  const handle = { detach: null, closed: false };
  ready().then(() => {
    if (!handle.closed) handle.detach = window.iwfCode.attachEditor(textarea, layer, gutter);
  }).catch((e) => console.error(e));
  return handle;
}

export function iwf_code_detach_editor(handle) {
  handle.closed = true;
  if (handle.detach) {
    handle.detach();
    handle.detach = null;
  }
}
"#)]
    extern "C" {
        pub fn iwf_code_render(el: &JsValue, text: &str, filename: &str);
        pub fn iwf_code_attach_editor(
            textarea: &JsValue,
            layer: &JsValue,
            gutter: &JsValue,
        ) -> JsValue;
        pub fn iwf_code_detach_editor(handle: &JsValue);
    }
}

/// Texto de la columna de números de línea para `text` (pura): `1\n2\n…`, al menos una línea.
pub fn line_numbers(text: &str) -> String {
    let lines: usize = text.split('\n').count();
    (1..=lines)
        .map(|n: usize| n.to_string())
        .collect::<Vec<String>>()
        .join("\n")
}

/// Visor de solo lectura con colores de sintaxis y números de línea.
#[component]
pub fn CodeView(
    #[prop(into)] text: Signal<String>,
    #[prop(into)] filename: String,
) -> impl IntoView {
    let code: NodeRef<html::Code> = NodeRef::new();
    let gutter: Memo<String> = Memo::new(move |_| text.with(|t: &String| line_numbers(t)));

    #[cfg(feature = "hydrate")]
    Effect::new(move |_| {
        use wasm_bindgen::JsCast;

        let content: String = text.get();
        if let Some(el) = code.get() {
            js::iwf_code_render(el.unchecked_ref(), &content, &filename);
        }
    });
    #[cfg(not(feature = "hydrate"))]
    let _ = &filename;

    view! {
        <div class="code-box code-box-view">
            <pre class="code-gutter" aria-hidden="true">
                {move || gutter.get()}
            </pre>
            <pre class="code-text">
                <code node_ref=code></code>
            </pre>
        </div>
    }
}

/// Editor con colores de sintaxis: un `textarea` transparente sobre una capa resaltada.
#[component]
pub fn CodeEditor(
    /// Contenido actual; el editor lo escribe al teclear.
    value: RwSignal<String>,
    #[prop(into)] filename: String,
    #[prop(optional, into)] disabled: Signal<bool>,
    /// Se invoca con Ctrl/Cmd+S.
    #[prop(optional, into)]
    on_save: Option<Callback<()>>,
) -> impl IntoView {
    let textarea: NodeRef<html::Textarea> = NodeRef::new();
    let layer: NodeRef<html::Code> = NodeRef::new();
    let layer_pre: NodeRef<html::Pre> = NodeRef::new();
    let gutter_pre: NodeRef<html::Pre> = NodeRef::new();
    let gutter: Memo<String> = Memo::new(move |_| value.with(|t: &String| line_numbers(t)));

    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::{JsCast, JsValue};

        let render_name: String = filename.clone();
        Effect::new(move |_| {
            let content: String = value.get();
            if let Some(el) = layer.get() {
                js::iwf_code_render(el.unchecked_ref(), &content, &render_name);
            }
        });

        let handle: StoredValue<Option<JsValue>, LocalStorage> = StoredValue::new_local(None);
        Effect::new(move |_| {
            if handle.with_value(|h: &Option<JsValue>| h.is_some()) {
                return;
            }
            if let (Some(t), Some(l), Some(g)) = (textarea.get(), layer_pre.get(), gutter_pre.get())
            {
                let attached: JsValue = js::iwf_code_attach_editor(
                    t.unchecked_ref(),
                    l.unchecked_ref(),
                    g.unchecked_ref(),
                );
                handle.set_value(Some(attached));
            }
        });
        on_cleanup(move || {
            if let Some(h) = handle.try_get_value().flatten() {
                js::iwf_code_detach_editor(&h);
            }
        });
    }
    #[cfg(not(feature = "hydrate"))]
    let _ = &filename;

    view! {
        <div class="code-box code-box-edit">
            <pre class="code-gutter code-gutter-edit" aria-hidden="true" node_ref=gutter_pre>
                {move || gutter.get()}
            </pre>
            <div class="code-stack">
                <pre class="code-text code-layer" aria-hidden="true" node_ref=layer_pre>
                    <code node_ref=layer></code>
                </pre>
                <textarea
                    class="code-input"
                    node_ref=textarea
                    spellcheck="false"
                    autocapitalize="off"
                    autocomplete="off"
                    wrap="off"
                    aria-label="Contenido del fichero"
                    prop:value=move || value.get()
                    prop:disabled=move || disabled.get()
                    on:input=move |ev| value.set(event_target_value(&ev))
                    on:keydown=move |ev: leptos::ev::KeyboardEvent| {
                        if (ev.ctrl_key() || ev.meta_key()) && ev.key().eq_ignore_ascii_case("s") {
                            ev.prevent_default();
                            if let Some(save) = on_save {
                                save.run(());
                            }
                        }
                    }
                ></textarea>
            </div>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn numbers_cover_every_line() {
        assert_eq!(line_numbers(""), "1");
        assert_eq!(line_numbers("a"), "1");
        assert_eq!(line_numbers("a\nb\n"), "1\n2\n3");
    }
}
