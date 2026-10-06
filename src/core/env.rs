//! Entorno de sesión para procesos hijos (terminal, agentes, catálogo de modelos): parten de un
//! entorno vacío con solo las variables de sesión del sistema y se lanzan por la shell de login.

use std::ffi::OsString;

const FALLBACK_SHELL: &str = "/bin/sh";
/// `PATH` base del sistema. No se hereda el del padre (puede traer entornos virtuales, shells de
/// nix, etc. de la terminal desde la que se lanzó el servidor): los perfiles de la shell lo amplían.
pub const DEFAULT_PATH: &str = "/usr/local/bin:/usr/bin:/bin";
/// Pide a la shell que se comporte como shell de login y cargue los perfiles del sistema y del usuario.
pub const LOGIN_SHELL_FLAG: &str = "-l";
/// Pide a la shell que ejecute el siguiente argumento como comando.
const COMMAND_FLAG: &str = "-c";
const SHELL_KEY: &str = "SHELL";
const PATH_KEY: &str = "PATH";
/// Variables que cualquier sesión de usuario trae por defecto (login, locale, escritorio).
/// Los procesos parten de un entorno limpio con solo estas: lo que el proceso padre (cargo-leptos,
/// iworkflow, etc.) haya añadido no se hereda, para no solapar con los proyectos del usuario.
pub const DEFAULT_ENV_KEYS: [&str; 23] = [
    "HOME",
    "USER",
    "LOGNAME",
    "SHELL",
    "LANG",
    "LANGUAGE",
    "TZ",
    "DISPLAY",
    "WAYLAND_DISPLAY",
    "XAUTHORITY",
    "DBUS_SESSION_BUS_ADDRESS",
    "SSH_AUTH_SOCK",
    "COLORTERM",
    "XDG_RUNTIME_DIR",
    "XDG_SESSION_ID",
    "XDG_SESSION_TYPE",
    "XDG_CURRENT_DESKTOP",
    "XDG_CONFIG_HOME",
    "XDG_DATA_HOME",
    "XDG_CACHE_HOME",
    "XDG_STATE_HOME",
    "XDG_DATA_DIRS",
    "XDG_CONFIG_DIRS",
];
/// Prefijo de las variables de locale (`LC_ALL`, `LC_CTYPE`...), también por defecto.
pub const LOCALE_ENV_PREFIX: &str = "LC_";

/// Pares de `vars` que forman el entorno de sesión (sin `PATH`: lo reconstruye la shell de login).
/// Compara bytes para no fallar con claves que no sean Unicode válido.
pub fn session_env(vars: impl Iterator<Item = (OsString, OsString)>) -> Vec<(OsString, OsString)> {
    vars.filter(|(key, _): &(OsString, OsString)| {
        let bytes: &[u8] = key.as_encoded_bytes();
        bytes.starts_with(LOCALE_ENV_PREFIX.as_bytes())
            || DEFAULT_ENV_KEYS
                .iter()
                .any(|allowed: &&str| bytes == allowed.as_bytes())
    })
    .collect()
}

/// Entrecomilla `value` para una shell POSIX (y fish): `'...'` con cada `'` como `'\''`.
pub fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}

/// Lanzamiento de un programa a través de la shell de login con entorno limpio.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LoginShellInvocation {
    /// Ejecutable de la shell.
    pub shell: String,
    /// Argumentos de la shell: `-l -c "exec 'programa' 'arg'..."`.
    pub args: Vec<String>,
    /// Entorno completo del hijo (debe aplicarse tras vaciar el heredado).
    pub env: Vec<(OsString, OsString)>,
}

/// Construye el lanzamiento de `program` con `program_args` por `$SHELL -l -c "exec ..."`.
/// `exec` hace que el proceso lanzado sea el propio agente y no la shell.
pub fn login_shell_invocation(
    program: &str,
    program_args: &[String],
    vars: impl Iterator<Item = (OsString, OsString)>,
) -> LoginShellInvocation {
    let mut env: Vec<(OsString, OsString)> = session_env(vars);
    let shell: String = env
        .iter()
        .find(|(key, _): &&(OsString, OsString)| key == SHELL_KEY)
        .and_then(|(_, value): &(OsString, OsString)| value.to_str())
        .filter(|value: &&str| !value.is_empty())
        .unwrap_or(FALLBACK_SHELL)
        .to_string();
    env.push((OsString::from(PATH_KEY), OsString::from(DEFAULT_PATH)));
    let script: String = std::iter::once(program)
        .chain(program_args.iter().map(String::as_str))
        .map(shell_quote)
        .fold(String::from("exec"), |acc: String, token: String| {
            format!("{acc} {token}")
        });
    LoginShellInvocation {
        shell,
        args: vec![
            LOGIN_SHELL_FLAG.to_string(),
            COMMAND_FLAG.to_string(),
            script,
        ],
        env,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pairs(entries: &[(&str, &str)]) -> Vec<(OsString, OsString)> {
        entries
            .iter()
            .map(|(key, value): &(&str, &str)| (OsString::from(key), OsString::from(value)))
            .collect()
    }

    #[test]
    fn session_env_drops_variables_set_by_the_parent() {
        let vars: Vec<(OsString, OsString)> = pairs(&[
            ("LEPTOS_SITE_ADDR", "127.0.0.1:3000"),
            ("IWF_DATABASE_URL", "x.db"),
            ("CARGO_MANIFEST_DIR", "/src"),
            ("RUST_LOG", "debug"),
            ("VIRTUAL_ENV", "/venv"),
            ("PATH", "/venv/bin:/usr/bin"),
            ("HOME", "/home/u"),
            ("LC_ALL", "C"),
            ("MY_HOME", "/nope"),
        ]);
        let kept: Vec<(OsString, OsString)> = session_env(vars.into_iter());
        assert_eq!(kept, pairs(&[("HOME", "/home/u"), ("LC_ALL", "C")]));
    }

    #[test]
    fn session_env_keeps_system_session_variables() {
        let vars: Vec<(OsString, OsString)> = pairs(&[
            ("DISPLAY", ":0"),
            ("WAYLAND_DISPLAY", "wayland-1"),
            ("XDG_RUNTIME_DIR", "/run/user/1000"),
            ("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus"),
            ("LANG", "es_ES.UTF-8"),
            ("LC_TIME", "es_ES.UTF-8"),
            ("CARGO_PKG_NAME", "iworkflow"),
        ]);
        let kept: Vec<(OsString, OsString)> = session_env(vars.into_iter());
        assert_eq!(kept.len(), 6);
        assert!(
            kept.iter()
                .all(|(key, _): &(OsString, OsString)| key != "CARGO_PKG_NAME")
        );
    }

    #[test]
    fn session_env_does_not_panic_on_non_unicode_keys() {
        use std::os::unix::ffi::OsStringExt;
        let odd: OsString = OsString::from_vec(b"LC_\xff".to_vec());
        let kept: Vec<(OsString, OsString)> =
            session_env(vec![(odd.clone(), OsString::from("v"))].into_iter());
        assert_eq!(kept, vec![(odd, OsString::from("v"))]);
    }

    #[test]
    fn shell_quote_wraps_in_single_quotes() {
        assert_eq!(shell_quote("a b"), "'a b'");
        assert_eq!(shell_quote(""), "''");
    }

    #[test]
    fn shell_quote_escapes_embedded_single_quotes() {
        assert_eq!(shell_quote("it's"), "'it'\\''s'");
    }

    #[test]
    fn invocation_runs_the_quoted_program_through_the_session_shell() {
        let vars: Vec<(OsString, OsString)> = pairs(&[
            ("SHELL", "/bin/zsh"),
            ("HOME", "/home/u"),
            ("PATH", "/venv/bin"),
            ("CARGO_MANIFEST_DIR", "/src"),
        ]);
        let invocation: LoginShellInvocation = login_shell_invocation(
            "/opt/my agent",
            &["--mode".to_string(), "it's".to_string()],
            vars.into_iter(),
        );
        assert_eq!(invocation.shell, "/bin/zsh");
        assert_eq!(
            invocation.args,
            vec![
                "-l".to_string(),
                "-c".to_string(),
                "exec '/opt/my agent' '--mode' 'it'\\''s'".to_string(),
            ]
        );
        assert_eq!(
            invocation.env,
            pairs(&[
                ("SHELL", "/bin/zsh"),
                ("HOME", "/home/u"),
                ("PATH", DEFAULT_PATH)
            ])
        );
    }

    #[test]
    fn invocation_falls_back_to_bin_sh_without_a_shell() {
        let missing: LoginShellInvocation =
            login_shell_invocation("agent", &[], pairs(&[("HOME", "/h")]).into_iter());
        let empty: LoginShellInvocation =
            login_shell_invocation("agent", &[], pairs(&[("SHELL", "")]).into_iter());
        assert_eq!(missing.shell, "/bin/sh");
        assert_eq!(empty.shell, "/bin/sh");
        assert_eq!(missing.args[2], "exec 'agent'");
    }
}
