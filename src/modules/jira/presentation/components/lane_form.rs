use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, TextArea, TextField};
use crate::core::i18n::Msg;
use crate::modules::jira::presentation::state::jira_issues_state::{
    JiraIssuesState, LaneFormMode, can_save_lane,
};

const EDIT_LANE: Msg = Msg {
    en: "Edit lane",
    es: "Editar carril",
    zh: "编辑泳道",
    de: "Lane bearbeiten",
    fr: "Modifier la voie",
};

const NEW_LANE: Msg = Msg {
    en: "New lane",
    es: "Nuevo carril",
    zh: "新建泳道",
    de: "Neue Lane",
    fr: "Nouvelle voie",
};

const NAME: Msg = Msg {
    en: "Name",
    es: "Nombre",
    zh: "名称",
    de: "Name",
    fr: "Nom",
};

const NAME_PLACEHOLDER: Msg = Msg {
    en: "Backend, Critical bugs, Pending reviews…",
    es: "Backend, Bugs críticos, Revisiones pendientes…",
    zh: "后端、严重缺陷、待审核…",
    de: "Backend, Kritische Bugs, Ausstehende Reviews…",
    fr: "Backend, Bugs critiques, Revues en attente…",
};

const JQL_LABEL: Msg = Msg {
    en: "JQL query",
    es: "Consulta JQL",
    zh: "JQL 查询",
    de: "JQL-Abfrage",
    fr: "Requête JQL",
};

const JQL_HINT: Msg = Msg {
    en: "The lane's issues are fetched from Jira with this query every time you sync.",
    es: "Las tareas del carril se traen de Jira con esta consulta cada vez que sincronizas.",
    zh: "每次同步时，都会使用此查询从 Jira 获取该泳道的任务。",
    de: "Die Aufgaben der Lane werden bei jeder Synchronisierung mit dieser Abfrage aus Jira geladen.",
    fr: "Les tâches de la voie sont récupérées depuis Jira avec cette requête à chaque synchronisation.",
};

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

const SAVING: Msg = Msg {
    en: "Saving and syncing…",
    es: "Guardando y sincronizando…",
    zh: "正在保存并同步…",
    de: "Wird gespeichert und synchronisiert…",
    fr: "Enregistrement et synchronisation…",
};

const SAVE_CHANGES: Msg = Msg {
    en: "Save changes",
    es: "Guardar cambios",
    zh: "保存更改",
    de: "Änderungen speichern",
    fr: "Enregistrer les modifications",
};

const CREATE_LANE: Msg = Msg {
    en: "Create lane",
    es: "Crear carril",
    zh: "创建泳道",
    de: "Lane erstellen",
    fr: "Créer la voie",
};

/// Name + JQL of a custom lane; shown when creating a lane or editing an existing one.
#[component]
pub fn LaneForm(state: JiraIssuesState) -> impl IntoView {
    let editing: Memo<bool> =
        Memo::new(move |_| matches!(state.lane_form.get(), LaneFormMode::Editing(_)));
    let pending: Signal<bool> = Signal::derive(move || {
        state.create_lane.pending().get() || state.update_lane.pending().get()
    });
    let submittable: Memo<bool> =
        Memo::new(move |_| can_save_lane(&state.lane_name.get(), &state.lane_jql.get()));
    view! {
        <form
            class="card stack animate-slide-down border-accent/40"
            on:submit=move |ev: SubmitEvent| {
                ev.prevent_default();
                state.save_lane();
            }
        >
            <h2 class="card-title">
                {move || if editing.get() { EDIT_LANE.get() } else { NEW_LANE.get() }}
            </h2>
            <TextField
                label=NAME.get()
                value=state.lane_name
                placeholder=NAME_PLACEHOLDER.get()
                required=true
            />
            <TextArea
                label=JQL_LABEL.get()
                value=state.lane_jql
                placeholder="project = ABC AND priority = High AND statusCategory != Done ORDER BY updated DESC"
                required=true
            />
            <p class="muted text-xs">
                {JQL_HINT.get()}
            </p>
            <ActionError result=state.create_lane.value().into() />
            <ActionError result=state.update_lane.value().into() />
            <div class="row justify-end gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| state.cancel_lane_form()
                >
                    {CANCEL.get()}
                </button>
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !submittable.get()
                    class:is-busy=move || pending.get()
                >
                    {move || {
                        if pending.get() {
                            SAVING.get()
                        } else if editing.get() {
                            SAVE_CHANGES.get()
                        } else {
                            CREATE_LANE.get()
                        }
                    }}
                </button>
            </div>
        </form>
    }
}
