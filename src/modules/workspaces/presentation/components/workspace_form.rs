use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{SelectField, SelectOption, TextField};
use crate::core::i18n::Msg;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::AgentsFileOptionDto;

const PATH_INPUT_ID: &str = "workspace-path";

const NAME: Msg = Msg {
    en: "Name",
    es: "Nombre",
    zh: "名称",
    de: "Name",
    fr: "Nom",
};
const NAME_PLACEHOLDER: Msg = Msg {
    en: "my-project",
    es: "mi-proyecto",
    zh: "我的项目",
    de: "mein-projekt",
    fr: "mon-projet",
};
const ABSOLUTE_PATH: Msg = Msg {
    en: "Absolute path (created if it does not exist)",
    es: "Ruta absoluta (se crea si no existe)",
    zh: "绝对路径（不存在时会创建）",
    de: "Absoluter Pfad (wird erstellt, falls nicht vorhanden)",
    fr: "Chemin absolu (créé s'il n'existe pas)",
};
const PATH_PLACEHOLDER: Msg = Msg {
    en: "/home/user/projects/my-project",
    es: "/home/usuario/proyectos/mi-proyecto",
    zh: "/home/用户/项目/我的项目",
    de: "/home/benutzer/projekte/mein-projekt",
    fr: "/home/utilisateur/projets/mon-projet",
};
const CHOOSE: Msg = Msg {
    en: "Choose…",
    es: "Elegir…",
    zh: "选择…",
    de: "Auswählen…",
    fr: "Choisir…",
};
const BASE_BRANCH: Msg = Msg {
    en: "Base branch (empty = current branch)",
    es: "Rama base (vacío = rama actual)",
    zh: "基础分支（留空 = 当前分支）",
    de: "Basis-Branch (leer = aktueller Branch)",
    fr: "Branche de base (vide = branche actuelle)",
};
const INITIAL_AGENTS: Msg = Msg {
    en: "Initial AGENTS.md (optional)",
    es: "AGENTS.md inicial (opcional)",
    zh: "初始 AGENTS.md（可选）",
    de: "Initiale AGENTS.md (optional)",
    fr: "AGENTS.md initial (facultatif)",
};
const NO_AGENTS: Msg = Msg {
    en: "No AGENTS.md",
    es: "Sin AGENTS.md",
    zh: "无 AGENTS.md",
    de: "Keine AGENTS.md",
    fr: "Sans AGENTS.md",
};
const REGISTERING: Msg = Msg {
    en: "Registering…",
    es: "Registrando…",
    zh: "注册中…",
    de: "Wird registriert…",
    fr: "Enregistrement…",
};
const REGISTER: Msg = Msg {
    en: "Register",
    es: "Registrar",
    zh: "注册",
    de: "Registrieren",
    fr: "Enregistrer",
};

/// Formulario de alta de workspace. El envío se delega en `on_submit`.
#[component]
pub fn WorkspaceForm(
    name: RwSignal<String>,
    path: RwSignal<String>,
    base_branch: RwSignal<String>,
    agents_file_id: RwSignal<String>,
    agents_file_options: Resource<Result<Vec<AgentsFileOptionDto>, ServerFnError>>,
    #[prop(into)] pending: Signal<bool>,
    on_submit: Callback<()>,
    on_browse: Callback<()>,
) -> impl IntoView {
    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        on_submit.run(());
    };

    view! {
        <form class="stack" on:submit=submit>
            <TextField label=NAME.get() value=name placeholder=NAME_PLACEHOLDER.get() required=true />
            <div class="stack gap-1.5">
                <label for=PATH_INPUT_ID class="text-sm font-medium text-fg">
                    {ABSOLUTE_PATH.get()}
                </label>
                <div class="row gap-2">
                    <input
                        id=PATH_INPUT_ID
                        type="text"
                        class="input min-w-0 flex-1"
                        placeholder=PATH_PLACEHOLDER.get()
                        required=true
                        prop:value=move || path.get()
                        on:input=move |ev| path.set(event_target_value(&ev))
                    />
                    <button type="button" class="btn-ghost" on:click=move |_| on_browse.run(())>
                        {CHOOSE.get()}
                    </button>
                </div>
            </div>
            <TextField
                label=BASE_BRANCH.get()
                value=base_branch
                placeholder="main"
            />
            <Transition fallback=|| ()>
                {move || Suspend::new(async move {
                    let options: Vec<SelectOption> = agents_file_options
                        .await
                        .unwrap_or_default()
                        .into_iter()
                        .map(|o: AgentsFileOptionDto| SelectOption {
                            value: o.id,
                            label: o.description,
                        })
                        .collect();
                    (!options.is_empty()).then(|| {
                        view! {
                            <SelectField
                                label=INITIAL_AGENTS.get()
                                value=agents_file_id
                                options
                                placeholder=NO_AGENTS.get()
                            />
                        }
                    })
                })}
            </Transition>
            <div class="row">
                <button type="submit" class="btn-primary" disabled=move || pending.get()>
                    {move || if pending.get() { REGISTERING.get() } else { REGISTER.get() }}
                </button>
            </div>
        </form>
    }
}
