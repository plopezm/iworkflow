use std::sync::Arc;

use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;

/// Entrega el cliente del proveedor detectado para un workspace.
pub trait CodeHostClientFactoryPort: Send + Sync {
    fn client_for(&self, provider: HostingProvider) -> Arc<dyn CodeHostClientPort>;
}
