use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Badge, Card, ErrorText};
use crate::core::i18n::Msg;
use crate::modules::preferences::infrastructure::adapters::inbound::dto::network_access_dto::NetworkAccessDto;
use crate::modules::preferences::infrastructure::adapters::inbound::server_fns::{
    RegenerateAccessToken, SaveNetworkAccess, SaveRequireAccessToken,
};
use crate::modules::preferences::presentation::state::network_access_state::NetworkAccessState;

const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};
const CARD_TITLE: Msg = Msg {
    en: "Network access",
    es: "Acceso desde la red",
    zh: "网络访问",
    de: "Netzwerkzugriff",
    fr: "Accès réseau",
};
const CARD_HELP: Msg = Msg {
    en: "By default iworkflow only listens on this computer. Enable network access to reach it from other devices, such as your phone. The change applies the next time the server starts.",
    es: "Por defecto iworkflow solo escucha en este equipo. Activa el acceso desde la red para usarlo desde otros dispositivos, como el móvil. El cambio se aplica la próxima vez que arranque el servidor.",
    zh: "默认情况下，iworkflow 只在本机监听。启用网络访问后，可从其他设备（例如手机）访问。更改将在服务器下次启动时生效。",
    de: "Standardmäßig lauscht iworkflow nur auf diesem Computer. Aktiviere den Netzwerkzugriff, um von anderen Geräten wie dem Smartphone darauf zuzugreifen. Die Änderung gilt ab dem nächsten Serverstart.",
    fr: "Par défaut, iworkflow n'écoute que sur cet ordinateur. Activez l'accès réseau pour l'utiliser depuis d'autres appareils, comme votre téléphone. Le changement s'applique au prochain démarrage du serveur.",
};
const WARNING: Msg = Msg {
    en: "iworkflow has no authentication and its agents and terminals run commands as your user. Anyone who can reach this port gets full control of this computer. Only enable it on a network you trust or a private network such as Tailscale.",
    es: "iworkflow no tiene autenticación y sus agentes y terminales ejecutan comandos con tu usuario. Quien llegue a este puerto controla por completo este equipo. Actívalo solo en una red de confianza o privada, como Tailscale.",
    zh: "iworkflow 没有身份验证，其智能体和终端会以你的用户身份执行命令。任何能访问此端口的人都能完全控制这台电脑。请仅在可信网络或 Tailscale 等私有网络中启用。",
    de: "iworkflow hat keine Authentifizierung, und seine Agenten und Terminals führen Befehle als dein Benutzer aus. Wer diesen Port erreicht, hat die volle Kontrolle über diesen Computer. Aktiviere es nur in einem vertrauenswürdigen oder privaten Netzwerk wie Tailscale.",
    fr: "iworkflow n'a pas d'authentification et ses agents et terminaux exécutent des commandes avec votre utilisateur. Quiconque atteint ce port contrôle entièrement cet ordinateur. Activez-le seulement sur un réseau de confiance ou privé, comme Tailscale.",
};
const TOGGLE_LABEL: Msg = Msg {
    en: "Allow access from other devices on the network",
    es: "Permitir el acceso desde otros dispositivos de la red",
    zh: "允许网络中的其他设备访问",
    de: "Zugriff von anderen Geräten im Netzwerk erlauben",
    fr: "Autoriser l'accès depuis d'autres appareils du réseau",
};
const LOCKED_BY_ENV: Msg = Msg {
    en: "The address is fixed by the LEPTOS_SITE_ADDR environment variable, so this setting has no effect.",
    es: "La dirección está fijada por la variable de entorno LEPTOS_SITE_ADDR, así que este ajuste no tiene efecto.",
    zh: "地址由环境变量 LEPTOS_SITE_ADDR 固定，因此此设置不起作用。",
    de: "Die Adresse ist durch die Umgebungsvariable LEPTOS_SITE_ADDR festgelegt, daher hat diese Einstellung keine Wirkung.",
    fr: "L'adresse est fixée par la variable d'environnement LEPTOS_SITE_ADDR ; ce réglage n'a donc aucun effet.",
};
const RESTART_PENDING: Msg = Msg {
    en: "Restart the server to apply the change.",
    es: "Reinicia el servidor para aplicar el cambio.",
    zh: "请重启服务器以应用更改。",
    de: "Starte den Server neu, um die Änderung zu übernehmen.",
    fr: "Redémarrez le serveur pour appliquer le changement.",
};
const STATUS_EXPOSED: Msg = Msg {
    en: "Reachable from the network",
    es: "Accesible desde la red",
    zh: "可通过网络访问",
    de: "Im Netzwerk erreichbar",
    fr: "Accessible depuis le réseau",
};
const STATUS_PRIVATE: Msg = Msg {
    en: "Only this computer",
    es: "Solo este equipo",
    zh: "仅限本机",
    de: "Nur dieser Computer",
    fr: "Cet ordinateur uniquement",
};
const OPEN_FROM_PHONE: Msg = Msg {
    en: "Open this address on your phone:",
    es: "Abre esta dirección en el móvil:",
    zh: "在手机上打开此地址：",
    de: "Öffne diese Adresse auf dem Smartphone:",
    fr: "Ouvrez cette adresse sur votre téléphone :",
};
const REQUIRE_TOKEN_LABEL: Msg = Msg {
    en: "Require the access token (recommended)",
    es: "Exigir el token de acceso (recomendado)",
    zh: "要求访问令牌（推荐）",
    de: "Zugriffstoken verlangen (empfohlen)",
    fr: "Exiger le jeton d'accès (recommandé)",
};
const TOKEN_OFF_WARNING: Msg = Msg {
    en: "Without the token, anyone on the network can use iworkflow and run commands on this computer.",
    es: "Sin el token, cualquiera en la red puede usar iworkflow y ejecutar comandos en este equipo.",
    zh: "没有令牌时，网络中的任何人都可以使用 iworkflow 并在这台电脑上执行命令。",
    de: "Ohne Token kann jeder im Netzwerk iworkflow nutzen und Befehle auf diesem Computer ausführen.",
    fr: "Sans le jeton, toute personne du réseau peut utiliser iworkflow et exécuter des commandes sur cet ordinateur.",
};
const SCAN_QR: Msg = Msg {
    en: "Scan this code with your phone to open iworkflow already signed in:",
    es: "Escanea este código con el móvil para abrir iworkflow ya autenticado:",
    zh: "用手机扫描此二维码，即可以已登录状态打开 iworkflow：",
    de: "Scanne diesen Code mit dem Smartphone, um iworkflow bereits angemeldet zu öffnen:",
    fr: "Scannez ce code avec votre téléphone pour ouvrir iworkflow déjà connecté :",
};
const QR_LABEL: Msg = Msg {
    en: "QR code to open iworkflow from your phone",
    es: "Código QR para abrir iworkflow desde el móvil",
    zh: "用于在手机上打开 iworkflow 的二维码",
    de: "QR-Code zum Öffnen von iworkflow auf dem Smartphone",
    fr: "Code QR pour ouvrir iworkflow depuis votre téléphone",
};
const TOKEN_SECRET_NOTE: Msg = Msg {
    en: "This link contains the access token: share it only with devices you trust.",
    es: "Este enlace contiene el token de acceso: compártelo solo con dispositivos de confianza.",
    zh: "此链接包含访问令牌：仅与你信任的设备共享。",
    de: "Dieser Link enthält das Zugriffstoken: Teile ihn nur mit vertrauenswürdigen Geräten.",
    fr: "Ce lien contient le jeton d'accès : ne le partagez qu'avec des appareils de confiance.",
};
const TOKEN_LABEL: Msg = Msg {
    en: "Access token",
    es: "Token de acceso",
    zh: "访问令牌",
    de: "Zugriffstoken",
    fr: "Jeton d'accès",
};
const REGENERATE: Msg = Msg {
    en: "Generate a new token",
    es: "Generar un token nuevo",
    zh: "生成新令牌",
    de: "Neues Token erzeugen",
    fr: "Générer un nouveau jeton",
};
const REGENERATE_NOTE: Msg = Msg {
    en: "Devices that use the current token will have to sign in again.",
    es: "Los dispositivos que usan el token actual tendrán que volver a autenticarse.",
    zh: "使用当前令牌的设备需要重新登录。",
    de: "Geräte mit dem aktuellen Token müssen sich erneut anmelden.",
    fr: "Les appareils qui utilisent le jeton actuel devront se reconnecter.",
};
const ADDRESS_PLACEHOLDER: &str = "<IP>";

/// Whether the saved choice differs from what the running server does.
pub fn restart_pending(access: &NetworkAccessDto) -> bool {
    if access.locked_by_env {
        return false;
    }
    access.enabled != access.active
        || (access.active && access.require_token != access.token_active)
}

/// Settings card that opts the server in or out of listening on every network interface.
#[component]
pub fn NetworkAccessCard() -> impl IntoView {
    let state: NetworkAccessState = NetworkAccessState::new();
    view! {
        <Card title=CARD_TITLE.get()>
            <p class="muted text-sm">{CARD_HELP.get()}</p>
            <Transition fallback=|| view! { <p class="muted">{LOADING.get()}</p> }>
                {move || Suspend::new(async move {
                    match state.access.await {
                        Ok(access) => view! { <NetworkAccessForm state access /> }.into_any(),
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}

#[component]
fn NetworkAccessForm(state: NetworkAccessState, access: NetworkAccessDto) -> impl IntoView {
    let busy: Signal<bool> = state.busy();
    let enabled: bool = access.enabled;
    let require_token: bool = access.require_token;
    let locked: bool = access.locked_by_env;
    let pending_restart: bool = restart_pending(&access);
    let reachable: bool = access.enabled || access.active;
    let (status_text, status_variant): (&str, &str) = if access.active {
        (STATUS_EXPOSED.get(), "warn")
    } else {
        (STATUS_PRIVATE.get(), "ok")
    };
    // Without a detected address there is nothing to scan, but the user still needs a URL hint.
    let plain_url: Option<String> = (access.active && access.access_url.is_none())
        .then(|| format!("http://{ADDRESS_PLACEHOLDER}:{}", access.port));
    let qr_svg: Option<String> = access.qr_svg.clone();
    let access_url: Option<String> = access.access_url.clone();
    let token: Option<String> = access.token.clone();

    view! {
        <div class="row items-center gap-2">
            <Badge text=status_text variant=status_variant />
        </div>
        <p class="text-sm text-warn" role="note">{WARNING.get()}</p>
        <label class="row items-center gap-2 text-sm text-fg">
            <input
                type="checkbox"
                class="h-4 w-4"
                prop:checked=enabled
                disabled=move || busy.get() || locked
                on:change=move |ev| {
                    state.save.dispatch(SaveNetworkAccess { enabled: event_target_checked(&ev) });
                }
            />
            {TOGGLE_LABEL.get()}
        </label>
        {locked.then(|| view! { <p class="muted text-sm">{LOCKED_BY_ENV.get()}</p> })}
        {reachable.then(|| view! {
            <label class="row items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-4 w-4"
                    prop:checked=require_token
                    disabled=move || busy.get()
                    on:change=move |ev| {
                        state
                            .save_require_token
                            .dispatch(SaveRequireAccessToken {
                                required: event_target_checked(&ev),
                            });
                    }
                />
                {REQUIRE_TOKEN_LABEL.get()}
            </label>
        })}
        {(reachable && !require_token)
            .then(|| view! { <p class="text-sm text-error" role="alert">{TOKEN_OFF_WARNING.get()}</p> })}
        {pending_restart.then(|| view! { <p class="text-sm text-warn">{RESTART_PENDING.get()}</p> })}
        {qr_svg.map(|svg: String| view! {
            <div class="stack gap-2">
                <p class="text-sm text-fg">{SCAN_QR.get()}</p>
                <div
                    class="w-48 max-w-full rounded bg-white p-2"
                    role="img"
                    aria-label=QR_LABEL.get()
                    inner_html=svg
                ></div>
            </div>
        })}
        {access_url.map(|url: String| view! {
            <div class="stack gap-1">
                <code class="break-all text-xs">{url}</code>
                <p class="muted text-sm">{TOKEN_SECRET_NOTE.get()}</p>
            </div>
        })}
        {plain_url.map(|url: String| view! {
            <p class="text-sm text-fg">{OPEN_FROM_PHONE.get()} " " <code>{url}</code></p>
        })}
        {token.map(|token: String| view! {
            <div class="stack gap-1">
                <span class="text-sm font-medium text-fg">{TOKEN_LABEL.get()}</span>
                <code class="break-all text-xs">{token}</code>
                <div class="row items-center gap-2">
                    <button
                        type="button"
                        class="btn-secondary"
                        disabled=move || busy.get()
                        on:click=move |_| {
                            state.regenerate.dispatch(RegenerateAccessToken {});
                        }
                    >
                        {REGENERATE.get()}
                    </button>
                    <span class="muted text-sm">{REGENERATE_NOTE.get()}</span>
                </div>
            </div>
        })}
        <ActionError result=state.save.value().into() />
        <ActionError result=state.save_require_token.value().into() />
        <ActionError result=state.regenerate.value().into() />
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn dto(enabled: bool, active: bool, locked_by_env: bool) -> NetworkAccessDto {
        // Token choice and running state agree by default, so only the case under test differs.
        NetworkAccessDto::new(enabled, active, locked_by_env, 3000, None)
            .with_token_state(true, true)
    }

    fn with_token(access: NetworkAccessDto, require: bool, active: bool) -> NetworkAccessDto {
        access.with_token_state(require, active)
    }

    #[test]
    fn restart_is_pending_when_the_choice_differs_from_the_running_server() {
        assert!(restart_pending(&dto(true, false, false)));
        assert!(restart_pending(&dto(false, true, false)));
    }

    #[test]
    fn no_restart_when_they_match_or_the_environment_decides() {
        assert!(!restart_pending(&dto(true, true, false)));
        assert!(!restart_pending(&dto(false, false, false)));
        assert!(!restart_pending(&dto(true, false, true)));
    }

    #[test]
    fn restart_is_pending_when_only_the_token_choice_differs() {
        assert!(restart_pending(&with_token(
            dto(true, true, false),
            false,
            true
        )));
        assert!(restart_pending(&with_token(
            dto(true, true, false),
            true,
            false
        )));
    }

    #[test]
    fn token_choice_is_ignored_while_the_server_is_private() {
        assert!(!restart_pending(&with_token(
            dto(false, false, false),
            true,
            false
        )));
    }
}
