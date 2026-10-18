use std::sync::Arc;
use std::time::Duration;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;

/// Cada cuánto se reevalúan los disparadores del grafo (cubre merges hechos fuera de la UI y
/// arranques fallidos que vuelven a `Backlog`).
pub const GRAPH_TICK: Duration = Duration::from_secs(15);

/// Bucle de fondo: arranca las tareas cuyas predecesoras ya han cumplido su disparador. No
/// termina por sí solo; quien lo lanza conserva el `JoinHandle` y lo aborta al apagar.
pub async fn run_graph_loop(trigger: Arc<dyn SuccessorTrigger>) {
    let mut ticker: tokio::time::Interval = tokio::time::interval(GRAPH_TICK);
    ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    loop {
        ticker.tick().await;
        match evaluate_once(trigger.as_ref()).await {
            TickOutcome::Idle => {}
            TickOutcome::Started(started) => {
                tracing::info!("{started} tareas arrancadas por el grafo de dependencias")
            }
            TickOutcome::Failed(error) => {
                tracing::warn!(%error, "fallo al evaluar el grafo de dependencias")
            }
        }
    }
}

/// Resultado de una evaluación del grafo.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TickOutcome {
    /// No había nada que arrancar.
    Idle,
    /// Número de tareas arrancadas (> 0).
    Started(usize),
    /// La evaluación falló; el bucle sigue en el siguiente tick.
    Failed(TaskError),
}

/// Cuerpo de un tick: evalúa los disparadores una vez.
pub async fn evaluate_once(trigger: &dyn SuccessorTrigger) -> TickOutcome {
    match trigger.trigger().await {
        Ok(0) => TickOutcome::Idle,
        Ok(started) => TickOutcome::Started(started),
        Err(error) => TickOutcome::Failed(error),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::MockSuccessorTrigger;

    #[tokio::test]
    async fn no_started_tasks_is_idle() {
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(0));
        assert_eq!(evaluate_once(&trigger).await, TickOutcome::Idle);
    }

    #[tokio::test]
    async fn started_tasks_are_reported() {
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger.expect_trigger().times(1).returning(|| Ok(3));
        assert_eq!(evaluate_once(&trigger).await, TickOutcome::Started(3));
    }

    #[tokio::test]
    async fn errors_are_reported_not_propagated() {
        let mut trigger: MockSuccessorTrigger = MockSuccessorTrigger::new();
        trigger
            .expect_trigger()
            .times(1)
            .returning(|| Err(TaskError::Git("boom".to_string())));
        assert_eq!(
            evaluate_once(&trigger).await,
            TickOutcome::Failed(TaskError::Git("boom".to_string()))
        );
    }
}
