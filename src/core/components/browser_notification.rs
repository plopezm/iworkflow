//! Thin wrapper over the Web Notifications API. Real behavior only under `hydrate`;
//! on the server every call is a no-op.

/// Notification permission as seen by the page.
#[cfg_attr(not(any(feature = "hydrate", test)), allow(dead_code))]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum NotificationPermission {
    /// Not asked yet; the browser may prompt the user.
    Default,
    Granted,
    Denied,
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

/// The browser has not asked the user yet, so its native prompt can still be shown.
#[cfg_attr(not(any(feature = "hydrate", test)), allow(dead_code))]
fn should_request(permission: NotificationPermission) -> bool {
    permission == NotificationPermission::Default
}

/// Notifications may only be shown once the user granted permission.
#[cfg_attr(not(any(feature = "hydrate", test)), allow(dead_code))]
fn can_show(permission: NotificationPermission) -> bool {
    permission == NotificationPermission::Granted
}

/// Whether the notification API exists in this browser.
#[cfg(feature = "hydrate")]
fn is_supported() -> bool {
    use wasm_bindgen::JsValue;

    web_sys::window()
        .map(|window: web_sys::Window| {
            let value: Result<JsValue, JsValue> =
                js_sys::Reflect::get(window.as_ref(), &JsValue::from_str("Notification"));
            value.is_ok_and(|v: JsValue| !v.is_undefined())
        })
        .unwrap_or(false)
}

/// Current permission, `None` when the browser has no Notification API.
#[cfg(feature = "hydrate")]
fn current_permission() -> Option<NotificationPermission> {
    is_supported().then(|| web_sys::Notification::permission().into())
}

/// Triggers the browser's native permission prompt on the user's first click on the page
/// (browsers only honor the request from a user gesture). Does nothing if permission was
/// already decided or the API is missing.
pub fn request_permission_on_first_interaction() {
    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;

        if !current_permission().is_some_and(should_request) {
            return;
        }
        let Some(document) = web_sys::window().and_then(|w: web_sys::Window| w.document()) else {
            return;
        };
        let handler: Closure<dyn FnMut(web_sys::Event)> =
            Closure::once(move |_: web_sys::Event| {
                if web_sys::Notification::request_permission().is_err() {
                    leptos::logging::error!("no se pudo solicitar el permiso de notificaciones");
                }
            });
        let options: web_sys::AddEventListenerOptions = web_sys::AddEventListenerOptions::new();
        options.set_once(true);
        if document
            .add_event_listener_with_callback_and_add_event_listener_options(
                "click",
                handler.as_ref().unchecked_ref(),
                &options,
            )
            .is_err()
        {
            leptos::logging::error!("no se pudo registrar el listener de notificaciones");
            return;
        }
        // The one-shot listener owns the handler until the first click.
        handler.forget();
    }
}

/// Shows a native notification; clicking it focuses the window. `tag` replaces any earlier
/// notification with the same tag. Does nothing unless permission is granted.
pub fn show_notification(title: &str, body: &str, tag: &str) {
    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;

        if !current_permission().is_some_and(can_show) {
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
            });
        notification.set_onclick(Some(handler.as_ref().unchecked_ref()));
        // The notification owns the handler; `Closure::once` frees itself after the click.
        handler.forget();
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _args: (&str, &str, &str) = (title, body, tag);
    }
}

#[cfg(all(test, not(feature = "hydrate")))]
mod tests {
    use super::*;

    #[test]
    fn only_undecided_permission_triggers_the_prompt() {
        assert!(should_request(NotificationPermission::Default));
        assert!(!should_request(NotificationPermission::Granted));
        assert!(!should_request(NotificationPermission::Denied));
    }

    #[test]
    fn only_granted_permission_allows_showing() {
        assert!(can_show(NotificationPermission::Granted));
        assert!(!can_show(NotificationPermission::Default));
        assert!(!can_show(NotificationPermission::Denied));
    }

    #[test]
    fn server_side_calls_are_noops() {
        request_permission_on_first_interaction();
        show_notification("t", "b", "tag");
    }
}
