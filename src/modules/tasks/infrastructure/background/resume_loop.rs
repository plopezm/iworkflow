use std::sync::Arc;
use std::time::Duration;

use chrono::Utc;

use crate::modules::tasks::application::usecases::resume_rate_limited_tasks::ResumeRateLimitedTasks;

/// Cada cuánto se comprueba si alguna tarea parada por límite de uso debe reanudarse.
pub const RESUME_TICK: Duration = Duration::from_secs(30);

/// Bucle de fondo: reanuda las tareas con límite de uso vencido. No termina por sí
/// solo; quien lo lanza conserva el `JoinHandle` y lo aborta al apagar.
pub async fn run_resume_loop(use_case: Arc<ResumeRateLimitedTasks>) {
    let mut ticker: tokio::time::Interval = tokio::time::interval(RESUME_TICK);
    ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    loop {
        ticker.tick().await;
        match use_case.execute(Utc::now()).await {
            Ok(0) => {}
            Ok(resumed) => tracing::info!("{resumed} tareas reanudadas tras el límite de uso"),
            Err(error) => tracing::warn!(%error, "fallo al reanudar tareas con límite de uso"),
        }
    }
}
