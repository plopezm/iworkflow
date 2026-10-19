//! The address other devices open, with the access token embedded so scanning the QR code logs in.

use qrcode::QrCode;
use qrcode::render::svg;

/// Smallest rendering of one QR module, in SVG pixels (the card scales it with CSS).
const QR_MODULE_PX: u32 = 4;
const QR_DARK: &str = "#000000";
const QR_LIGHT: &str = "#ffffff";

/// `http://<host>:<port>/?token=<token>`; without a token the plain address. IPv6 hosts are
/// bracketed. The guard redirects away from the token URL right after storing it in a cookie.
pub fn build_access_url(host: &str, port: u16, token: Option<&str>) -> String {
    let host: String = if host.contains(':') && !host.starts_with('[') {
        format!("[{host}]")
    } else {
        host.to_string()
    };
    match token {
        Some(token) => format!("http://{host}:{port}/?token={token}"),
        None => format!("http://{host}:{port}"),
    }
}

/// Inline SVG of the QR code for `url`; `None` if it cannot be encoded.
pub fn render_qr_svg(url: &str) -> Option<String> {
    let code: QrCode = QrCode::new(url.as_bytes()).ok()?;
    Some(
        code.render::<svg::Color>()
            .module_dimensions(QR_MODULE_PX, QR_MODULE_PX)
            .dark_color(svg::Color(QR_DARK))
            .light_color(svg::Color(QR_LIGHT))
            .build(),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn url_embeds_the_token_in_the_query() {
        assert_eq!(
            build_access_url("192.168.1.5", 3000, Some("abc")),
            "http://192.168.1.5:3000/?token=abc"
        );
    }

    #[test]
    fn url_without_token_is_the_plain_address() {
        assert_eq!(
            build_access_url("192.168.1.5", 3000, None),
            "http://192.168.1.5:3000"
        );
    }

    #[test]
    fn ipv6_hosts_are_bracketed() {
        assert_eq!(
            build_access_url("fd00::1", 9000, None),
            "http://[fd00::1]:9000"
        );
    }

    #[test]
    fn qr_is_an_inline_svg() {
        let svg: String = render_qr_svg("http://192.168.1.5:3000/?token=abc").expect("qr");
        assert!(svg.contains("<svg"));
        assert!(svg.contains("</svg>"));
    }
}
