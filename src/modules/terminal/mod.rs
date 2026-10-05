#[cfg(feature = "ssr")]
pub mod application;
#[cfg(feature = "ssr")]
pub mod domain;
#[cfg(feature = "ssr")]
pub mod infrastructure;
#[cfg(all(test, feature = "ssr"))]
pub mod mocks;
pub mod presentation;
