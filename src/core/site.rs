//! Resolution of the Leptos site assets (CSS, WASM, JS) for a packaged binary.
//!
//! `cargo leptos` injects `LEPTOS_*` variables only in development. A release archive
//! ships `iworkflow[.exe]` next to a `site/` directory, so without help the server would
//! look for `target/site` relative to the current directory and serve the page unstyled.

use std::path::{Path, PathBuf};

use leptos::prelude::LeptosOptions;

const SITE_DIR_NAME: &str = "site";
const OUTPUT_NAME: &str = "iworkflow";
const SITE_ROOT_VAR: &str = "LEPTOS_SITE_ROOT";
const OUTPUT_NAME_VAR: &str = "LEPTOS_OUTPUT_NAME";
const SITE_ADDR_VAR: &str = "LEPTOS_SITE_ADDR";
/// Port of a packaged server (release binary, Electron app) when `LEPTOS_SITE_ADDR` is unset.
const PACKAGED_PORT: u16 = 9000;

/// Site directory shipped next to the executable, if it exists.
pub fn bundled_site_root(exe_dir: &Path) -> Option<PathBuf> {
    let candidate: PathBuf = exe_dir.join(SITE_DIR_NAME);
    candidate.is_dir().then_some(candidate)
}

/// Fills in the options the environment did not set explicitly.
pub fn apply_packaged_defaults(
    mut options: LeptosOptions,
    exe_dir: Option<&Path>,
    site_root_set: bool,
    output_name_set: bool,
) -> LeptosOptions {
    if !output_name_set {
        options.output_name = OUTPUT_NAME.into();
    }
    if !site_root_set && let Some(root) = exe_dir.and_then(bundled_site_root) {
        options.site_root = root.to_string_lossy().as_ref().into();
    }
    options
}

/// Moves the listen port to [`PACKAGED_PORT`] unless the environment pinned the address.
/// The host is kept (loopback), so the network-access setting still decides the exposure.
pub fn apply_packaged_port(mut options: LeptosOptions, site_addr_set: bool) -> LeptosOptions {
    if !site_addr_set {
        options.site_addr.set_port(PACKAGED_PORT);
    }
    options
}

/// Same as [`apply_packaged_defaults`] reading the real environment and executable path.
pub fn with_packaged_defaults(options: LeptosOptions) -> LeptosOptions {
    let exe_dir: Option<PathBuf> = std::env::current_exe()
        .ok()
        .and_then(|exe: PathBuf| exe.parent().map(Path::to_path_buf));
    let options: LeptosOptions = apply_packaged_defaults(
        options,
        exe_dir.as_deref(),
        std::env::var_os(SITE_ROOT_VAR).is_some(),
        std::env::var_os(OUTPUT_NAME_VAR).is_some(),
    );
    apply_packaged_port(options, std::env::var_os(SITE_ADDR_VAR).is_some())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn base() -> LeptosOptions {
        LeptosOptions::builder().output_name("other").build()
    }

    #[test]
    fn uses_site_dir_next_to_the_executable() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        std::fs::create_dir(dir.path().join("site")).expect("site dir");
        let options: LeptosOptions = apply_packaged_defaults(base(), Some(dir.path()), false, true);
        assert_eq!(
            &*options.site_root,
            dir.path().join("site").to_string_lossy().as_ref()
        );
        assert_eq!(&*options.output_name, "other");
    }

    #[test]
    fn explicit_site_root_wins() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        std::fs::create_dir(dir.path().join("site")).expect("site dir");
        let before: String = base().site_root.to_string();
        let options: LeptosOptions = apply_packaged_defaults(base(), Some(dir.path()), true, true);
        assert_eq!(&*options.site_root, before);
    }

    #[test]
    fn missing_site_dir_keeps_default_root() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let before: String = base().site_root.to_string();
        let options: LeptosOptions = apply_packaged_defaults(base(), Some(dir.path()), false, true);
        assert_eq!(&*options.site_root, before);
    }

    #[test]
    fn output_name_defaults_to_the_binary_name() {
        let options: LeptosOptions = apply_packaged_defaults(base(), None, true, false);
        assert_eq!(&*options.output_name, "iworkflow");
    }

    #[test]
    fn packaged_port_is_used_when_the_address_is_not_pinned() {
        let options: LeptosOptions = apply_packaged_port(base(), false);
        assert_eq!(options.site_addr.port(), 9000);
        assert!(options.site_addr.ip().is_loopback());
    }

    #[test]
    fn pinned_address_keeps_its_port() {
        let before: u16 = base().site_addr.port();
        let options: LeptosOptions = apply_packaged_port(base(), true);
        assert_eq!(options.site_addr.port(), before);
    }
}
