use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    MessageModeDto, TaskDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::DialogAnswerDto;
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    AbortTask, AnswerTaskDialog, DeleteTask, DiscardTask, FinishTask, MergeTask, ResetTask,
    ResumeTask, SendTaskMessage, StartTask, get_task, get_task_diff,
};
use crate::modules::tasks::presentation::state::image_attachments::ImageAttachments;
use crate::modules::tasks::presentation::state::transcript::Transcript;

/// Espera antes de reabrir el stream SSE cuando el navegador lo da por cerrado.
#[cfg(feature = "hydrate")]
const SSE_RECONNECT_DELAY: std::time::Duration = std::time::Duration::from_secs(3);

/// Estado de la pantalla de detalle de una tarea.
#[derive(Clone, Copy)]
pub struct TaskDetailState {
    pub task_id: StoredValue<String>,
    pub task: Resource<Result<TaskDto, ServerFnError>>,
    pub transcript: RwSignal<Transcript>,
    pub connected: RwSignal<bool>,
    pub show_diff: RwSignal<bool>,
    pub diff: Resource<Option<Result<TaskDiffDto, ServerFnError>>>,
    pub message: RwSignal<String>,
    pub mode: RwSignal<MessageModeDto>,
    /// Imágenes adjuntas al próximo mensaje.
    pub images: ImageAttachments,
    pub send: ServerAction<SendTaskMessage>,
    pub answer: ServerAction<AnswerTaskDialog>,
    pub abort: ServerAction<AbortTask>,
    pub finish: ServerAction<FinishTask>,
    pub resume: ServerAction<ResumeTask>,
    pub merge: ServerAction<MergeTask>,
    pub discard: ServerAction<DiscardTask>,
    pub reset: ServerAction<ResetTask>,
    pub start: ServerAction<StartTask>,
    pub delete: ServerAction<DeleteTask>,
    /// Se incrementa con cada cambio de estado o edición recibido por SSE.
    pub status_version: RwSignal<u32>,
    /// Se marca cuando la tarea se elimina desde el tablero mientras se ve el detalle.
    pub removed: RwSignal<bool>,
}

/// Construye el envío de un mensaje; `None` si no hay texto ni imágenes (función pura).
pub fn build_send_message(
    id: String,
    message: String,
    images: Vec<ImageAttachmentDto>,
    mode: MessageModeDto,
) -> Option<SendTaskMessage> {
    if message.trim().is_empty() && images.is_empty() {
        return None;
    }
    Some(SendTaskMessage {
        id,
        message,
        images,
        mode,
    })
}

impl TaskDetailState {
    pub fn new(task_id: String) -> Self {
        let status_version: RwSignal<u32> = RwSignal::new(0);
        let send: ServerAction<SendTaskMessage> = ServerAction::new();
        let answer: ServerAction<AnswerTaskDialog> = ServerAction::new();
        let abort: ServerAction<AbortTask> = ServerAction::new();
        let finish: ServerAction<FinishTask> = ServerAction::new();
        let resume: ServerAction<ResumeTask> = ServerAction::new();
        let merge: ServerAction<MergeTask> = ServerAction::new();
        let discard: ServerAction<DiscardTask> = ServerAction::new();
        let reset: ServerAction<ResetTask> = ServerAction::new();
        let start: ServerAction<StartTask> = ServerAction::new();
        let delete: ServerAction<DeleteTask> = ServerAction::new();
        let show_diff: RwSignal<bool> = RwSignal::new(false);
        let id: StoredValue<String> = StoredValue::new(task_id);

        let task: Resource<Result<TaskDto, ServerFnError>> = Resource::new(
            move || {
                (
                    status_version.get(),
                    finish.version().get(),
                    resume.version().get(),
                    merge.version().get(),
                    discard.version().get(),
                    reset.version().get(),
                    start.version().get(),
                )
            },
            move |_| get_task(id.get_value()),
        );
        // El diff depende de la tarea ya refrescada (no de los eventos): solo se pide si
        // está visible y el estado lo permite (tras merge o descarte no hay worktree).
        let diff: Resource<Option<Result<TaskDiffDto, ServerFnError>>> = Resource::new(
            move || {
                let current: Option<(bool, String)> = task
                    .get()
                    .and_then(|r: Result<TaskDto, ServerFnError>| r.ok())
                    .map(|t: TaskDto| (t.actions.can_view_diff, t.updated_at));
                let allowed: bool = current.as_ref().is_some_and(|(can, _)| *can);
                (
                    show_diff.get() && allowed,
                    current.map(|(_, updated)| updated),
                )
            },
            move |(visible, _)| async move {
                if visible {
                    Some(get_task_diff(id.get_value()).await)
                } else {
                    None
                }
            },
        );

        let state: Self = Self {
            task_id: id,
            task,
            transcript: RwSignal::new(Transcript::default()),
            connected: RwSignal::new(false),
            show_diff,
            diff,
            message: RwSignal::new(String::new()),
            mode: RwSignal::new(MessageModeDto::Steer),
            images: ImageAttachments::new(),
            send,
            answer,
            abort,
            finish,
            resume,
            merge,
            discard,
            reset,
            start,
            delete,
            status_version,
            removed: RwSignal::new(false),
        };
        // Tras restablecer, el historial local ya no existe en el servidor.
        Effect::new(move |_| {
            if matches!(reset.value().get(), Some(Ok(_))) {
                state.transcript.set(Transcript::default());
            }
        });
        state.connect_events();
        state
    }

    pub fn send_message(&self) {
        let message: String = self.message.get_untracked();
        let images: Vec<ImageAttachmentDto> = self.images.images.get_untracked();
        let Some(action) = build_send_message(
            self.task_id.get_value(),
            message,
            images,
            self.mode.get_untracked(),
        ) else {
            return;
        };
        self.send.dispatch(action);
        self.message.set(String::new());
        self.images.clear();
    }

    pub fn answer_dialog(&self, dialog_id: String, answer: DialogAnswerDto) {
        self.answer.dispatch(AnswerTaskDialog {
            id: self.task_id.get_value(),
            dialog_id,
            answer,
        });
    }

    pub fn abort(&self) {
        self.abort.dispatch(AbortTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn finish(&self) {
        self.finish.dispatch(FinishTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn resume(&self) {
        self.resume.dispatch(ResumeTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn merge(&self) {
        self.merge.dispatch(MergeTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn discard(&self) {
        self.discard.dispatch(DiscardTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn reset(&self) {
        self.reset.dispatch(ResetTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn start(&self) {
        self.start.dispatch(StartTask {
            id: self.task_id.get_value(),
        });
    }

    pub fn delete(&self) {
        self.delete.dispatch(DeleteTask {
            id: self.task_id.get_value(),
        });
    }

    /// Primer error pendiente de las acciones de la pantalla.
    pub fn action_error(&self) -> Option<String> {
        fn error_of<T>(value: Option<Result<T, ServerFnError>>) -> Option<String> {
            value
                .and_then(|r: Result<T, ServerFnError>| r.err())
                .map(|e: ServerFnError| e.to_string())
        }
        error_of(self.send.value().get())
            .or_else(|| error_of(self.answer.value().get()))
            .or_else(|| error_of(self.abort.value().get()))
            .or_else(|| error_of(self.finish.value().get()))
            .or_else(|| error_of(self.resume.value().get()))
            .or_else(|| error_of(self.merge.value().get()))
            .or_else(|| error_of(self.discard.value().get()))
            .or_else(|| error_of(self.reset.value().get()))
            .or_else(|| error_of(self.start.value().get()))
            .or_else(|| error_of(self.delete.value().get()))
    }

    #[cfg(not(feature = "hydrate"))]
    fn connect_events(&self) {}

    /// Abre el stream SSE de la tarea y lo cierra al desmontar la pantalla.
    #[cfg(feature = "hydrate")]
    fn connect_events(&self) {
        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;
        use web_sys::{Event, EventSource, MessageEvent};

        use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
            TaskEventDto, TaskEventKindDto,
        };

        type Handler = Closure<dyn FnMut(Event)>;

        struct Connection {
            source: EventSource,
            _handlers: Vec<Handler>,
        }

        #[derive(Default)]
        struct SseState {
            connection: Option<Connection>,
            reconnect: Option<TimeoutHandle>,
        }

        /// Cierra la conexión actual y cancela cualquier reapertura pendiente.
        fn teardown(slot: StoredValue<SseState, LocalStorage>) {
            slot.update_value(|state: &mut SseState| {
                if let Some(handle) = state.reconnect.take() {
                    handle.clear();
                }
                if let Some(connection) = state.connection.take() {
                    connection.source.close();
                }
            });
        }

        /// Programa la reapertura fuera del handler en curso: soltar aquí el `Closure` que
        /// se está ejecutando provocaría "closure invoked recursively or after being dropped".
        fn schedule_reopen(
            delay: std::time::Duration,
            url: String,
            signals: Signals,
            slot: StoredValue<SseState, LocalStorage>,
        ) {
            slot.update_value(|state: &mut SseState| {
                if let Some(handle) = state.reconnect.take() {
                    handle.clear();
                }
            });
            let handle: Option<TimeoutHandle> = set_timeout_with_handle(
                move || {
                    teardown(slot);
                    open(&url, signals, slot);
                },
                delay,
            )
            .ok();
            slot.update_value(|state: &mut SseState| state.reconnect = handle);
        }

        #[derive(Clone, Copy)]
        struct Signals {
            transcript: RwSignal<Transcript>,
            connected: RwSignal<bool>,
            status_version: RwSignal<u32>,
            removed: RwSignal<bool>,
        }

        fn open(url: &str, signals: Signals, slot: StoredValue<SseState, LocalStorage>) {
            let Signals {
                transcript,
                connected,
                status_version,
                removed,
            } = signals;
            let Ok(source) = EventSource::new(url) else {
                connected.set(false);
                return;
            };
            let on_message: Handler = Closure::new(move |event: Event| {
                let Some(data) = event
                    .dyn_ref::<MessageEvent>()
                    .and_then(|m: &MessageEvent| m.data().as_string())
                else {
                    return;
                };
                if let Ok(event) = serde_json::from_str::<TaskEventDto>(&data) {
                    match event.kind {
                        TaskEventKindDto::Status { .. } | TaskEventKindDto::Changed => {
                            status_version.update(|v: &mut u32| *v += 1);
                        }
                        TaskEventKindDto::Removed => removed.set(true),
                        TaskEventKindDto::Agent { .. } => {}
                    }
                    transcript.update(|t: &mut Transcript| t.apply(event));
                }
            });
            let on_reset: Handler = Closure::new(move |_: Event| {
                transcript.set(Transcript::default());
                connected.set(true);
            });
            let watched_source: EventSource = source.clone();
            let error_url: String = url.to_string();
            let on_error: Handler = Closure::new(move |_: Event| {
                connected.set(false);
                // Los cortes recuperables los reintenta el propio `EventSource`; si queda
                // CLOSED hay que reabrirlo a mano.
                if watched_source.ready_state() == EventSource::CLOSED {
                    schedule_reopen(SSE_RECONNECT_DELAY, error_url.clone(), signals, slot);
                }
            });
            let lagged_url: String = url.to_string();
            let on_lagged: Handler = Closure::new(move |_: Event| {
                // Se perdieron eventos en vivo: se reabre para recibir el historial completo.
                schedule_reopen(std::time::Duration::ZERO, lagged_url.clone(), signals, slot);
            });
            source.set_onmessage(Some(on_message.as_ref().unchecked_ref()));
            source.set_onerror(Some(on_error.as_ref().unchecked_ref()));
            let listeners: [(&str, &Handler); 2] = [("reset", &on_reset), ("lagged", &on_lagged)];
            for (name, handler) in listeners {
                if source
                    .add_event_listener_with_callback(name, handler.as_ref().unchecked_ref())
                    .is_err()
                {
                    connected.set(false);
                }
            }
            slot.update_value(|state: &mut SseState| {
                state.connection = Some(Connection {
                    source,
                    _handlers: vec![on_message, on_reset, on_error, on_lagged],
                });
            });
        }

        let slot: StoredValue<SseState, LocalStorage> = StoredValue::new_local(SseState::default());
        let url: String = format!("/api/tasks/{}/events", self.task_id.get_value());
        let signals: Signals = Signals {
            transcript: self.transcript,
            connected: self.connected,
            status_version: self.status_version,
            removed: self.removed,
        };
        open(&url, signals, slot);
        on_cleanup(move || teardown(slot));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn image() -> ImageAttachmentDto {
        ImageAttachmentDto {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        }
    }

    #[test]
    fn send_requires_text_or_images() {
        assert!(
            build_send_message("t".into(), "  ".into(), vec![], MessageModeDto::Steer).is_none()
        );
        assert!(
            build_send_message("t".into(), "hola".into(), vec![], MessageModeDto::Steer).is_some()
        );
    }

    #[test]
    fn send_with_only_images_carries_them() {
        let action: Option<SendTaskMessage> = build_send_message(
            "t".into(),
            String::new(),
            vec![image()],
            MessageModeDto::FollowUp,
        );
        let action: SendTaskMessage = action.expect("images alone are enough");
        assert_eq!(action.images, vec![image()]);
        assert_eq!(action.id, "t");
    }
}
