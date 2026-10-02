//! Renderizado seguro de Markdown a HTML para mensajes de agentes.

use pulldown_cmark::{CowStr, Event, Options, Parser, Tag, html};

/// Esquemas de enlace permitidos; cualquier otro (p. ej. `javascript:`) se anula.
const SAFE_SCHEMES: [&str; 3] = ["http://", "https://", "mailto:"];

fn is_safe_url(url: &str) -> bool {
    let lower: String = url.trim().to_lowercase();
    SAFE_SCHEMES
        .iter()
        .any(|scheme: &&str| lower.starts_with(scheme))
        || lower.starts_with('#')
        || (lower.starts_with('/') && !lower.starts_with("//") && !lower.starts_with("/\\"))
}

fn sanitize(event: Event<'_>) -> Event<'_> {
    match event {
        // El HTML crudo se muestra como texto: nunca se inyecta en el DOM.
        Event::Html(raw) | Event::InlineHtml(raw) => Event::Text(raw),
        Event::Start(Tag::Link {
            link_type,
            dest_url,
            title,
            id,
        }) if !is_safe_url(&dest_url) => Event::Start(Tag::Link {
            link_type,
            dest_url: CowStr::Borrowed("#"),
            title,
            id,
        }),
        Event::Start(Tag::Image {
            link_type,
            dest_url,
            title,
            id,
        }) if !is_safe_url(&dest_url) => Event::Start(Tag::Image {
            link_type,
            dest_url: CowStr::Borrowed("#"),
            title,
            id,
        }),
        other => other,
    }
}

/// Convierte Markdown en HTML sin HTML crudo ni enlaces peligrosos.
pub fn render_markdown(source: &str) -> String {
    let options: Options =
        Options::ENABLE_TABLES | Options::ENABLE_STRIKETHROUGH | Options::ENABLE_TASKLISTS;
    let parser = Parser::new_ext(source, options).map(sanitize);
    let mut output: String = String::new();
    html::push_html(&mut output, parser);
    output
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn renders_bold_and_lists() {
        let html: String = render_markdown("**Advertencias**\n- **Fallo:** texto");
        assert!(html.contains("<strong>Advertencias</strong>"));
        assert!(html.contains("<li><strong>Fallo:</strong> texto</li>"));
    }

    #[test]
    fn escapes_raw_html() {
        let html: String = render_markdown("<script>alert(1)</script>");
        assert!(!html.contains("<script>"));
        assert!(html.contains("&lt;script&gt;"));
    }

    #[test]
    fn neutralizes_unsafe_links() {
        let html: String = render_markdown("[x](javascript:alert(1)) [ok](https://a.dev)");
        assert!(!html.contains("javascript:"));
        assert!(html.contains("href=\"https://a.dev\""));
    }

    #[test]
    fn rejects_mixed_case_data_and_protocol_relative_urls() {
        for url in [
            "JavaScript:alert(1)",
            "data:text/html,x",
            "//evil.com/x",
            "/\\evil.com/x",
        ] {
            let link: String = render_markdown(&format!("[x]({url})"));
            assert!(link.contains("href=\"#\""), "{url}");
            let image: String = render_markdown(&format!("![x]({url})"));
            assert!(image.contains("src=\"#\""), "{url}");
        }
    }

    #[test]
    fn keeps_root_relative_urls() {
        let html: String = render_markdown("[x](/tasks/1)");
        assert!(html.contains("href=\"/tasks/1\""));
    }
}
