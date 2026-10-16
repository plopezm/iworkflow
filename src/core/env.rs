//! Entorno de sesión para procesos hijos (terminal, agentes, catálogo de modelos): parten de un
//! entorno vacío con solo las variables de sesión del sistema y se lanzan por la shell del sistema
//! (shell de login en Unix, PowerShell en Windows).

use std::ffi::OsString;

const POSIX_FALLBACK_SHELL: &str = "/bin/sh";
const POWERSHELL_SHELL: &str = "powershell.exe";
/// `PATH` base del sistema. No se hereda el del padre (puede traer entornos virtuales, shells de
/// nix, etc. de la terminal desde la que se lanzó el servidor): los perfiles de la shell lo amplían.
/// Solo aplica a shells POSIX; en Windows el `PATH` lo define el propio sistema y se hereda.
pub const DEFAULT_PATH: &str = "/usr/local/bin:/usr/bin:/bin";
const SHELL_KEY: &str = "SHELL";
const PATH_KEY: &str = "PATH";
/// Variables que cualquier sesión de usuario trae por defecto (login, locale, escritorio).
/// Los procesos parten de un entorno limpio con solo estas: lo que el proceso padre (cargo-leptos,
/// iworkflow, etc.) haya añadido no se hereda, para no solapar con los proyectos del usuario.
const POSIX_ENV_KEYS: [&str; 23] = [
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
/// Equivalente en Windows (se comparan sin distinguir mayúsculas). Incluye `PATH`: no hay shell
/// de login que lo reconstruya, así que se hereda el del sistema.
const POWERSHELL_ENV_KEYS: [&str; 34] = [
    "PATH",
    "PATHEXT",
    "SYSTEMROOT",
    "SYSTEMDRIVE",
    "WINDIR",
    "COMSPEC",
    "OS",
    "USERPROFILE",
    "HOME",
    "HOMEDRIVE",
    "HOMEPATH",
    "USERNAME",
    "USERDOMAIN",
    "COMPUTERNAME",
    "LOGONSERVER",
    "SESSIONNAME",
    "APPDATA",
    "LOCALAPPDATA",
    "PROGRAMDATA",
    "PROGRAMFILES",
    "PROGRAMFILES(X86)",
    "PROGRAMW6432",
    "COMMONPROGRAMFILES",
    "COMMONPROGRAMFILES(X86)",
    "COMMONPROGRAMW6432",
    "ALLUSERSPROFILE",
    "PUBLIC",
    "TEMP",
    "TMP",
    "PSMODULEPATH",
    "PROCESSOR_ARCHITECTURE",
    "NUMBER_OF_PROCESSORS",
    "LANG",
    "SSH_AUTH_SOCK",
];
/// Prefijo de las variables de locale (`LC_ALL`, `LC_CTYPE`...), también por defecto.
pub const LOCALE_ENV_PREFIX: &str = "LC_";

/// Familia de shell con la que se lanzan los procesos hijos.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ShellFlavor {
    /// `$SHELL -l -c "exec ..."` (sh, bash, zsh, fish).
    Posix,
    /// `powershell -Command "& ..."` (Windows).
    PowerShell,
}

impl ShellFlavor {
    /// Familia propia de la plataforma en la que se compila.
    pub const fn native() -> Self {
        if cfg!(windows) {
            Self::PowerShell
        } else {
            Self::Posix
        }
    }

    /// Shell a usar. En Posix manda `configured` (normalmente `$SHELL`) si no está vacía; en
    /// Windows siempre es PowerShell, pues sus argumentos no valen para otras shells.
    pub fn select_shell(self, configured: Option<&str>) -> String {
        match self {
            Self::Posix => configured
                .filter(|value: &&str| !value.is_empty())
                .unwrap_or(POSIX_FALLBACK_SHELL)
                .to_string(),
            Self::PowerShell => POWERSHELL_SHELL.to_string(),
        }
    }

    /// Argumentos para una shell interactiva (terminal embebida).
    pub fn interactive_args(self) -> Vec<String> {
        match self {
            Self::Posix => vec!["-l".to_string()],
            Self::PowerShell => vec!["-NoLogo".to_string()],
        }
    }

    /// `PATH` base que se impone al hijo; `None` si se hereda el del sistema.
    pub fn base_path(self) -> Option<&'static str> {
        match self {
            Self::Posix => Some(DEFAULT_PATH),
            Self::PowerShell => None,
        }
    }

    fn is_session_key(self, key: &OsString) -> bool {
        let bytes: &[u8] = key.as_encoded_bytes();
        if bytes.starts_with(LOCALE_ENV_PREFIX.as_bytes()) {
            return true;
        }
        match self {
            Self::Posix => POSIX_ENV_KEYS
                .iter()
                .any(|allowed: &&str| bytes == allowed.as_bytes()),
            Self::PowerShell => POWERSHELL_ENV_KEYS
                .iter()
                .any(|allowed: &&str| bytes.eq_ignore_ascii_case(allowed.as_bytes())),
        }
    }

    /// Entrecomilla `value` como literal para esta shell.
    pub fn quote(self, value: &str) -> String {
        match self {
            Self::Posix => shell_quote(value),
            Self::PowerShell => powershell_quote(value),
        }
    }

    /// Argumentos de la shell para ejecutar `program` con `program_args` y devolver su código de salida.
    fn command_args(self, program: &str, program_args: &[String]) -> Vec<String> {
        let tokens = std::iter::once(program)
            .chain(program_args.iter().map(String::as_str))
            .map(|token: &str| self.quote(token));
        match self {
            // `exec` hace que el proceso lanzado sea el propio agente y no la shell.
            Self::Posix => vec![
                "-l".to_string(),
                "-c".to_string(),
                tokens.fold(String::from("exec"), |acc: String, token: String| {
                    format!("{acc} {token}")
                }),
            ],
            Self::PowerShell => vec![
                "-NoLogo".to_string(),
                "-NonInteractive".to_string(),
                "-Command".to_string(),
                format!(
                    "& {}; exit $LASTEXITCODE",
                    tokens.collect::<Vec<String>>().join(" ")
                ),
            ],
        }
    }
}

/// Pares de `vars` que forman el entorno de sesión de `flavor` (sin `PATH` en Posix: lo reconstruye
/// la shell de login). Compara bytes para no fallar con claves que no sean Unicode válido.
pub fn session_env_for(
    flavor: ShellFlavor,
    vars: impl Iterator<Item = (OsString, OsString)>,
) -> Vec<(OsString, OsString)> {
    vars.filter(|(key, _): &(OsString, OsString)| flavor.is_session_key(key))
        .collect()
}

/// [`session_env_for`] con la familia de la plataforma actual.
pub fn session_env(vars: impl Iterator<Item = (OsString, OsString)>) -> Vec<(OsString, OsString)> {
    session_env_for(ShellFlavor::native(), vars)
}

/// Entrecomilla `value` para una shell POSIX (y fish): `'...'` con cada `'` como `'\''`.
pub fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}

/// Comillas simples tipográficas que PowerShell trata como delimitadores de cadena (`U+2018`,
/// `U+2019`, `U+201A`, `U+201B`), además de la comilla recta.
const POWERSHELL_SINGLE_QUOTES: [char; 5] = ['\'', '\u{2018}', '\u{2019}', '\u{201A}', '\u{201B}'];

/// Entrecomilla `value` como literal de PowerShell: `'...'` con cada comilla simple (recta o
/// tipográfica) duplicada.
pub fn powershell_quote(value: &str) -> String {
    let mut quoted: String = String::with_capacity(value.len() + 2);
    quoted.push('\'');
    for c in value.chars() {
        quoted.push(c);
        if POWERSHELL_SINGLE_QUOTES.contains(&c) {
            quoted.push(c);
        }
    }
    quoted.push('\'');
    quoted
}

/// Lanzamiento de un programa a través de la shell del sistema con entorno limpio.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LoginShellInvocation {
    /// Ejecutable de la shell.
    pub shell: String,
    /// Argumentos de la shell (`-l -c "exec 'programa' 'arg'..."` o `-Command "& 'programa' ..."`).
    pub args: Vec<String>,
    /// Entorno completo del hijo (debe aplicarse tras vaciar el heredado).
    pub env: Vec<(OsString, OsString)>,
}

/// Construye el lanzamiento de `program` con `program_args` para la plataforma actual.
pub fn login_shell_invocation(
    program: &str,
    program_args: &[String],
    vars: impl Iterator<Item = (OsString, OsString)>,
) -> LoginShellInvocation {
    login_shell_invocation_for(ShellFlavor::native(), program, program_args, vars)
}

/// Construye el lanzamiento de `program` con `program_args` por la shell de la familia `flavor`.
pub fn login_shell_invocation_for(
    flavor: ShellFlavor,
    program: &str,
    program_args: &[String],
    vars: impl Iterator<Item = (OsString, OsString)>,
) -> LoginShellInvocation {
    let mut env: Vec<(OsString, OsString)> = session_env_for(flavor, vars);
    let configured: Option<String> = env
        .iter()
        .find(|(key, _): &&(OsString, OsString)| key == SHELL_KEY)
        .and_then(|(_, value): &(OsString, OsString)| value.to_str())
        .map(str::to_string);
    let shell: String = flavor.select_shell(configured.as_deref());
    if let Some(path) = flavor.base_path() {
        env.push((OsString::from(PATH_KEY), OsString::from(path)));
    }
    LoginShellInvocation {
        shell,
        args: flavor.command_args(program, program_args),
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
        let kept: Vec<(OsString, OsString)> = session_env_for(ShellFlavor::Posix, vars.into_iter());
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
        let kept: Vec<(OsString, OsString)> = session_env_for(ShellFlavor::Posix, vars.into_iter());
        assert_eq!(kept.len(), 6);
        assert!(
            kept.iter()
                .all(|(key, _): &(OsString, OsString)| key != "CARGO_PKG_NAME")
        );
    }

    #[cfg(unix)]
    #[test]
    fn session_env_does_not_panic_on_non_unicode_keys() {
        use std::os::unix::ffi::OsStringExt;
        let odd: OsString = OsString::from_vec(b"LC_\xff".to_vec());
        let kept: Vec<(OsString, OsString)> = session_env_for(
            ShellFlavor::Posix,
            vec![(odd.clone(), OsString::from("v"))].into_iter(),
        );
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
        let invocation: LoginShellInvocation = login_shell_invocation_for(
            ShellFlavor::Posix,
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
        let missing: LoginShellInvocation = login_shell_invocation_for(
            ShellFlavor::Posix,
            "agent",
            &[],
            pairs(&[("HOME", "/h")]).into_iter(),
        );
        let empty: LoginShellInvocation = login_shell_invocation_for(
            ShellFlavor::Posix,
            "agent",
            &[],
            pairs(&[("SHELL", "")]).into_iter(),
        );
        assert_eq!(missing.shell, "/bin/sh");
        assert_eq!(empty.shell, "/bin/sh");
        assert_eq!(missing.args[2], "exec 'agent'");
    }

    #[test]
    fn powershell_env_keeps_system_variables_ignoring_case_and_inherits_path() {
        let vars: Vec<(OsString, OsString)> = pairs(&[
            ("Path", "C:\\Windows\\System32"),
            ("SystemRoot", "C:\\Windows"),
            ("USERPROFILE", "C:\\Users\\u"),
            ("LC_ALL", "C"),
            ("VIRTUAL_ENV", "C:\\venv"),
            ("CARGO_MANIFEST_DIR", "C:\\src"),
            ("SHELL", "C:\\Git\\bash.exe"),
        ]);
        let kept: Vec<(OsString, OsString)> =
            session_env_for(ShellFlavor::PowerShell, vars.into_iter());
        assert_eq!(
            kept,
            pairs(&[
                ("Path", "C:\\Windows\\System32"),
                ("SystemRoot", "C:\\Windows"),
                ("USERPROFILE", "C:\\Users\\u"),
                ("LC_ALL", "C"),
            ])
        );
    }

    #[test]
    fn powershell_quote_doubles_single_quotes() {
        assert_eq!(powershell_quote("a b"), "'a b'");
        assert_eq!(powershell_quote("it's"), "'it''s'");
    }

    #[test]
    fn powershell_quote_doubles_typographic_single_quotes() {
        assert_eq!(
            powershell_quote("a\u{2018}b\u{2019}c\u{201A}d\u{201B}e"),
            "'a\u{2018}\u{2018}b\u{2019}\u{2019}c\u{201A}\u{201A}d\u{201B}\u{201B}e'"
        );
    }

    #[test]
    fn powershell_invocation_ignores_shell_and_keeps_the_system_path() {
        let vars: Vec<(OsString, OsString)> = pairs(&[
            ("SHELL", "C:\\Git\\bash.exe"),
            ("Path", "C:\\Windows"),
            ("USERPROFILE", "C:\\Users\\u"),
        ]);
        let invocation: LoginShellInvocation = login_shell_invocation_for(
            ShellFlavor::PowerShell,
            "C:\\Program Files\\agent.exe",
            &["--mode".to_string(), "it's".to_string()],
            vars.into_iter(),
        );
        assert_eq!(invocation.shell, "powershell.exe");
        assert_eq!(
            invocation.args,
            vec![
                "-NoLogo".to_string(),
                "-NonInteractive".to_string(),
                "-Command".to_string(),
                "& 'C:\\Program Files\\agent.exe' '--mode' 'it''s'; exit $LASTEXITCODE".to_string(),
            ]
        );
        assert_eq!(
            invocation.env,
            pairs(&[("Path", "C:\\Windows"), ("USERPROFILE", "C:\\Users\\u")])
        );
    }

    #[test]
    fn interactive_args_and_base_path_depend_on_the_flavor() {
        assert_eq!(ShellFlavor::Posix.interactive_args(), vec!["-l"]);
        assert_eq!(ShellFlavor::PowerShell.interactive_args(), vec!["-NoLogo"]);
        assert_eq!(ShellFlavor::Posix.base_path(), Some(DEFAULT_PATH));
        assert_eq!(ShellFlavor::PowerShell.base_path(), None);
        assert_eq!(
            ShellFlavor::PowerShell.select_shell(Some("/bin/zsh")),
            "powershell.exe"
        );
        assert_eq!(ShellFlavor::Posix.select_shell(None), "/bin/sh");
    }
}
