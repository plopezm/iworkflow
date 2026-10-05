use leptos::html;
use leptos::prelude::*;

#[cfg(feature = "hydrate")]
mod js {
    use wasm_bindgen::JsValue;
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(inline_js = r#"
const ASSETS = ['/vendor/xterm/xterm.js', '/vendor/xterm/addon-fit.js', '/terminal.js'];
const CSS = '/vendor/xterm/xterm.css';

function loadScript(src) {
  const existing = document.querySelector('script[data-iwf-src="' + src + '"]');
  if (existing) {
    return existing.__iwfLoaded;
  }
  const script = document.createElement('script');
  script.dataset.iwfSrc = src;
  script.src = src;
  const loaded = new Promise((resolve, reject) => {
    script.onload = resolve;
    script.onerror = () => reject(new Error('no se pudo cargar ' + src));
  });
  script.__iwfLoaded = loaded;
  document.head.appendChild(script);
  return loaded;
}

function loadCss(href) {
  if (document.querySelector('link[data-iwf-src="' + href + '"]')) return;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = href;
  link.dataset.iwfSrc = href;
  document.head.appendChild(link);
}

export function iwf_terminal_open(el, workspaceId) {
  const handle = { inner: null, closed: false };
  loadCss(CSS);
  (async () => {
    for (const src of ASSETS) await loadScript(src);
    if (!handle.closed) handle.inner = window.iwfTerminal.open(el, workspaceId);
  })().catch((e) => console.error(e));
  return handle;
}

export function iwf_terminal_close(handle) {
  handle.closed = true;
  if (handle.inner) {
    window.iwfTerminal.close(handle.inner);
    handle.inner = null;
  }
}
"#)]
    extern "C" {
        pub fn iwf_terminal_open(el: &JsValue, workspace_id: &str) -> JsValue;
        pub fn iwf_terminal_close(handle: &JsValue);
    }
}

/// Monta un xterm.js conectado a la sesión de terminal del workspace. Solo actúa en el navegador.
#[component]
pub fn TerminalView(#[prop(into)] workspace_id: String) -> impl IntoView {
    let container: NodeRef<html::Div> = NodeRef::new();

    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::JsValue;

        let handle: StoredValue<Option<JsValue>, LocalStorage> = StoredValue::new_local(None);
        // Solo monta el DOM: la conexión al backend la gestiona el WebSocket de terminal.js.
        Effect::new(move |_| {
            if let Some(el) = container.get()
                && handle.with_value(Option::is_none)
            {
                let opened: JsValue = js::iwf_terminal_open(&JsValue::from(el), &workspace_id);
                handle.set_value(Some(opened));
            }
        });
        on_cleanup(move || {
            if let Some(opened) = handle.try_update_value(Option::take).flatten() {
                js::iwf_terminal_close(&opened);
            }
        });
    }
    #[cfg(not(feature = "hydrate"))]
    let _workspace_id: String = workspace_id;

    view! { <div node_ref=container class="min-h-0 flex-1 overflow-hidden rounded-lg border border-border bg-surface p-2"></div> }
}
