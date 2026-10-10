#[cfg(feature = "ssr")]
pub mod application;
#[cfg(feature = "ssr")]
pub mod domain;
#[cfg(all(test, feature = "ssr"))]
pub mod mocks;
