//! Thin wrapper over the Web Notifications API. Real behavior only under `hydrate`;
//! on the server every call is a no-op and permission is `Unsupported`.

/// Notification permission as seen by the page.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NotificationPermission {
    /// Not asked yet; a user gesture may request it.
    Default,
    Granted,
    Denied,
    /// No Notification API (server render or unsupported browser).
    Unsupported,
}

#[cfg(feature = "hydrate")]
impl From<web_sys::NotificationPermission> for NotificationPermission {
    fn from(value: web_sys::NotificationPermission) -> Self {
        match value {
            web_sys::NotificationPermission::Granted => NotificationPermission::Granted,
            web_sys::NotificationPermission::Denied => NotificationPermission::Denied,
            _ => NotificationPermission::Default,
        }
    }
}

/// Whether the notification API exists in this browser.
#[cfg(feature = "hydrate")]
fn is_supported() -> bool {
    web_sys::window()
        .and_then(|window: web_sys::Window| js_sys_has(&window, "Notification"))
        .unwrap_or(false)
}

#[cfg(feature = "hydrate")]
fn js_sys_has(window: &web_sys::Window, key: &str) -> Option<bool> {
    use wasm_bindgen::JsValue;
    let value: Result<JsValue, JsValue> =
        js_sys::Reflect::get(window.as_ref(), &JsValue::from_str(key));
    value.ok().map(|v: JsValue| !v.is_undefined())
}

/// Current permission state.
pub fn current_permission() -> NotificationPermission {
    #[cfg(feature = "hydrate")]
    {
        if is_supported() {
            web_sys::Notification::permission().into()
        } else {
            NotificationPermission::Unsupported
        }
    }
    #[cfg(not(feature = "hydrate"))]
    {
        NotificationPermission::Unsupported
    }
}

/// Asks the user for permission (call from a user gesture) and reports the outcome.
pub fn request_permission(on_done: impl Fn(NotificationPermission) + 'static) {
    #[cfg(feature = "hydrate")]
    {
        use std::cell::RefCell;
        use std::rc::Rc;

        /// Permission callback shared with the browser closure; `take()` makes it fire once.
        type SharedDone = Rc<RefCell<Option<Box<dyn Fn(NotificationPermission)>>>>;

        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;

        if !is_supported() {
            on_done(NotificationPermission::Unsupported);
            return;
        }
        // Shared so the error path can still report; taken on first use so it fires once.
        let on_done: SharedDone = Rc::new(RefCell::new(Some(Box::new(on_done))));
        let callback_done: SharedDone = Rc::clone(&on_done);
        let callback: Closure<dyn FnMut(web_sys::NotificationPermission)> =
            Closure::once(move |_: web_sys::NotificationPermission| {
                if let Some(done) = callback_done.take() {
                    done(current_permission());
                }
            });
        if web_sys::Notification::request_permission_with_permission_callback(
            callback.as_ref().unchecked_ref(),
        )
        .is_err()
        {
            leptos::logging::error!("no se pudo solicitar el permiso de notificaciones");
            // The browser never got the callback, so report the outcome ourselves and
            // let the closure drop instead of leaking it.
            if let Some(done) = on_done.take() {
                done(current_permission());
            }
            return;
        }
        // The browser keeps the callback until it fires once.
        callback.forget();
    }
    #[cfg(not(feature = "hydrate"))]
    {
        on_done(NotificationPermission::Unsupported);
    }
}

/// The user is not looking at the page: tab hidden, window minimized, or focus elsewhere
/// (e.g. another window covering the browser, where `document.hidden` stays `false`).
#[cfg_attr(not(any(feature = "hydrate", test)), allow(dead_code))]
fn is_inactive(hidden: bool, has_focus: bool) -> bool {
    hidden || !has_focus
}

/// Whether the page is currently inactive (see [`is_inactive`]). Without a window or
/// document (server render) it reports `false`, so nothing is notified.
pub fn is_page_inactive() -> bool {
    #[cfg(feature = "hydrate")]
    {
        web_sys::window()
            .and_then(|window: web_sys::Window| window.document())
            .map(|document: web_sys::Document| {
                // A failed focus query counts as focused: err on the side of not notifying.
                let has_focus: bool = document.has_focus().unwrap_or(true);
                is_inactive(document.hidden(), has_focus)
            })
            .unwrap_or(false)
    }
    #[cfg(not(feature = "hydrate"))]
    {
        false
    }
}

/// Shows an OS notification; clicking it focuses the window and runs `on_click`.
/// `tag` replaces any earlier notification with the same tag. Does nothing unless
/// permission is granted.
pub fn show_notification(title: &str, body: &str, tag: &str, on_click: impl FnOnce() + 'static) {
    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;

        if current_permission() != NotificationPermission::Granted {
            return;
        }
        let options: web_sys::NotificationOptions = web_sys::NotificationOptions::new();
        options.set_body(body);
        options.set_tag(tag);
        // Same tag replaces the previous notification of the task; still alert for it.
        options.set_renotify(true);
        let Ok(notification) = web_sys::Notification::new_with_options(title, &options) else {
            leptos::logging::error!("no se pudo mostrar la notificación");
            return;
        };
        let handler: Closure<dyn FnMut(web_sys::Event)> =
            Closure::once(move |_: web_sys::Event| {
                if let Some(window) = web_sys::window()
                    && window.focus().is_err()
                {
                    leptos::logging::error!("no se pudo enfocar la ventana");
                }
                on_click();
            });
        notification.set_onclick(Some(handler.as_ref().unchecked_ref()));
        // The notification owns the handler. `Closure::once` frees itself after the click;
        // if the notification is dismissed unclicked, the leak is one small closure per
        // notification, bounded by the number of task transitions.
        handler.forget();
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _args: (&str, &str, &str) = (title, body, tag);
        drop(on_click);
    }
}

#[cfg(all(test, not(feature = "hydrate")))]
mod tests {
    use super::*;

    #[test]
    fn server_side_permission_is_unsupported() {
        assert_eq!(current_permission(), NotificationPermission::Unsupported);
    }

    #[test]
    fn page_is_inactive_when_hidden_or_unfocused() {
        assert!(!is_inactive(false, true));
        assert!(is_inactive(true, true));
        assert!(is_inactive(false, false));
        assert!(is_inactive(true, false));
    }
}
