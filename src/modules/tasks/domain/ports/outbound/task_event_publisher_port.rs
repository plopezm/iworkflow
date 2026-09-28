use crate::modules::tasks::domain::models::task_event::TaskEvent;

/// Difunde eventos de tarea a los observadores en vivo.
pub trait TaskEventPublisherPort: Send + Sync {
    fn publish(&self, event: TaskEvent);
}
