use leptos::prelude::*;

use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::MergeRequestStatusDto;
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    CreateTaskMergeRequest, MergeTaskMergeRequest, PushTaskBranch, get_task_merge_request,
};

/// Estado del panel de merge request de una tarea: lectura (MR + CI) y acciones.
#[derive(Clone, Copy)]
pub struct TaskMergeRequestState {
    pub task_id: StoredValue<String>,
    pub status: Resource<Result<Option<MergeRequestStatusDto>, ServerFnError>>,
    pub create: ServerAction<CreateTaskMergeRequest>,
    pub merge: ServerAction<MergeTaskMergeRequest>,
    pub push: ServerAction<PushTaskBranch>,
    /// Título opcional del MR a crear (vacío = el de la tarea).
    pub title: RwSignal<String>,
    refresh: RwSignal<u32>,
}

impl TaskMergeRequestState {
    pub fn new(task_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(task_id);
        let create: ServerAction<CreateTaskMergeRequest> = ServerAction::new();
        let merge: ServerAction<MergeTaskMergeRequest> = ServerAction::new();
        let push: ServerAction<PushTaskBranch> = ServerAction::new();
        let refresh: RwSignal<u32> = RwSignal::new(0);
        let status: Resource<Result<Option<MergeRequestStatusDto>, ServerFnError>> = Resource::new(
            move || (refresh.get(), create.version().get(), merge.version().get()),
            move |_| get_task_merge_request(id.get_value()),
        );
        Self {
            task_id: id,
            status,
            create,
            merge,
            push,
            title: RwSignal::new(String::new()),
            refresh,
        }
    }

    pub fn refresh(&self) {
        self.refresh.update(|n: &mut u32| *n += 1);
    }

    pub fn busy(&self) -> Signal<bool> {
        let create: ServerAction<CreateTaskMergeRequest> = self.create;
        let merge: ServerAction<MergeTaskMergeRequest> = self.merge;
        let push: ServerAction<PushTaskBranch> = self.push;
        Signal::derive(move || {
            create.pending().get() || merge.pending().get() || push.pending().get()
        })
    }
}
