//! Señal global de "los datos compartidos cambiaron" (sin dominio de ningún feature).
//!
//! Con pestañas, la pantalla que muta datos (p. ej. registrar un workspace) sigue visible
//! junto a otras que los listan, así que ya no basta recargar al cambiar de ruta.

use leptos::prelude::*;

/// Contador de cambios; los listados lo leen en su clave de recarga.
#[derive(Clone, Copy)]
pub struct DataRefresh(RwSignal<u32>);

impl DataRefresh {
    /// Crea el contador y lo provee por contexto.
    pub fn provide() -> Self {
        let refresh: Self = Self(RwSignal::new(0));
        provide_context(refresh);
        refresh
    }

    /// Valor actual (se suscribe).
    pub fn epoch(&self) -> u32 {
        self.0.get()
    }

    pub fn bump(&self) {
        self.0.update(|n: &mut u32| *n = n.wrapping_add(1));
    }
}

/// Lectura sin contexto: `0` si no hay `DataRefresh` (tests, SSR de piezas sueltas).
pub fn current_epoch() -> u32 {
    use_context::<DataRefresh>().map_or(0, |r: DataRefresh| r.epoch())
}

/// Avisa de un cambio cada vez que `version` (versión de una acción) avanza.
pub fn bump_when_changes(version: RwSignal<usize>) {
    let Some(refresh) = use_context::<DataRefresh>() else {
        return;
    };
    Effect::new(move |previous: Option<usize>| {
        let now: usize = version.get();
        if previous.is_some_and(|p: usize| p != now) {
            refresh.bump();
        }
        now
    });
}
