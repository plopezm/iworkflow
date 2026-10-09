use axum::Router;
use axum::extract::DefaultBodyLimit;

/// Tope del cuerpo de las peticiones de server functions: 5 imágenes de 4 MiB en base64
/// (~26,7 MiB) más margen. El límite por defecto de axum es 2 MiB.
pub const SERVER_FN_MAX_BODY_BYTES: usize = 30 * 1024 * 1024;

/// Eleva el límite de cuerpo solo para las rutas del `router` recibido.
pub fn with_server_fn_body_limit<S>(router: Router<S>) -> Router<S>
where
    S: Clone + Send + Sync + 'static,
{
    router.layer(DefaultBodyLimit::max(SERVER_FN_MAX_BODY_BYTES))
}

#[cfg(test)]
mod tests {
    use axum::Router;
    use axum::body::Bytes;
    use axum::routing::post;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::{TcpListener, TcpStream};

    use super::*;

    async fn post_status(router: Router, body_len: usize) -> String {
        let listener: TcpListener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr: std::net::SocketAddr = listener.local_addr().expect("addr");
        tokio::spawn(async move {
            axum::serve(listener, router).await.expect("serve");
        });
        let mut stream: TcpStream = TcpStream::connect(addr).await.expect("connect");
        let head: String = format!(
            "POST /echo HTTP/1.1\r\nHost: x\r\nContent-Length: {body_len}\r\nConnection: close\r\n\r\n"
        );
        stream.write_all(head.as_bytes()).await.expect("head");
        let _ = stream.write_all(&vec![b'a'; body_len]).await;
        let mut response: Vec<u8> = Vec::new();
        let _ = stream.read_to_end(&mut response).await;
        String::from_utf8_lossy(&response)
            .lines()
            .next()
            .unwrap_or_default()
            .to_string()
    }

    fn router() -> Router {
        Router::new().route(
            "/echo",
            post(|body: Bytes| async move { body.len().to_string() }),
        )
    }

    #[test]
    fn body_limit_fits_five_four_mib_images_in_base64() {
        assert_eq!(SERVER_FN_MAX_BODY_BYTES, 30 * 1024 * 1024);
    }

    #[tokio::test]
    async fn default_axum_limit_rejects_a_payload_of_five_images() {
        let status: String = post_status(router(), SERVER_FN_MAX_BODY_BYTES - 1024 * 1024).await;

        assert!(status.contains("413"), "{status}");
    }

    #[tokio::test]
    async fn scoped_limit_accepts_a_payload_of_five_images() {
        let status: String =
            post_status(with_server_fn_body_limit(router()), 27 * 1024 * 1024).await;

        assert!(status.contains("200"), "{status}");
    }

    #[tokio::test]
    async fn scoped_limit_still_rejects_oversized_payloads() {
        let status: String = post_status(
            with_server_fn_body_limit(router()),
            SERVER_FN_MAX_BODY_BYTES + 1024,
        )
        .await;

        assert!(status.contains("413"), "{status}");
    }
}
