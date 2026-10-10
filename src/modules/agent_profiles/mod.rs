#[cfg(feature = "ssr")]
pub mod application;
#[cfg(feature = "ssr")]
pub mod domain;
pub mod infrastructure;
#[cfg(all(test, feature = "ssr"))]
pub mod mocks;
