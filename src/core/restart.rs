//! Restart of the server process, requested from the UI.
//!
//! The server shuts down gracefully and then either hands over to its supervisor (the desktop
//! shell, which respawns it) or starts a fresh copy of itself.

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use tokio::sync::Notify;

/// Exit code that tells a supervisor "restart me" (`EX_TEMPFAIL`).
pub const RESTART_EXIT_CODE: i32 = 75;
/// Set by a supervisor that respawns the server when it exits with [`RESTART_EXIT_CODE`].
pub const SUPERVISED_VAR: &str = "IWF_SUPERVISED";
/// Value the desktop shell gives [`SUPERVISED_VAR`].
const SUPERVISED_VALUE: &str = "1";
const NO_OPEN_FLAG: &str = "--no-open";

/// Shared flag between the server function that asks for a restart and `main`, which owns the
/// shutdown.
#[derive(Clone, Default)]
pub struct RestartHandle {
    notify: Arc<Notify>,
    requested: Arc<AtomicBool>,
}

impl RestartHandle {
    /// Asks the server to shut down and start again. Safe to call more than once.
    pub fn request(&self) {
        self.requested.store(true, Ordering::SeqCst);
        self.notify.notify_one();
    }

    pub fn is_requested(&self) -> bool {
        self.requested.load(Ordering::SeqCst)
    }

    /// Resolves once a restart was requested, even if that happened before the call.
    pub async fn requested(&self) {
        self.notify.notified().await;
    }
}

/// Whether a supervisor will respawn the server after it exits with [`RESTART_EXIT_CODE`].
pub fn is_supervised() -> bool {
    supervised_from(std::env::var(SUPERVISED_VAR).ok().as_deref())
}

/// Only the value the shell sets counts, so `IWF_SUPERVISED=0` or an empty value does not.
fn supervised_from(value: Option<&str>) -> bool {
    value == Some(SUPERVISED_VALUE)
}

/// What `main` does once the server has shut down.
#[derive(Debug, PartialEq, Eq)]
pub enum AfterShutdown {
    /// Plain shutdown.
    Stop,
    /// Exit with [`RESTART_EXIT_CODE`] so the supervisor starts the server again.
    HandOver,
    /// Start a fresh copy of this executable.
    Respawn,
}

pub fn after_shutdown(restart_requested: bool, supervised: bool) -> AfterShutdown {
    match (restart_requested, supervised) {
        (false, _) => AfterShutdown::Stop,
        (true, true) => AfterShutdown::HandOver,
        (true, false) => AfterShutdown::Respawn,
    }
}

/// Arguments for the new process: the current ones, never reopening the browser tab.
pub fn respawn_args(args: &[String]) -> Vec<String> {
    let mut next: Vec<String> = args.to_vec();
    if !next.iter().any(|arg: &String| arg == NO_OPEN_FLAG) {
        next.push(NO_OPEN_FLAG.to_string());
    }
    next
}

/// Starts a detached copy of this executable. Call it only after the listener is closed.
pub fn respawn_self(args: &[String]) -> std::io::Result<()> {
    std::process::Command::new(std::env::current_exe()?)
        .args(respawn_args(args))
        .spawn()
        .map(drop)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_the_shell_value_marks_the_server_as_supervised() {
        assert!(supervised_from(Some("1")));
        assert!(!supervised_from(Some("0")));
        assert!(!supervised_from(Some("")));
        assert!(!supervised_from(None));
    }

    #[test]
    fn shutdown_outcome_depends_on_the_request_and_the_supervisor() {
        assert_eq!(after_shutdown(false, true), AfterShutdown::Stop);
        assert_eq!(after_shutdown(false, false), AfterShutdown::Stop);
        assert_eq!(after_shutdown(true, true), AfterShutdown::HandOver);
        assert_eq!(after_shutdown(true, false), AfterShutdown::Respawn);
    }

    #[test]
    fn respawn_never_reopens_the_browser() {
        assert_eq!(respawn_args(&[]), vec!["--no-open".to_string()]);
    }

    #[test]
    fn respawn_keeps_an_existing_no_open_flag_once() {
        let args: Vec<String> = vec!["--no-open".to_string()];
        assert_eq!(respawn_args(&args), args);
    }

    #[tokio::test]
    async fn a_request_made_before_waiting_is_not_lost() {
        let handle: RestartHandle = RestartHandle::default();
        assert!(!handle.is_requested());
        handle.request();
        handle.request();
        assert!(handle.is_requested());
        tokio::time::timeout(std::time::Duration::from_secs(1), handle.requested())
            .await
            .expect("la petición pendiente despierta al servidor");
    }
}
