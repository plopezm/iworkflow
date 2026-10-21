//! Panel de pestaña que conserva su estado: se monta la primera vez que se muestra y después
//! solo se oculta, en vez de desmontarse como hace `<Show>`.

use leptos::prelude::*;

/// Clase del contenedor según esté activo o no (pure).
pub fn pane_class(active: bool) -> &'static str {
    if active { "contents" } else { "hidden" }
}

/// `true` desde la primera activación y para siempre: notifica una sola vez (pure).
pub fn seen_once(active: Signal<bool>) -> Memo<bool> {
    // Un `Memo` solo notifica cuando cambia: reactivar la pestaña no remonta el contenido.
    Memo::new(move |prev: Option<&bool>| prev.copied().unwrap_or(false) || active.get())
}

/// Monta `children` en la primera activación y los mantiene montados (ocultos) después.
#[component]
pub fn KeepAlive(#[prop(into)] active: Signal<bool>, children: ChildrenFn) -> impl IntoView {
    let seen: Memo<bool> = seen_once(active);
    view! {
        <div class=move || pane_class(active.get())>
            {move || seen.get().then(|| untrack(|| children()))}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn inactive_pane_is_hidden_and_active_pane_is_shown() {
        assert_eq!(pane_class(true), "contents");
        assert_eq!(pane_class(false), "hidden");
    }

    /// Lets the pending reactive effects run.
    async fn settle() {
        for _ in 0..5 {
            tokio::task::yield_now().await;
        }
    }

    #[tokio::test]
    async fn reactivating_does_not_notify_again() {
        use any_spawner::Executor;
        use std::sync::Arc;
        use std::sync::atomic::{AtomicUsize, Ordering};
        let _init: Result<(), _> = Executor::init_tokio();
        tokio::task::LocalSet::new()
            .run_until(async move {
                let owner: Owner = Owner::new();
                owner.set();
                let active: RwSignal<bool> = RwSignal::new(false);
                let seen: Memo<bool> = seen_once(active.into());
                let runs: Arc<AtomicUsize> = Arc::new(AtomicUsize::new(0));
                let counter: Arc<AtomicUsize> = Arc::clone(&runs);
                Effect::new_isomorphic(move |_| {
                    if seen.get() {
                        counter.fetch_add(1, Ordering::SeqCst);
                    }
                });
                settle().await;
                assert_eq!(runs.load(Ordering::SeqCst), 0);
                for value in [true, false, true] {
                    active.set(value);
                    settle().await;
                }
                assert_eq!(runs.load(Ordering::SeqCst), 1);
            })
            .await;
    }
}
