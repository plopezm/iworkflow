//! Pure logic behind the printable diff report.

use crate::modules::tasks::presentation::state::transcript::{Transcript, TranscriptItem};

/// Prompt sent to the task agent to obtain the explanations embedded in the report.
pub const EXPLANATION_PROMPT: &str = "Genera un informe explicativo de los cambios realizados en esta tarea. \
Para cada fichero modificado explica qué cambió y por qué. Incluye además las decisiones clave, \
los trade-offs y las alternativas descartadas. Responde en markdown con encabezados, \
y no hagas ningún cambio de código adicional.";

/// Diff of a single file extracted from a unified patch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FilePatch {
    pub name: String,
    pub lines: Vec<String>,
}

/// Returns the prompt that asks the agent to explain the changes.
pub fn explanation_prompt() -> String {
    EXPLANATION_PROMPT.to_string()
}

/// Splits a unified patch into one chunk per `diff --git` header.
///
/// Lines before the first header (or the whole patch when there is no header) are grouped
/// into a nameless chunk so nothing is lost. An empty patch yields no chunks.
pub fn split_patch_by_file(patch: &str) -> Vec<FilePatch> {
    let mut files: Vec<FilePatch> = Vec::new();
    for line in patch.lines() {
        if line.starts_with("diff --git") {
            files.push(FilePatch {
                name: file_name_from_header(line),
                lines: vec![line.to_string()],
            });
        } else if let Some(current) = files.last_mut() {
            current.lines.push(line.to_string());
        } else {
            files.push(FilePatch {
                name: String::new(),
                lines: vec![line.to_string()],
            });
        }
    }
    files
}

/// Extracts the `b/` path of a `diff --git a/x b/x` header (the whole header if unparsable).
fn file_name_from_header(header: &str) -> String {
    let rest: &str = header.trim_start_matches("diff --git").trim();
    match rest.rsplit_once(" b/") {
        Some((_, name)) => name.to_string(),
        None => rest.to_string(),
    }
}

/// Finds the agent's answer to `prompt`, looking only at transcript items from `since` on.
///
/// The answer is ready when the last matching user message is followed by at least one
/// assistant message, nothing is streaming, no tool is still running and the last relevant
/// item is an assistant message. Returns the assistant texts joined by blank lines.
pub fn find_explanation(transcript: &Transcript, prompt: &str, since: usize) -> Option<String> {
    if transcript.is_streaming() {
        return None;
    }
    let items: &[TranscriptItem] = transcript.items.get(since..)?;
    let prompt_index: usize = items.iter().rposition(
        |item: &TranscriptItem| matches!(item, TranscriptItem::User { text } if text == prompt),
    )?;
    let after: &[TranscriptItem] = &items[prompt_index + 1..];

    let tool_running: bool = after
        .iter()
        .any(|item: &TranscriptItem| matches!(item, TranscriptItem::Tool { is_error: None, .. }));
    let last_relevant: Option<&TranscriptItem> = after.iter().rfind(|item: &&TranscriptItem| {
        !matches!(
            item,
            TranscriptItem::Status { .. } | TranscriptItem::Notice { .. }
        )
    });
    if tool_running || !matches!(last_relevant, Some(TranscriptItem::Assistant { .. })) {
        return None;
    }

    let text: String = after
        .iter()
        .filter_map(|item: &TranscriptItem| match item {
            TranscriptItem::Assistant { text, .. } if !text.trim().is_empty() => {
                Some(text.as_str())
            }
            _ => None,
        })
        .collect::<Vec<&str>>()
        .join("\n\n");
    (!text.is_empty()).then_some(text)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;

    fn user(text: &str) -> TranscriptItem {
        TranscriptItem::User {
            text: text.to_string(),
        }
    }

    fn assistant(text: &str) -> TranscriptItem {
        TranscriptItem::Assistant {
            text: text.to_string(),
            thinking: None,
            error: None,
        }
    }

    fn tool(is_error: Option<bool>) -> TranscriptItem {
        TranscriptItem::Tool {
            call_id: "c1".to_string(),
            name: "bash".to_string(),
            args: String::new(),
            output: String::new(),
            is_error,
        }
    }

    fn transcript(items: Vec<TranscriptItem>) -> Transcript {
        Transcript {
            items,
            ..Transcript::default()
        }
    }

    #[test]
    fn prompt_asks_for_explanations_in_spanish() {
        let prompt: String = explanation_prompt();
        assert_eq!(prompt, EXPLANATION_PROMPT);
        assert!(prompt.contains("markdown"));
        assert!(prompt.contains("alternativas descartadas"));
    }

    #[test]
    fn splits_patch_per_file() {
        let patch: &str = "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1 +1 @@\n-x\n+y\ndiff --git a/b.txt b/b.txt\n+z";
        let files: Vec<FilePatch> = split_patch_by_file(patch);
        assert_eq!(files.len(), 2);
        assert_eq!(files[0].name, "src/a.rs");
        assert_eq!(files[0].lines.len(), 6);
        assert_eq!(files[1].name, "b.txt");
        assert_eq!(files[1].lines, vec!["diff --git a/b.txt b/b.txt", "+z"]);
    }

    #[test]
    fn patch_without_header_is_a_single_nameless_chunk() {
        let files: Vec<FilePatch> = split_patch_by_file("+a\n-b");
        assert_eq!(files.len(), 1);
        assert_eq!(files[0].name, "");
        assert_eq!(files[0].lines, vec!["+a", "-b"]);
    }

    #[test]
    fn leading_lines_before_first_header_are_kept() {
        let files: Vec<FilePatch> = split_patch_by_file("preamble\ndiff --git a/x b/x\n+1");
        assert_eq!(files.len(), 2);
        assert_eq!(files[0].name, "");
        assert_eq!(files[1].name, "x");
    }

    #[test]
    fn empty_patch_has_no_files() {
        assert!(split_patch_by_file("").is_empty());
    }

    #[test]
    fn unparsable_header_falls_back_to_its_text() {
        let files: Vec<FilePatch> = split_patch_by_file("diff --git weird");
        assert_eq!(files[0].name, "weird");
    }

    #[test]
    fn finds_trailing_assistant_answer() {
        let t: Transcript = transcript(vec![
            user("hola"),
            user("P"),
            assistant("uno"),
            assistant("dos"),
        ]);
        assert_eq!(find_explanation(&t, "P", 0), Some("uno\n\ndos".to_string()));
    }

    #[test]
    fn no_trailing_assistant_is_not_ready() {
        let t: Transcript = transcript(vec![assistant("viejo"), user("P")]);
        assert_eq!(find_explanation(&t, "P", 0), None);
    }

    #[test]
    fn missing_prompt_is_not_ready() {
        let t: Transcript = transcript(vec![user("otro"), assistant("x")]);
        assert_eq!(find_explanation(&t, "P", 0), None);
    }

    #[test]
    fn running_tool_is_not_ready() {
        let t: Transcript = transcript(vec![user("P"), assistant("a"), tool(None)]);
        assert_eq!(find_explanation(&t, "P", 0), None);
        let running_before_answer: Transcript =
            transcript(vec![user("P"), tool(None), assistant("a")]);
        assert_eq!(find_explanation(&running_before_answer, "P", 0), None);
    }

    #[test]
    fn finished_tool_before_answer_is_ready() {
        let t: Transcript = transcript(vec![user("P"), tool(Some(false)), assistant("a")]);
        assert_eq!(find_explanation(&t, "P", 0), Some("a".to_string()));
    }

    #[test]
    fn finished_tool_after_answer_is_not_ready() {
        let t: Transcript = transcript(vec![user("P"), assistant("a"), tool(Some(false))]);
        assert_eq!(find_explanation(&t, "P", 0), None);
    }

    #[test]
    fn streaming_is_not_ready() {
        let mut t: Transcript = transcript(vec![user("P"), assistant("a")]);
        t.streaming_text = "sigue".to_string();
        assert_eq!(find_explanation(&t, "P", 0), None);
    }

    #[test]
    fn status_items_are_ignored() {
        let t: Transcript = transcript(vec![
            user("P"),
            assistant("a"),
            TranscriptItem::Status {
                status: TaskStatusDto::Running,
                error: None,
            },
        ]);
        assert_eq!(find_explanation(&t, "P", 0), Some("a".to_string()));
    }

    #[test]
    fn uses_the_last_matching_prompt() {
        let t: Transcript = transcript(vec![user("P"), assistant("viejo"), user("P")]);
        assert_eq!(find_explanation(&t, "P", 0), None);
    }

    #[test]
    fn ignores_answers_before_the_baseline() {
        let t: Transcript = transcript(vec![user("P"), assistant("viejo")]);
        assert_eq!(find_explanation(&t, "P", 2), None);
        assert_eq!(find_explanation(&t, "P", 9), None);
    }

    #[test]
    fn blank_answer_is_not_ready() {
        let t: Transcript = transcript(vec![user("P"), assistant("  ")]);
        assert_eq!(find_explanation(&t, "P", 0), None);
    }
}
