use leptos::prelude::*;
use leptos::web_sys::FormData;

use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::{
    AgentsFileDto, ApplyAgentsFileDto, ApplyAgentsFileOutcomeDto, ProjectOptionDto,
};
use crate::modules::agents_files::infrastructure::adapters::inbound::server_fns::{
    ApplyAgentsFile, DeleteAgentsFile, list_agents_file_projects, list_agents_files,
    upload_agents_file,
};

/// Resultado de añadir un AGENTS.md a un proyecto, listo para pintar.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ApplyFeedback {
    Committed,
    NotCommitted,
    TargetExists,
}

impl ApplyFeedback {
    pub fn from_outcome(outcome: &ApplyAgentsFileOutcomeDto) -> Self {
        match outcome {
            ApplyAgentsFileOutcomeDto::Applied { committed: true } => Self::Committed,
            ApplyAgentsFileOutcomeDto::Applied { committed: false } => Self::NotCommitted,
            ApplyAgentsFileOutcomeDto::TargetExists => Self::TargetExists,
        }
    }

    pub fn message(self) -> &'static str {
        match self {
            Self::Committed => "Añadido y commiteado",
            Self::NotCommitted => "Añadido (no se pudo crear el commit; queda sin commitear)",
            Self::TargetExists => "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?",
        }
    }
}

/// Estado de la pantalla de ficheros AGENTS.md: listado, subida, borrado y aplicación a proyectos.
#[derive(Clone, Copy)]
pub struct AgentsFilesState {
    pub agents_files: Resource<Result<Vec<AgentsFileDto>, ServerFnError>>,
    pub projects: Resource<Result<Vec<ProjectOptionDto>, ServerFnError>>,
    pub upload: Action<FormData, Result<AgentsFileDto, ServerFnError>>,
    pub delete: ServerAction<DeleteAgentsFile>,
    pub apply: ServerAction<ApplyAgentsFile>,
    pub description: RwSignal<String>,
    pub form_ref: NodeRef<leptos::html::Form>,
    dismissed: RwSignal<bool>,
    /// Último envío de `apply`; la confirmación de sobrescritura reenvía este.
    last_apply: StoredValue<Option<ApplyAgentsFileDto>>,
    /// Fichero afectado por el último `apply` y qué mostrar; `None` mientras hay una
    /// petición en curso o tras descartarlo.
    pub apply_feedback: Memo<Option<(String, ApplyFeedback)>>,
}

impl AgentsFilesState {
    pub fn new() -> Self {
        let upload: Action<FormData, Result<AgentsFileDto, ServerFnError>> =
            Action::new_local(|data: &FormData| upload_agents_file(data.clone().into()));
        let delete: ServerAction<DeleteAgentsFile> = ServerAction::<DeleteAgentsFile>::new();
        let apply: ServerAction<ApplyAgentsFile> = ServerAction::<ApplyAgentsFile>::new();
        let agents_files: Resource<Result<Vec<AgentsFileDto>, ServerFnError>> = Resource::new(
            move || (upload.version().get(), delete.version().get()),
            |_| list_agents_files(),
        );
        let projects: Resource<Result<Vec<ProjectOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_agents_file_projects());
        let dismissed: RwSignal<bool> = RwSignal::new(false);
        let last_apply: StoredValue<Option<ApplyAgentsFileDto>> = StoredValue::new(None);
        let apply_feedback: Memo<Option<(String, ApplyFeedback)>> = Memo::new(move |_| {
            if dismissed.get() || apply.pending().get() {
                return None;
            }
            match apply.value().get() {
                Some(Ok(outcome)) => last_apply.get_value().map(|input: ApplyAgentsFileDto| {
                    (input.agents_file_id, ApplyFeedback::from_outcome(&outcome))
                }),
                _ => None,
            }
        });
        Self {
            agents_files,
            projects,
            upload,
            delete,
            apply,
            description: RwSignal::new(String::new()),
            form_ref: NodeRef::new(),
            dismissed,
            last_apply,
            apply_feedback,
        }
    }

    pub fn submit_upload(&self, data: FormData) {
        self.upload.dispatch_local(data);
    }

    /// Limpia descripción y selector de fichero (se invoca tras una subida correcta).
    pub fn reset_form(&self) {
        self.description.set(String::new());
        if let Some(form) = self.form_ref.get_untracked() {
            form.reset();
        }
    }

    pub fn remove(&self, id: String) {
        self.delete.dispatch(DeleteAgentsFile { id });
    }

    pub fn apply_to_project(&self, agents_file_id: String, project_id: String) {
        self.send_apply(ApplyAgentsFileDto {
            agents_file_id,
            project_id,
            overwrite: false,
        });
    }

    /// Reenvía exactamente el último envío con `overwrite: true`.
    pub fn confirm_overwrite(&self) {
        let Some(last) = self.last_apply.get_value() else {
            return;
        };
        self.send_apply(ApplyAgentsFileDto {
            overwrite: true,
            ..last
        });
    }

    pub fn dismiss_feedback(&self) {
        self.dismissed.set(true);
    }

    /// Feedback del último `apply` solo si corresponde al fichero `id`.
    pub fn feedback_for(&self, id: String) -> Signal<Option<ApplyFeedback>> {
        let feedback: Memo<Option<(String, ApplyFeedback)>> = self.apply_feedback;
        Signal::derive(move || {
            feedback
                .get()
                .filter(|(file_id, _): &(String, ApplyFeedback)| *file_id == id)
                .map(|(_, f): (String, ApplyFeedback)| f)
        })
    }

    fn send_apply(&self, input: ApplyAgentsFileDto) {
        self.last_apply.set_value(Some(input.clone()));
        self.dismissed.set(false);
        self.apply.dispatch(ApplyAgentsFile { input });
    }
}

impl Default for AgentsFilesState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn maps_applied_committed() {
        let feedback: ApplyFeedback =
            ApplyFeedback::from_outcome(&ApplyAgentsFileOutcomeDto::Applied { committed: true });

        assert_eq!(feedback, ApplyFeedback::Committed);
        assert_eq!(feedback.message(), "Añadido y commiteado");
    }

    #[test]
    fn maps_applied_without_commit() {
        let feedback: ApplyFeedback =
            ApplyFeedback::from_outcome(&ApplyAgentsFileOutcomeDto::Applied { committed: false });

        assert_eq!(feedback, ApplyFeedback::NotCommitted);
        assert_eq!(
            feedback.message(),
            "Añadido (no se pudo crear el commit; queda sin commitear)"
        );
    }

    #[test]
    fn maps_target_exists_to_overwrite_prompt() {
        let feedback: ApplyFeedback =
            ApplyFeedback::from_outcome(&ApplyAgentsFileOutcomeDto::TargetExists);

        assert_eq!(feedback, ApplyFeedback::TargetExists);
        assert_eq!(
            feedback.message(),
            "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?"
        );
    }
}
