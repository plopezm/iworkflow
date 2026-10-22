use leptos::prelude::*;

use crate::core::components::busy::Loading;

use crate::core::components::confirm_panel::ConfirmPanel;
use crate::core::components::form::{ActionError, TextField};
use crate::core::components::layout::{Badge, ErrorText};
use crate::core::i18n::Msg;
use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::{
    CreateMergeRequestDto, MergeRequestDto, MergeRequestStatusDto, PipelineDto, PipelineJobDto,
};
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    CreateTaskMergeRequest, MergeTaskMergeRequest, PushTaskBranch,
};
use crate::modules::code_hosting::presentation::provider_text;
use crate::modules::code_hosting::presentation::state::task_merge_request_state::TaskMergeRequestState;

const NO_PUSH_NOTE: Msg = Msg {
    en: "If the branch is not on origin yet, it is pushed automatically.",
    es: "Si la rama aún no está en origin, se publica automáticamente (push).",
    zh: "如果分支尚未在 origin 上，将自动执行 push 发布。",
    de: "Ist der Branch noch nicht auf origin, wird er automatisch gepusht.",
    fr: "Si la branche n'est pas encore sur origin, elle est publiée automatiquement (push).",
};
const STATUS_SUCCESS: Msg = Msg {
    en: "successful",
    es: "correcto",
    zh: "成功",
    de: "erfolgreich",
    fr: "réussi",
};
const STATUS_FAILED: Msg = Msg {
    en: "failed",
    es: "fallido",
    zh: "失败",
    de: "fehlgeschlagen",
    fr: "échoué",
};
const STATUS_RUNNING: Msg = Msg {
    en: "running",
    es: "en curso",
    zh: "运行中",
    de: "läuft",
    fr: "en cours",
};
const STATUS_PENDING: Msg = Msg {
    en: "pending",
    es: "pendiente",
    zh: "等待中",
    de: "ausstehend",
    fr: "en attente",
};
const STATUS_CREATED: Msg = Msg {
    en: "created",
    es: "creado",
    zh: "已创建",
    de: "erstellt",
    fr: "créé",
};
const STATUS_CANCELED: Msg = Msg {
    en: "canceled",
    es: "cancelado",
    zh: "已取消",
    de: "abgebrochen",
    fr: "annulé",
};
const STATUS_SKIPPED: Msg = Msg {
    en: "skipped",
    es: "omitido",
    zh: "已跳过",
    de: "übersprungen",
    fr: "ignoré",
};
const STATUS_MANUAL: Msg = Msg {
    en: "manual",
    es: "manual",
    zh: "手动",
    de: "manuell",
    fr: "manuel",
};
const STATUS_OPENED: Msg = Msg {
    en: "open",
    es: "abierto",
    zh: "打开",
    de: "offen",
    fr: "ouvert",
};
const STATUS_MERGED: Msg = Msg {
    en: "merged",
    es: "mezclado",
    zh: "已合并",
    de: "gemergt",
    fr: "fusionné",
};
const STATUS_CLOSED: Msg = Msg {
    en: "closed",
    es: "cerrado",
    zh: "已关闭",
    de: "geschlossen",
    fr: "fermé",
};
const STATUS_MERGEABLE: Msg = Msg {
    en: "can be merged",
    es: "se puede mezclar",
    zh: "可以合并",
    de: "kann gemergt werden",
    fr: "peut être fusionné",
};
const STATUS_CANNOT_MERGE: Msg = Msg {
    en: "cannot be merged",
    es: "no se puede mezclar",
    zh: "无法合并",
    de: "kann nicht gemergt werden",
    fr: "ne peut pas être fusionné",
};
const STATUS_CONFLICT: Msg = Msg {
    en: "has conflicts",
    es: "con conflictos",
    zh: "存在冲突",
    de: "mit Konflikten",
    fr: "avec conflits",
};
const STATUS_CI_RUNNING: Msg = Msg {
    en: "CI running",
    es: "CI en curso",
    zh: "CI 运行中",
    de: "CI läuft",
    fr: "CI en cours",
};
const STATUS_CI_MUST_PASS: Msg = Msg {
    en: "CI must pass",
    es: "el CI debe pasar",
    zh: "CI 必须通过",
    de: "CI muss bestehen",
    fr: "la CI doit réussir",
};
const STATUS_NOT_APPROVED: Msg = Msg {
    en: "pending approval",
    es: "pendiente de aprobación",
    zh: "等待批准",
    de: "Freigabe ausstehend",
    fr: "en attente d'approbation",
};
const STATUS_DISCUSSIONS: Msg = Msg {
    en: "unresolved discussions",
    es: "discusiones sin resolver",
    zh: "讨论未解决",
    de: "ungelöste Diskussionen",
    fr: "discussions non résolues",
};
const STATUS_CHECKING: Msg = Msg {
    en: "checking",
    es: "comprobando",
    zh: "检查中",
    de: "wird geprüft",
    fr: "vérification",
};
const HAS_CONFLICTS: Msg = Msg {
    en: "has conflicts",
    es: "con conflictos",
    zh: "存在冲突",
    de: "mit Konflikten",
    fr: "avec conflits",
};
const OPEN_IN: Msg = Msg {
    en: "Open in {provider}",
    es: "Abrir en {provider}",
    zh: "在 {provider} 中打开",
    de: "In {provider} öffnen",
    fr: "Ouvrir dans {provider}",
};
const OPEN_REQUEST_IN: Msg = Msg {
    en: "Open {abbrev} in {provider}",
    es: "Abrir {abbrev} en {provider}",
    zh: "在 {provider} 中打开 {abbrev}",
    de: "{abbrev} in {provider} öffnen",
    fr: "Ouvrir la {abbrev} dans {provider}",
};
const MERGE_REQUEST_BUTTON: Msg = Msg {
    en: "Merge {abbrev}",
    es: "Mezclar {abbrev}",
    zh: "合并 {abbrev}",
    de: "{abbrev} mergen",
    fr: "Fusionner la {abbrev}",
};
const MERGE_CONFIRM: Msg = Msg {
    en: "Merge the {abbrev} into {provider}? The target branch will be updated on the server.",
    es: "¿Mezclar el {abbrev} en {provider}? La rama destino se actualizará en el servidor.",
    zh: "要将 {abbrev} 合并到 {provider} 吗？目标分支将在服务器上更新。",
    de: "{abbrev} in {provider} mergen? Der Zielbranch wird auf dem Server aktualisiert.",
    fr: "Fusionner la {abbrev} dans {provider} ? La branche cible sera mise à jour sur le serveur.",
};
const PUSH_BRANCH: Msg = Msg {
    en: "Push branch",
    es: "Subir rama (push)",
    zh: "推送分支",
    de: "Branch pushen",
    fr: "Pousser la branche",
};
const PUSH_DONE: Msg = Msg {
    en: "Branch pushed to origin.",
    es: "Rama subida a origin.",
    zh: "分支已推送到 origin。",
    de: "Branch wurde zu origin gepusht.",
    fr: "Branche poussée vers origin.",
};
const NO_PIPELINE: Msg = Msg {
    en: "No CI pipeline.",
    es: "Sin pipeline de CI.",
    zh: "没有 CI 流水线。",
    de: "Keine CI-Pipeline.",
    fr: "Aucun pipeline de CI.",
};
const MERGE: Msg = Msg {
    en: "Merge",
    es: "Mezclar",
    zh: "合并",
    de: "Mergen",
    fr: "Fusionner",
};
const NO_MR_YET: Msg = Msg {
    en: "This branch has no merge/pull request yet.",
    es: "Esta rama aún no tiene merge/pull request.",
    zh: "此分支还没有 merge/pull request。",
    de: "Dieser Branch hat noch keinen Merge/Pull Request.",
    fr: "Cette branche n'a pas encore de merge/pull request.",
};
const TITLE_OPTIONAL: Msg = Msg {
    en: "Title (optional, defaults to the task's)",
    es: "Título (opcional, por defecto el de la tarea)",
    zh: "标题（可选，默认为任务标题）",
    de: "Titel (optional, standardmäßig der der Aufgabe)",
    fr: "Titre (facultatif, par défaut celui de la tâche)",
};
const CREATE_MR: Msg = Msg {
    en: "Create merge/pull request",
    es: "Crear merge/pull request",
    zh: "创建 merge/pull request",
    de: "Merge/Pull Request erstellen",
    fr: "Créer une merge/pull request",
};
const PANEL_TITLE: Msg = Msg {
    en: "Merge / Pull request",
    es: "Merge / Pull request",
    zh: "Merge / Pull request",
    de: "Merge / Pull Request",
    fr: "Merge / Pull request",
};
const REFRESH: Msg = Msg {
    en: "Refresh",
    es: "Actualizar",
    zh: "刷新",
    de: "Aktualisieren",
    fr: "Actualiser",
};
const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};

/// Variante de color de un estado de MR/PR, pipeline o job (pura).
pub fn status_variant(status: &str) -> &'static str {
    match status {
        "success" | "passed" | "merged" | "mergeable" | "can_be_merged" => "ok",
        "failed" | "cannot_be_merged" | "conflict" | "broken_status" => "error",
        "running"
        | "pending"
        | "created"
        | "preparing"
        | "waiting_for_resource"
        | "scheduled"
        | "checking"
        | "ci_still_running"
        | "opened" => "info",
        "manual" | "blocked_status" | "not_approved" | "discussions_not_resolved" => "warn",
        _ => "muted",
    }
}

/// Etiqueta traducida de un estado de MR/PR o CI; los desconocidos se muestran tal cual (pura).
pub fn status_label(status: &str) -> String {
    let label: Option<Msg> = match status {
        "success" => Some(STATUS_SUCCESS),
        "failed" => Some(STATUS_FAILED),
        "running" => Some(STATUS_RUNNING),
        "pending" => Some(STATUS_PENDING),
        "created" => Some(STATUS_CREATED),
        "canceled" => Some(STATUS_CANCELED),
        "skipped" => Some(STATUS_SKIPPED),
        "manual" => Some(STATUS_MANUAL),
        "opened" => Some(STATUS_OPENED),
        "merged" => Some(STATUS_MERGED),
        "closed" => Some(STATUS_CLOSED),
        "mergeable" | "can_be_merged" => Some(STATUS_MERGEABLE),
        "cannot_be_merged" => Some(STATUS_CANNOT_MERGE),
        "conflict" => Some(STATUS_CONFLICT),
        "ci_still_running" => Some(STATUS_CI_RUNNING),
        "ci_must_pass" => Some(STATUS_CI_MUST_PASS),
        "not_approved" => Some(STATUS_NOT_APPROVED),
        "discussions_not_resolved" => Some(STATUS_DISCUSSIONS),
        "checking" => Some(STATUS_CHECKING),
        _ => None,
    };
    label.map_or_else(|| status.to_string(), |msg: Msg| msg.get().to_string())
}

#[component]
fn StatusBadge(#[prop(into)] status: String) -> impl IntoView {
    view! { <Badge text=status_label(&status) variant=status_variant(&status) /> }
}

#[component]
fn ExternalLink(#[prop(into)] href: String, children: Children) -> impl IntoView {
    view! {
        <a class="text-sm underline" href=href target="_blank" rel="noopener noreferrer">
            {children()}
        </a>
    }
}

#[component]
fn PipelineView(pipeline: PipelineDto, provider: String) -> impl IntoView {
    let title: String = format!(
        "{} #{}",
        provider_text::pipeline_noun(&provider),
        pipeline.id
    );
    let open_label: String = OPEN_IN.fmt(&[("provider", provider_text::label(&provider))]);
    view! {
        <div class="stack gap-2">
            <div class="row items-center gap-2">
                <span class="text-sm font-medium text-fg">{title}</span>
                <StatusBadge status=pipeline.status />
                <ExternalLink href=pipeline.web_url>{open_label}</ExternalLink>
            </div>
            <ul class="stack gap-1">
                {pipeline
                    .jobs
                    .into_iter()
                    .map(|job: PipelineJobDto| {
                        view! {
                            <li class="row items-center gap-2 text-sm">
                                <StatusBadge status=job.status />
                                <span class="text-fg">{job.name}</span>
                                <span class="muted">{format!("({})", job.stage)}</span>
                                <ExternalLink href=job.web_url>"log"</ExternalLink>
                            </li>
                        }
                    })
                    .collect_view()}
            </ul>
        </div>
    }
}

#[component]
fn MergeRequestView(state: TaskMergeRequestState, status: MergeRequestStatusDto) -> impl IntoView {
    let provider: String = status.provider;
    let merge_request: MergeRequestDto = status.merge_request;
    let open: bool = merge_request.is_open();
    let busy: Signal<bool> = state.busy();
    let confirming: RwSignal<bool> = RwSignal::new(false);
    let heading: String = format!(
        "{}{} {}",
        provider_text::request_prefix(&provider),
        merge_request.iid,
        merge_request.title
    );
    let abbrev: &str = provider_text::request_abbrev(&provider);
    let open_request_label: String = OPEN_REQUEST_IN.fmt(&[
        ("abbrev", abbrev),
        ("provider", provider_text::label(&provider)),
    ]);
    let merge_label: String = MERGE_REQUEST_BUTTON.fmt(&[("abbrev", abbrev)]);
    let merge_confirm: String = MERGE_CONFIRM.fmt(&[
        ("abbrev", abbrev),
        ("provider", provider_text::label(&provider)),
    ]);
    let branches: String = format!(
        "{} → {} · {}",
        merge_request.source_branch, merge_request.target_branch, status.project_path
    );
    let on_confirm: Callback<()> = Callback::new(move |_| {
        confirming.set(false);
        state.merge.dispatch(MergeTaskMergeRequest {
            task_id: state.task_id.get_value(),
        });
    });
    let on_cancel: Callback<()> = Callback::new(move |_| confirming.set(false));
    let on_push = move |_| {
        state.push.dispatch(PushTaskBranch {
            task_id: state.task_id.get_value(),
        });
    };
    let pushed: Signal<bool> =
        Signal::derive(move || matches!(state.push.value().get(), Some(Ok(()))));
    view! {
        <div class="stack gap-3">
            <div class="stack gap-1">
                <span class="text-sm font-medium text-fg">{heading}</span>
                <span class="muted text-xs">{branches}</span>
                <div class="row flex-wrap items-center gap-2">
                    <StatusBadge status=merge_request.state.clone() />
                    {open
                        .then(|| {
                            view! { <StatusBadge status=merge_request.merge_status.clone() /> }
                        })}
                    {merge_request
                        .has_conflicts
                        .then(|| view! { <Badge text=HAS_CONFLICTS.get() variant="error" /> })}
                    <ExternalLink href=merge_request.web_url>{open_request_label}</ExternalLink>
                </div>
            </div>
            {match status.pipeline {
                Some(pipeline) => view! { <PipelineView pipeline provider=provider.clone() /> }.into_any(),
                None => view! { <p class="muted text-sm">{NO_PIPELINE.get()}</p> }.into_any(),
            }}
            {open
                .then(|| {
                    view! {
                        <Show
                            when=move || confirming.get()
                            fallback=move || {
                                view! {
                                    <button
                                        type="button"
                                        class="btn-primary"
                                        disabled=move || busy.get()
                                        class:is-busy=move || busy.get()
                                        on:click=move |_| confirming.set(true)
                                    >
                                        {merge_label.clone()}
                                    </button>
                                }
                            }
                        >
                            <ConfirmPanel
                                message=merge_confirm.clone()
                                confirm_label=MERGE.get()
                                pending=busy
                                on_confirm
                                on_cancel
                            />
                        </Show>
                    }
                })}
            {open
                .then(|| {
                    view! {
                        <div class="row items-center gap-2">
                            <button
                                type="button"
                                class="btn-secondary"
                                disabled=move || busy.get()
                                class:is-busy=move || state.push.pending().get()
                                on:click=on_push
                            >
                                {PUSH_BRANCH.get()}
                            </button>
                            <Show when=move || pushed.get()>
                                <span class="text-sm text-ok">{PUSH_DONE.get()}</span>
                            </Show>
                        </div>
                    }
                })}
            <ActionError result=state.push.value().into() />
            <ActionError result=state.merge.value().into() />
        </div>
    }
}

#[component]
fn CreateMergeRequestView(state: TaskMergeRequestState) -> impl IntoView {
    let busy: Signal<bool> = state.busy();
    let on_create = move |_| {
        let title: String = state.title.get_untracked();
        state.create.dispatch(CreateTaskMergeRequest {
            input: CreateMergeRequestDto {
                task_id: state.task_id.get_value(),
                title: Some(title).filter(|t: &String| !t.trim().is_empty()),
                description: None,
            },
        });
    };
    view! {
        <div class="stack gap-3">
            <p class="muted text-sm">{NO_MR_YET.get()}</p>
            <TextField
                label=TITLE_OPTIONAL.get()
                value=state.title
            />
            <p class="muted text-xs">{NO_PUSH_NOTE.get()}</p>
            <div>
                <button
                    type="button"
                    class="btn-primary"
                    disabled=move || busy.get()
                    class:is-busy=move || busy.get()
                    on:click=on_create
                >
                    {CREATE_MR.get()}
                </button>
            </div>
            <ActionError result=state.create.value().into() />
        </div>
    }
}

/// Panel de MR/PR de una tarea (GitLab o GitHub según su origin): crear, ver estado y CI, y mezclar.
#[component]
pub fn TaskMergeRequestPanel(#[prop(into)] task_id: String) -> impl IntoView {
    let state: TaskMergeRequestState = TaskMergeRequestState::new(task_id);
    view! {
        <div class="stack gap-3">
            <div class="row items-center justify-between">
                <span class="action-section">{PANEL_TITLE.get()}</span>
                <button
                    type="button"
                    class="btn-secondary min-h-8 px-3 py-1 text-xs"
                    on:click=move |_| state.refresh()
                >
                    {REFRESH.get()}
                </button>
            </div>
            <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
                {move || Suspend::new(async move {
                    match state.status.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(Some(status)) => {
                            view! { <MergeRequestView state status /> }.into_any()
                        }
                        Ok(None) => view! { <CreateMergeRequestView state /> }.into_any(),
                    }
                })}
            </Transition>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn ci_statuses_map_to_semantic_variants() {
        assert_eq!(status_variant("success"), "ok");
        assert_eq!(status_variant("failed"), "error");
        assert_eq!(status_variant("running"), "info");
        assert_eq!(status_variant("manual"), "warn");
        assert_eq!(status_variant("canceled"), "muted");
        assert_eq!(status_variant("algo-nuevo"), "muted");
    }

    #[test]
    fn labels_are_translated_and_unknown_statuses_pass_through() {
        assert_eq!(status_label("failed"), "failed");
        assert_eq!(status_label("merged"), "merged");
        assert_eq!(STATUS_FAILED.in_locale(Locale::Es), "fallido");
        assert_eq!(status_label("algo-nuevo"), "algo-nuevo");
    }
}
