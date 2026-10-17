use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Duration;

use reqwest::Client;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;

const REQUEST_TIMEOUT: Duration = Duration::from_secs(30);
const CONNECT_TIMEOUT: Duration = Duration::from_secs(10);

/// Gestor único de clientes HTTP de GitLab.
///
/// Se construye una sola vez en la raíz de composición (`CodeHostContext`) y se comparte por
/// `Arc`: así se reutiliza el pool de conexiones (y las sesiones TLS) de `reqwest` en vez de
/// abrir un cliente nuevo por petición. Mantiene a lo sumo dos clientes, uno con verificación
/// TLS estricta y otro que acepta certificados no confiables, porque esa opción es del cliente.
#[derive(Debug, Default)]
pub struct HttpClientManager {
    clients: Mutex<HashMap<bool, Client>>,
}

impl HttpClientManager {
    pub fn new() -> Self {
        Self::default()
    }

    /// Cliente compartido para el modo TLS indicado; se crea en el primer uso.
    pub fn client(&self, insecure_tls: bool) -> Result<Client, CodeHostError> {
        let mut clients = self
            .clients
            .lock()
            .map_err(|_| CodeHostError::Network("cliente HTTP no disponible".to_string()))?;
        if let Some(client) = clients.get(&insecure_tls) {
            return Ok(client.clone());
        }
        let client: Client = Client::builder()
            .timeout(REQUEST_TIMEOUT)
            .connect_timeout(CONNECT_TIMEOUT)
            // El token va en una cabecera propia que reqwest no retira al cambiar de host.
            .redirect(reqwest::redirect::Policy::none())
            .danger_accept_invalid_certs(insecure_tls)
            .build()
            .map_err(|e: reqwest::Error| CodeHostError::Network(e.without_url().to_string()))?;
        clients.insert(insecure_tls, client.clone());
        Ok(client)
    }

    /// Cuántos clientes distintos se han creado (para tests).
    #[cfg(test)]
    fn built(&self) -> usize {
        self.clients.lock().map(|c| c.len()).unwrap_or_default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn one_client_is_built_per_tls_mode_and_reused() {
        let manager: HttpClientManager = HttpClientManager::new();
        manager.client(false).expect("strict");
        manager.client(false).expect("strict again");
        assert_eq!(manager.built(), 1);
        manager.client(true).expect("insecure");
        manager.client(true).expect("insecure again");
        assert_eq!(manager.built(), 2);
    }
}
