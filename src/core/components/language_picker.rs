use leptos::prelude::*;

use crate::core::components::layout::Card;
use crate::core::i18n::{Locale, LocaleState, Msg, choose_locale, current_locale};

const CARD_TITLE: Msg = Msg {
    en: "Language",
    es: "Idioma",
    zh: "语言",
    de: "Sprache",
    fr: "Langue",
};

const CARD_HELP: Msg = Msg {
    en: "Language of the interface. The page reloads to apply the change.",
    es: "Idioma de la interfaz. La página se recarga para aplicar el cambio.",
    zh: "界面语言。更改后页面将重新加载。",
    de: "Sprache der Oberfläche. Die Seite wird neu geladen, um die Änderung zu übernehmen.",
    fr: "Langue de l'interface. La page se recharge pour appliquer le changement.",
};

const PROMPT_TITLE: Msg = Msg {
    en: "Choose your language",
    es: "Elige tu idioma",
    zh: "选择你的语言",
    de: "Wähle deine Sprache",
    fr: "Choisissez votre langue",
};

const PROMPT_HELP: Msg = Msg {
    en: "You can change it later in Settings.",
    es: "Puedes cambiarlo más tarde en Configuración.",
    zh: "之后可以在“设置”中更改。",
    de: "Du kannst dies später in den Einstellungen ändern.",
    fr: "Vous pourrez le modifier plus tard dans les Paramètres.",
};

/// Settings card with one button per supported language; picking one reloads the page.
#[component]
pub fn LanguageCard() -> impl IntoView {
    let active: Locale = current_locale();
    view! {
        <Card title=CARD_TITLE.get()>
            <p class="muted text-sm">{CARD_HELP.get()}</p>
            <div class="row flex-wrap gap-2" role="group" aria-label=CARD_TITLE.get()>
                {Locale::ALL
                    .into_iter()
                    .map(|locale: Locale| {
                        view! {
                            <button
                                class=if locale == active {
                                    "chip chip-selected"
                                } else {
                                    "chip"
                                }
                                aria-pressed=(locale == active).to_string()
                                lang=locale.code()
                                on:click=move |_| {
                                    if locale != active {
                                        choose_locale(locale);
                                    }
                                }
                            >
                                {locale.native_name()}
                            </button>
                        }
                    })
                    .collect_view()}
            </div>
        </Card>
    }
}

/// First-run dialog that asks for the language until the user has picked one.
#[component]
pub fn LanguagePrompt() -> impl IntoView {
    let state: LocaleState = expect_context::<LocaleState>();
    let other_languages: String = Locale::ALL
        .into_iter()
        .filter(|locale: &Locale| *locale != Locale::En)
        .map(|locale: Locale| PROMPT_TITLE.in_locale(locale))
        .collect::<Vec<&str>>()
        .join(" · ");
    view! {
        <Show when=move || !state.chosen>
            <div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-md">
                <div
                    class="stack w-full rounded-3xl border border-border/60 bg-surface/95 p-6 animate-scale-in sm:max-w-md"
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="language-prompt-title"
                >
                    <h2 id="language-prompt-title" class="text-lg font-semibold text-fg">
                        {PROMPT_TITLE.in_locale(Locale::En)}
                    </h2>
                    <p class="muted text-sm">{other_languages.clone()}</p>
                    <div class="stack gap-2">
                        {Locale::ALL
                            .into_iter()
                            .map(|locale: Locale| {
                                view! {
                                    <button
                                        class="btn-secondary justify-center"
                                        lang=locale.code()
                                        on:click=move |_| choose_locale(locale)
                                    >
                                        {locale.native_name()}
                                    </button>
                                }
                            })
                            .collect_view()}
                    </div>
                    <p class="muted text-xs">{PROMPT_HELP.in_locale(Locale::En)}</p>
                </div>
            </div>
        </Show>
    }
}
