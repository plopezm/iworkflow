//! Terminación de árboles de procesos en Windows, donde no hay señales ni `/proc`.

#[cfg(windows)]
use std::process::{Command, Stdio};

/// Evita que `taskkill` abra una ventana de consola.
#[cfg(windows)]
const CREATE_NO_WINDOW: u32 = 0x0800_0000;

/// Argumentos de `taskkill` para matar `pid` y todos sus descendientes a la fuerza.
#[cfg(any(windows, test))]
fn taskkill_args(pid: u32) -> Vec<String> {
    vec![
        "/PID".to_string(),
        pid.to_string(),
        "/T".to_string(),
        "/F".to_string(),
    ]
}

/// Mata `pid` y todos sus descendientes. El proceso puede haber terminado ya: se ignora el fallo.
#[cfg(windows)]
pub fn kill_tree(pid: u32) {
    use std::os::windows::process::CommandExt;

    let _ = Command::new("taskkill")
        .args(taskkill_args(pid))
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .creation_flags(CREATE_NO_WINDOW)
        .status();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn taskkill_args_kill_the_whole_tree_by_pid() {
        assert_eq!(taskkill_args(42), vec!["/PID", "42", "/T", "/F"]);
    }
}
