use leptos::prelude::*;

use crate::core::browser;
use crate::core::i18n::Msg;

const THEME_LIGHT: Msg = Msg {
    en: "☀ Light",
    es: "☀ Claro",
    zh: "☀ 浅色",
    de: "☀ Hell",
    fr: "☀ Clair",
};
const THEME_DARK: Msg = Msg {
    en: "☾ Dark",
    es: "☾ Oscuro",
    zh: "☾ 深色",
    de: "☾ Dunkel",
    fr: "☾ Sombre",
};
const THEME_AUTO: Msg = Msg {
    en: "Auto",
    es: "Auto",
    zh: "自动",
    de: "Auto",
    fr: "Auto",
};
const THEME: Msg = Msg {
    en: "Theme",
    es: "Tema",
    zh: "主题",
    de: "Design",
    fr: "Thème",
};

/// Script bloqueante del `<head>`: fija `data-theme` antes del primer pintado.
pub const THEME_INIT_SCRIPT: &str = "(function(){try{var m=localStorage.getItem('pi-theme');\
if(m!=='light'&&m!=='dark'){m=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}\
document.documentElement.setAttribute('data-theme',m);}catch(e){}})();";

/// Preferencia de tema del usuario.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ThemeMode {
    Light,
    Dark,
    #[default]
    System,
}

impl ThemeMode {
    pub fn as_str(self) -> &'static str {
        match self {
            ThemeMode::Light => "light",
            ThemeMode::Dark => "dark",
            ThemeMode::System => "system",
        }
    }

    /// Valores desconocidos caen en `System`.
    pub fn parse(value: &str) -> ThemeMode {
        match value {
            "light" => ThemeMode::Light,
            "dark" => ThemeMode::Dark,
            _ => ThemeMode::System,
        }
    }

    /// Tema efectivo (`"light"` o `"dark"`) dado el esquema preferido por el sistema.
    pub fn resolve(self, system_prefers_dark: bool) -> &'static str {
        match (self, system_prefers_dark) {
            (ThemeMode::Dark, _) | (ThemeMode::System, true) => "dark",
            (ThemeMode::Light, _) | (ThemeMode::System, false) => "light",
        }
    }
}

/// Registra el modo de tema en el contexto y lo sincroniza con `localStorage` tras la hidratación.
pub fn provide_theme_mode() {
    let mode: RwSignal<ThemeMode> = RwSignal::new(ThemeMode::default());
    provide_context(mode);
    Effect::new(move |_| {
        if let Some(stored) = browser::read_stored_theme() {
            mode.set(ThemeMode::parse(&stored));
        }
    });
}

fn choose_theme(mode: RwSignal<ThemeMode>, next: ThemeMode) {
    mode.set(next);
    browser::write_stored_theme(next.as_str());
    browser::set_document_theme(next.resolve(browser::system_prefers_dark()));
}

const OPTIONS: [(ThemeMode, Msg); 3] = [
    (ThemeMode::Light, THEME_LIGHT),
    (ThemeMode::Dark, THEME_DARK),
    (ThemeMode::System, THEME_AUTO),
];

/// Selector claro / oscuro / sistema. Requiere `provide_theme_mode()` en un ancestro.
#[component]
pub fn ThemeToggle() -> impl IntoView {
    let mode: RwSignal<ThemeMode> = expect_context::<RwSignal<ThemeMode>>();
    view! {
        <div class="flex gap-1.5" role="group" aria-label=THEME.get()>
            {OPTIONS
                .into_iter()
                .map(|(option, label): (ThemeMode, Msg)| {
                    view! {
                        <button
                            class=move || {
                                if mode.get() == option {
                                    "chip chip-selected flex-1 justify-center"
                                } else {
                                    "chip flex-1 justify-center"
                                }
                            }
                            aria-pressed=move || (mode.get() == option).to_string()
                            on:click=move |_| choose_theme(mode, option)
                        >
                            {label.get()}
                        </button>
                    }
                })
                .collect_view()}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn mode_round_trips_through_its_string_form() {
        for mode in [ThemeMode::Light, ThemeMode::Dark, ThemeMode::System] {
            assert_eq!(ThemeMode::parse(mode.as_str()), mode);
        }
        assert_eq!(ThemeMode::parse("garbage"), ThemeMode::System);
    }

    #[test]
    fn resolve_follows_system_only_in_system_mode() {
        assert_eq!(ThemeMode::Light.resolve(true), "light");
        assert_eq!(ThemeMode::Dark.resolve(false), "dark");
        assert_eq!(ThemeMode::System.resolve(true), "dark");
        assert_eq!(ThemeMode::System.resolve(false), "light");
    }

    #[test]
    fn init_script_uses_the_storage_key() {
        assert!(THEME_INIT_SCRIPT.contains(browser::THEME_STORAGE_KEY));
    }
}
