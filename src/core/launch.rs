//! Opciones de línea de comandos del ejecutable y apertura del navegador al arrancar.

use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};
use std::process::Command;

const NO_OPEN_FLAG: &str = "--no-open";
const HELP_FLAGS: [&str; 2] = ["-h", "--help"];

/// Texto de ayuda mostrado con `--help`.
pub const USAGE: &str = "Uso: iworkflow [OPCIONES]\n\n\
Opciones:\n  \
--no-open    Arranca solo el servidor, sin abrir la web en el navegador\n  \
-h, --help   Muestra esta ayuda";

#[derive(Debug, thiserror::Error, PartialEq, Eq)]
pub enum LaunchError {
    #[error("argumento desconocido: {0}\n\n{USAGE}")]
    UnknownArgument(String),
    #[error("no se pudo abrir el navegador: {0}")]
    BrowserFailed(String),
}

/// Qué se pide hacer al arrancar.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LaunchCommand {
    /// Arrancar el servidor con estas opciones.
    Run(LaunchOptions),
    /// Mostrar la ayuda y salir.
    Help,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LaunchOptions {
    /// Si se abre la web en el navegador una vez el servidor escucha.
    pub open_browser: bool,
}

impl LaunchOptions {
    /// Interpreta los argumentos (sin el nombre del ejecutable). Por defecto se abre la web.
    pub fn parse<I, S>(args: I) -> Result<LaunchCommand, LaunchError>
    where
        I: IntoIterator<Item = S>,
        S: AsRef<str>,
    {
        let mut open_browser: bool = true;
        for arg in args {
            let arg: &str = arg.as_ref();
            if arg == NO_OPEN_FLAG {
                open_browser = false;
            } else if HELP_FLAGS.contains(&arg) {
                return Ok(LaunchCommand::Help);
            } else {
                return Err(LaunchError::UnknownArgument(arg.to_string()));
            }
        }
        Ok(LaunchCommand::Run(Self { open_browser }))
    }
}

/// URL local desde la que abrir la web: una dirección no especificada (`0.0.0.0`, `::`)
/// no es navegable, así que se sustituye por loopback.
pub fn browser_url(addr: SocketAddr) -> String {
    let ip: IpAddr = match addr.ip() {
        IpAddr::V4(v4) if v4.is_unspecified() => IpAddr::V4(Ipv4Addr::LOCALHOST),
        IpAddr::V6(v6) if v6.is_unspecified() => IpAddr::V6(Ipv6Addr::LOCALHOST),
        other => other,
    };
    format!("http://{}", SocketAddr::new(ip, addr.port()))
}

/// Abre `url` con el lanzador del sistema sin esperar a que el navegador termine.
pub fn open_browser(url: &str) -> Result<(), LaunchError> {
    let mut command: Command = opener_command(url);
    command
        .spawn()
        .map(drop)
        .map_err(|error: std::io::Error| LaunchError::BrowserFailed(error.to_string()))
}

#[cfg(target_os = "windows")]
fn opener_command(url: &str) -> Command {
    let mut command: Command = Command::new("cmd");
    command.args(["/C", "start", "", url]);
    command
}

#[cfg(target_os = "macos")]
fn opener_command(url: &str) -> Command {
    let mut command: Command = Command::new("open");
    command.arg(url);
    command
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
fn opener_command(url: &str) -> Command {
    let mut command: Command = Command::new("xdg-open");
    command.arg(url);
    command
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn opens_the_browser_by_default() {
        let command: LaunchCommand = LaunchOptions::parse(Vec::<String>::new()).unwrap();
        assert_eq!(
            command,
            LaunchCommand::Run(LaunchOptions { open_browser: true })
        );
    }

    #[test]
    fn no_open_flag_disables_the_browser() {
        let command: LaunchCommand = LaunchOptions::parse(["--no-open"]).unwrap();
        assert_eq!(
            command,
            LaunchCommand::Run(LaunchOptions {
                open_browser: false
            })
        );
    }

    #[test]
    fn help_flags_request_usage() {
        assert_eq!(LaunchOptions::parse(["-h"]).unwrap(), LaunchCommand::Help);
        assert_eq!(
            LaunchOptions::parse(["--no-open", "--help"]).unwrap(),
            LaunchCommand::Help
        );
    }

    #[test]
    fn unknown_argument_is_rejected() {
        let error: LaunchError = LaunchOptions::parse(["--nope"]).unwrap_err();
        assert_eq!(error, LaunchError::UnknownArgument("--nope".to_string()));
    }

    #[test]
    fn unspecified_addresses_map_to_loopback() {
        let v4: SocketAddr = "0.0.0.0:9000".parse().unwrap();
        let v6: SocketAddr = "[::]:9000".parse().unwrap();
        assert_eq!(browser_url(v4), "http://127.0.0.1:9000");
        assert_eq!(browser_url(v6), "http://[::1]:9000");
    }

    #[test]
    fn specific_addresses_are_kept() {
        let addr: SocketAddr = "127.0.0.1:3000".parse().unwrap();
        assert_eq!(browser_url(addr), "http://127.0.0.1:3000");
    }
}
