use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;

/// Clase CSS de una línea de un parche unificado.
pub fn line_class(line: &str) -> &'static str {
    if line.starts_with("diff --git") {
        "diff-file"
    } else if line.starts_with("+++") || line.starts_with("---") {
        "diff-meta"
    } else if line.starts_with("@@") {
        "diff-hunk"
    } else if line.starts_with('+') {
        "diff-add"
    } else if line.starts_with('-') {
        "diff-del"
    } else {
        "diff-ctx"
    }
}

#[component]
pub fn DiffView(diff: TaskDiffDto) -> impl IntoView {
    if diff.patch.is_empty() && diff.commits.is_empty() {
        return view! { <p class="muted">"Sin cambios respecto a la rama base."</p> }.into_any();
    }
    view! {
        <div class="stack">
            <h3>{format!("Commits ({})", diff.commits.len())}</h3>
            <pre>{diff.commits.join("\n")}</pre>
            <h3>"Resumen"</h3>
            <pre>{diff.stat}</pre>
            {diff
                .truncated
                .then(|| view! { <p class="notice notice-warn">"Parche truncado: revisa el resto con git."</p> })}
            <pre class="diff">
                {diff
                    .patch
                    .lines()
                    .map(|line: &str| {
                        view! { <div class=line_class(line)>{line.to_string()}</div> }
                    })
                    .collect_view()}
            </pre>
        </div>
    }
    .into_any()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_patch_lines() {
        assert_eq!(line_class("diff --git a/x b/x"), "diff-file");
        assert_eq!(line_class("+++ b/x"), "diff-meta");
        assert_eq!(line_class("@@ -1 +1 @@"), "diff-hunk");
        assert_eq!(line_class("+nuevo"), "diff-add");
        assert_eq!(line_class("-viejo"), "diff-del");
        assert_eq!(line_class(" igual"), "diff-ctx");
    }
}
