use std::sync::Arc;

use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_factory_port::CodeHostClientFactoryPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::github_http_client_adapter::GitHubHttpClientAdapter;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::gitlab_http_client_adapter::GitLabHttpClientAdapter;
use crate::modules::code_hosting::infrastructure::adapters::outbound::http::http_client_manager::HttpClientManager;

/// Construye el cliente HTTP de cada proveedor sobre un único gestor de clientes compartido.
#[derive(Debug, Default)]
pub struct CodeHostClientFactory {
    clients: Arc<HttpClientManager>,
}

impl CodeHostClientFactory {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_clients(clients: Arc<HttpClientManager>) -> Self {
        Self { clients }
    }
}

impl CodeHostClientFactoryPort for CodeHostClientFactory {
    fn client_for(&self, provider: HostingProvider) -> Arc<dyn CodeHostClientPort> {
        match provider {
            HostingProvider::GitLab => {
                Arc::new(GitLabHttpClientAdapter::with_clients(self.clients.clone()))
            }
            HostingProvider::GitHub => {
                Arc::new(GitHubHttpClientAdapter::with_clients(self.clients.clone()))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use axum::Router;
    use tokio::net::TcpListener;

    use super::*;
    use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;

    /// Servidor que responde en la raíz de la API de cada proveedor y en ninguna otra.
    async fn fake_server() -> std::net::SocketAddr {
        let app: Router = Router::new()
            .route(
                "/api/v4/user",
                axum::routing::get(|| async { r#"{"username":"desde-gitlab"}"# }),
            )
            .route(
                "/api/v3/user",
                axum::routing::get(|| async { r#"{"login":"desde-github"}"# }),
            );
        let listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr: std::net::SocketAddr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            let _ = axum::serve(listener, app).await;
        });
        addr
    }

    async fn user_via_factory(provider: HostingProvider) -> String {
        let addr: std::net::SocketAddr = fake_server().await;
        let settings: CodeHostSettings =
            CodeHostSettings::new(provider, &format!("http://{addr}"), "tok", false)
                .expect("settings");
        CodeHostClientFactory::new()
            .client_for(provider)
            .current_user(&settings)
            .await
            .expect("user")
    }

    #[tokio::test]
    async fn gitlab_provider_gets_the_gitlab_client() {
        assert_eq!(
            user_via_factory(HostingProvider::GitLab).await,
            "desde-gitlab"
        );
    }

    #[tokio::test]
    async fn github_provider_gets_the_github_client() {
        assert_eq!(
            user_via_factory(HostingProvider::GitHub).await,
            "desde-github"
        );
    }
}
