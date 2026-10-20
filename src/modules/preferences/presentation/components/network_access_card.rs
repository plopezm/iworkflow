use leptos::prelude::*;

use crate::core::browser::reload_when_server_returns;
use crate::core::components::busy::Loading;
use crate::core::components::form::ActionError;
use crate::core::components::layout::{Badge, Card, ErrorText};
use crate::core::i18n::Msg;
use crate::modules::preferences::infrastructure::adapters::inbound::dto::network_access_dto::{
    AccessTargetDto, NetworkAccessDto,
};
use crate::modules::preferences::infrastructure::adapters::inbound::server_fns::{
    RegenerateAccessToken, RestartServer, SaveListenPort, SaveNetworkAccess, SaveRequireAccessToken,
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
const RESTART_NOW: Msg = Msg {
    en: "Restart now",
    es: "Reiniciar ahora",
    zh: "立即重启",
    de: "Jetzt neu starten",
    fr: "Redémarrer maintenant",
};
const RESTARTING: Msg = Msg {
    en: "Restarting… this page will reload when the server is back.",
    es: "Reiniciando… esta página se recargará cuando el servidor vuelva.",
    zh: "正在重启…服务器恢复后此页面将重新加载。",
    de: "Neustart läuft… Diese Seite wird neu geladen, sobald der Server wieder da ist.",
    fr: "Redémarrage… cette page se rechargera quand le serveur sera de retour.",
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
const SCAN_QR_SIGNED_IN: Msg = Msg {
    en: "Scan this code with your phone to open iworkflow already signed in:",
    es: "Escanea este código con el móvil para abrir iworkflow ya autenticado:",
    zh: "用手机扫描此二维码，即可以已登录状态打开 iworkflow：",
    de: "Scanne diesen Code mit dem Smartphone, um iworkflow bereits angemeldet zu öffnen:",
    fr: "Scannez ce code avec votre téléphone pour ouvrir iworkflow déjà connecté :",
};
const SCAN_QR_PLAIN: Msg = Msg {
    en: "Scan this code with your phone to open iworkflow:",
    es: "Escanea este código con el móvil para abrir iworkflow:",
    zh: "用手机扫描此二维码以打开 iworkflow：",
    de: "Scanne diesen Code mit dem Smartphone, um iworkflow zu öffnen:",
    fr: "Scannez ce code avec votre téléphone pour ouvrir iworkflow :",
};
const INTERFACE_LABEL: Msg = Msg {
    en: "Network interface",
    es: "Interfaz de red",
    zh: "网络接口",
    de: "Netzwerkschnittstelle",
    fr: "Interface réseau",
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
const PORT_LABEL: Msg = Msg {
    en: "Server port",
    es: "Puerto del servidor",
    zh: "服务器端口",
    de: "Server-Port",
    fr: "Port du serveur",
};
const PORT_HELP: Msg = Msg {
    en: "Between 1024 and 65535. The change applies the next time the server starts.",
    es: "Entre 1024 y 65535. El cambio se aplica la próxima vez que arranque el servidor.",
    zh: "范围为 1024 至 65535。更改将在服务器下次启动时生效。",
    de: "Zwischen 1024 und 65535. Die Änderung gilt ab dem nächsten Serverstart.",
    fr: "Entre 1024 et 65535. Le changement s'applique au prochain démarrage du serveur.",
};
const PORT_SAVE: Msg = Msg {
    en: "Save port",
    es: "Guardar puerto",
    zh: "保存端口",
    de: "Port speichern",
    fr: "Enregistrer le port",
};
/// Lowest port the server accepts (the domain enforces it too).
const MIN_PORT: u16 = 1024;
const ADDRESS_PLACEHOLDER: &str = "<IP>";

/// Whether the saved choice differs from what the running server does.
pub fn restart_pending(access: &NetworkAccessDto) -> bool {
    if access.locked_by_env {
        return false;
    }
    access.enabled != access.active
        || access.configured_port != access.port
        || (access.active && access.require_token != access.token_active)
}

/// Settings card that opts the server in or out of listening on every network interface.
#[component]
pub fn NetworkAccessCard() -> impl IntoView {
    let state: NetworkAccessState = NetworkAccessState::new();
    view! {
        <Card title=CARD_TITLE.get()>
            <p class="muted text-sm">{CARD_HELP.get()}</p>
            <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
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
    let plain_url: Option<String> = (reachable && access.targets.is_empty())
        .then(|| format!("http://{ADDRESS_PLACEHOLDER}:{}", access.port));
    let targets: Vec<AccessTargetDto> = access.targets.clone();
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
                class:is-busy=move || busy.get()
                on:change=move |ev| {
                    state.save.dispatch(SaveNetworkAccess { enabled: event_target_checked(&ev) });
                }
            />
            {TOGGLE_LABEL.get()}
        </label>
        {locked.then(|| view! { <p class="muted text-sm">{LOCKED_BY_ENV.get()}</p> })}
        <PortField state configured_port=access.configured_port locked />
        {reachable.then(|| view! {
            <label class="row items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-4 w-4"
                    prop:checked=require_token
                    disabled=move || busy.get()
                    class:is-busy=move || busy.get()
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
        {pending_restart.then(|| view! { <RestartPrompt state port=access.configured_port /> })}
        {(!targets.is_empty()).then(|| view! { <AccessTargets state targets require_token /> })}
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
                        class:is-busy=move || busy.get()
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
        <ActionError result=state.save_port.value().into() />
        <ActionError result=state.regenerate.value().into() />
        <ActionError result=state.restart.value().into() />
    }
}

/// Tells the user the saved change needs a restart and offers to do it right away.
#[component]
fn RestartPrompt(state: NetworkAccessState, port: u16) -> impl IntoView {
    let busy: Signal<bool> = state.busy();
    let accepted: Signal<bool> =
        Signal::derive(move || matches!(state.restart.value().get(), Some(Ok(()))));
    // Once the server accepted the restart, wait for it (on the saved port) and reopen the page.
    Effect::new(move |_| {
        if accepted.get() {
            reload_when_server_returns(port);
        }
    });
    view! {
        <div class="stack gap-1">
            <p class="text-sm text-warn" role="status">
                {move || if accepted.get() { RESTARTING.get() } else { RESTART_PENDING.get() }}
            </p>
            <div class="row items-center gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    disabled=move || busy.get() || accepted.get()
                    class:is-busy=move || busy.get()
                    on:click=move |_| {
                        state.restart.dispatch(RestartServer {});
                    }
                >
                    {RESTART_NOW.get()}
                </button>
            </div>
        </div>
    }
}

/// Number input and save button for the port the server listens on.
#[component]
fn PortField(state: NetworkAccessState, configured_port: u16, locked: bool) -> impl IntoView {
    let busy: Signal<bool> = state.busy();
    let draft: RwSignal<String> = RwSignal::new(configured_port.to_string());
    view! {
        <div class="stack gap-1">
            <label class="row items-center gap-2 text-sm text-fg">
                {PORT_LABEL.get()}
                <input
                    type="number"
                    class="input w-28"
                    min="1024"
                    max="65535"
                    prop:value=move || draft.get()
                    disabled=move || busy.get() || locked
                    class:is-busy=move || busy.get()
                    on:input=move |ev| draft.set(event_target_value(&ev))
                />
            </label>
            <div class="row items-center gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    disabled=move || busy.get() || locked || parse_port(&draft.get()).is_none()
                    class:is-busy=move || busy.get()
                    on:click=move |_| {
                        if let Some(port) = parse_port(&draft.get_untracked()) {
                            state.save_port.dispatch(SaveListenPort { port });
                        }
                    }
                >
                    {PORT_SAVE.get()}
                </button>
                <span class="muted text-sm">{PORT_HELP.get()}</span>
            </div>
        </div>
    }
}

/// Port typed by the user, or `None` while the text is not a number the server could bind.
pub fn parse_port(text: &str) -> Option<u16> {
    text.trim()
        .parse::<u16>()
        .ok()
        .filter(|port: &u16| *port >= MIN_PORT)
}

/// Interface selector plus the link and QR code of the chosen interface.
#[component]
fn AccessTargets(
    state: NetworkAccessState,
    targets: Vec<AccessTargetDto>,
    require_token: bool,
) -> impl IntoView {
    let options: Vec<AccessTargetDto> = targets.clone();
    let selected: Memo<AccessTargetDto> = Memo::new(move |_| {
        let wanted: Option<String> = state.selected_address.get();
        pick_target(&targets, wanted.as_deref()).clone()
    });
    view! {
        <div class="stack gap-2">
            <label class="row items-center gap-2 text-sm text-fg">
                {INTERFACE_LABEL.get()}
                <select
                    class="input"
                    on:change=move |ev| state.selected_address.set(Some(event_target_value(&ev)))
                >
                    {options
                        .into_iter()
                        .map(|target: AccessTargetDto| {
                            let address: String = target.address.clone();
                            let label: String = format!("{} — {}", target.interface, target.address);
                            view! {
                                <option
                                    value=address.clone()
                                    selected=move || selected.get().address == address
                                >
                                    {label}
                                </option>
                            }
                        })
                        .collect_view()}
                </select>
            </label>
            <p class="text-sm text-fg">
                {if require_token { SCAN_QR_SIGNED_IN.get() } else { SCAN_QR_PLAIN.get() }}
            </p>
            {move || selected.get().qr_svg.map(|svg: String| view! {
                <div
                    class="w-48 max-w-full rounded bg-white p-2"
                    role="img"
                    aria-label=QR_LABEL.get()
                    inner_html=svg
                ></div>
            })}
            <code class="break-all text-xs">{move || selected.get().access_url}</code>
            {require_token.then(|| view! { <p class="muted text-sm">{TOKEN_SECRET_NOTE.get()}</p> })}
        </div>
    }
}

/// The target matching `wanted`, or the first (preferred) one. `targets` must not be empty.
pub fn pick_target<'a>(
    targets: &'a [AccessTargetDto],
    wanted: Option<&str>,
) -> &'a AccessTargetDto {
    wanted
        .and_then(|address: &str| {
            targets
                .iter()
                .find(|t: &&AccessTargetDto| t.address == address)
        })
        .unwrap_or(&targets[0])
}

#[cfg(test)]
mod tests {
    use super::*;

    fn dto(enabled: bool, active: bool, locked_by_env: bool) -> NetworkAccessDto {
        // Token choice and running state agree by default, so only the case under test differs.
        NetworkAccessDto::new(enabled, active, locked_by_env, 3000).with_token_state(true, true)
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

    fn target(interface: &str, address: &str) -> AccessTargetDto {
        AccessTargetDto {
            interface: interface.to_string(),
            address: address.to_string(),
            access_url: format!("http://{address}:3000"),
            qr_svg: None,
        }
    }

    #[test]
    fn restart_is_pending_when_only_the_port_differs() {
        assert!(restart_pending(
            &dto(true, true, false).with_configured_port(9100)
        ));
        assert!(!restart_pending(
            &dto(true, true, true).with_configured_port(9100)
        ));
    }

    #[test]
    fn parse_port_accepts_only_bindable_numbers() {
        assert_eq!(parse_port(" 9000 "), Some(9000));
        assert_eq!(parse_port("80"), None);
        assert_eq!(parse_port("70000"), None);
        assert_eq!(parse_port("abc"), None);
        assert_eq!(parse_port(""), None);
    }

    #[test]
    fn picks_the_chosen_interface_or_falls_back_to_the_first() {
        let targets: Vec<AccessTargetDto> = vec![
            target("eth0", "192.168.1.5"),
            target("tailscale0", "100.64.0.7"),
        ];
        assert_eq!(
            pick_target(&targets, Some("100.64.0.7")).interface,
            "tailscale0"
        );
        assert_eq!(pick_target(&targets, Some("10.9.9.9")).interface, "eth0");
        assert_eq!(pick_target(&targets, None).interface, "eth0");
    }
}
