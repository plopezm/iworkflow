//! Autocompletion of the message box: `/command` at the start of the message and `@file`
//! references anywhere. Everything here is pure text logic so it can be tested natively.

use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::AgentModeDto;

/// What the token under the caret is asking to complete.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TriggerKind {
    Command,
    File,
}

/// The token under the caret that opened the completion menu.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Trigger {
    pub kind: TriggerKind,
    /// Byte offset of the `/` or `@` that opens the token.
    pub start: usize,
    /// Byte offset of the caret (end of the token).
    pub end: usize,
    /// Text typed after the trigger character.
    pub query: String,
}

/// A slash command offered by the menu. It is sent to the agent as plain text.
#[derive(Debug, Clone, Copy)]
pub struct SlashCommand {
    pub name: &'static str,
    pub description: Msg,
}

pub const SLASH_COMMANDS: [SlashCommand; 7] = [
    SlashCommand {
        name: "plan",
        description: Msg {
            en: "Switch to plan mode",
            es: "Cambiar al modo plan",
            zh: "切换到计划模式",
            de: "In den Planmodus wechseln",
            fr: "Passer en mode plan",
        },
    },
    SlashCommand {
        name: "agent",
        description: Msg {
            en: "Switch to agent mode",
            es: "Cambiar al modo agente",
            zh: "切换到智能体模式",
            de: "In den Agent-Modus wechseln",
            fr: "Passer en mode agent",
        },
    },
    SlashCommand {
        name: "compact",
        description: Msg {
            en: "Summarize the conversation to free context",
            es: "Resumir la conversación para liberar contexto",
            zh: "总结对话以释放上下文",
            de: "Unterhaltung zusammenfassen, um Kontext freizugeben",
            fr: "Résumer la conversation pour libérer du contexte",
        },
    },
    SlashCommand {
        name: "clear",
        description: Msg {
            en: "Clear the conversation",
            es: "Limpiar la conversación",
            zh: "清除对话",
            de: "Unterhaltung löschen",
            fr: "Effacer la conversation",
        },
    },
    SlashCommand {
        name: "model",
        description: Msg {
            en: "Change the model",
            es: "Cambiar el modelo",
            zh: "更换模型",
            de: "Modell wechseln",
            fr: "Changer de modèle",
        },
    },
    SlashCommand {
        name: "review",
        description: Msg {
            en: "Review the current changes",
            es: "Revisar los cambios actuales",
            zh: "审查当前更改",
            de: "Aktuelle Änderungen prüfen",
            fr: "Relire les modifications en cours",
        },
    },
    SlashCommand {
        name: "help",
        description: Msg {
            en: "List the agent's commands",
            es: "Listar los comandos del agente",
            zh: "列出智能体的命令",
            de: "Befehle des Agents auflisten",
            fr: "Lister les commandes de l’agent",
        },
    },
];

/// The mode a message asks for when it is exactly `/plan` or `/agent` (surrounding whitespace ignored).
pub fn mode_command(text: &str) -> Option<AgentModeDto> {
    match text.trim() {
        "/plan" => Some(AgentModeDto::Plan),
        "/agent" => Some(AgentModeDto::Agent),
        _ => None,
    }
}

/// Converts a JavaScript caret position (UTF-16 code units) into a byte offset of `text`.
pub fn byte_offset(text: &str, utf16: usize) -> usize {
    let mut units: usize = 0;
    for (index, ch) in text.char_indices() {
        if units >= utf16 {
            return index;
        }
        units += ch.len_utf16();
    }
    text.len()
}

/// Converts a byte offset of `text` into a JavaScript caret position (UTF-16 code units).
pub fn utf16_offset(text: &str, byte: usize) -> usize {
    text.get(..byte)
        .map_or(0, |head: &str| head.encode_utf16().count())
}

/// Finds the completion trigger at the caret, if any. A command only counts when it is the
/// first thing in the message; a file reference counts anywhere after whitespace.
pub fn detect(text: &str, caret: usize) -> Option<Trigger> {
    let head: &str = text.get(..caret)?;
    let start: usize = head.rfind(char::is_whitespace).map_or(0, |i: usize| {
        i + head[i..].chars().next().map_or(1, char::len_utf8)
    });
    let token: &str = &head[start..];
    let mut chars = token.chars();
    let kind: TriggerKind = match chars.next()? {
        '@' => TriggerKind::File,
        '/' if start == 0 => TriggerKind::Command,
        _ => return None,
    };
    Some(Trigger {
        kind,
        start,
        end: caret,
        query: chars.as_str().to_string(),
    })
}

/// Commands whose name starts with `query` (case-insensitive).
pub fn matching_commands(query: &str) -> Vec<SlashCommand> {
    let needle: String = query.to_lowercase();
    SLASH_COMMANDS
        .iter()
        .copied()
        .filter(|command: &SlashCommand| command.name.starts_with(&needle))
        .collect()
}

/// Replaces the trigger token with `replacement` followed by a space. Returns the new text and
/// the caret position (byte offset) right after the inserted space.
pub fn apply(text: &str, trigger: &Trigger, replacement: &str) -> (String, usize) {
    let tail: &str = text.get(trigger.end..).unwrap_or_default();
    let inserted: String = format!("{replacement} ");
    let caret: usize = trigger.start + inserted.len();
    (format!("{}{inserted}{tail}", &text[..trigger.start]), caret)
}

/// Distinct `@path` references in `text`, in order of appearance (path without the `@`).
pub fn references(text: &str) -> Vec<String> {
    let mut found: Vec<String> = Vec::new();
    for token in text.split_whitespace() {
        if let Some(path) = token.strip_prefix('@')
            && !path.is_empty()
            && !found.iter().any(|known: &String| known == path)
        {
            found.push(path.to_string());
        }
    }
    found
}

/// Removes every `@path` reference to `path` from `text`.
pub fn remove_reference(text: &str, path: &str) -> String {
    let target: String = format!("@{path}");
    let mut out: String = String::with_capacity(text.len());
    let mut rest: &str = text;
    while !rest.is_empty() {
        let word_len: usize = rest.find(char::is_whitespace).unwrap_or(rest.len());
        let (word, after) = rest.split_at(word_len);
        let space_len: usize = after
            .find(|c: char| !c.is_whitespace())
            .unwrap_or(after.len());
        if word == target {
            rest = &after[space_len..];
        } else {
            out.push_str(word);
            out.push_str(&after[..space_len]);
            rest = &after[space_len..];
        }
    }
    out.trim_end().to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn at_end(text: &str) -> Option<Trigger> {
        detect(text, text.len())
    }

    #[test]
    fn slash_only_triggers_at_message_start() {
        let trigger: Trigger = at_end("/pl").expect("command");
        assert_eq!(trigger.kind, TriggerKind::Command);
        assert_eq!(trigger.query, "pl");
        assert_eq!(at_end("hello /pl"), None);
        assert_eq!(at_end("/plan now"), None);
    }

    #[test]
    fn at_triggers_after_whitespace_and_reports_the_query() {
        let trigger: Trigger = at_end("look at @src/ma").expect("file");
        assert_eq!(trigger.kind, TriggerKind::File);
        assert_eq!(trigger.query, "src/ma");
        assert_eq!(trigger.start, 8);
        assert_eq!(at_end("mail me@host"), None);
    }

    #[test]
    fn caret_in_the_middle_only_sees_text_before_it() {
        let trigger: Trigger = detect("fix @ab cd", 7).expect("file");
        assert_eq!(trigger.query, "ab");
        assert_eq!(trigger.end, 7);
    }

    #[test]
    fn utf16_and_byte_offsets_round_trip_with_multibyte_text() {
        let text: &str = "ñ😀@a";
        let byte: usize = byte_offset(text, 4);
        assert_eq!(&text[byte..], "a");
        assert_eq!(utf16_offset(text, byte), 4);
        assert_eq!(byte_offset(text, 99), text.len());
    }

    #[test]
    fn apply_replaces_the_token_and_keeps_the_tail() {
        let text: &str = "fix @ab now";
        let trigger: Trigger = detect(text, 7).expect("file");
        let (out, caret) = apply(text, &trigger, "@src/ab.rs");
        assert_eq!(out, "fix @src/ab.rs  now");
        assert_eq!(&out[..caret], "fix @src/ab.rs ");
    }

    #[test]
    fn commands_are_filtered_by_prefix() {
        let names: Vec<&str> = matching_commands("C")
            .iter()
            .map(|c: &SlashCommand| c.name)
            .collect();
        assert_eq!(names, vec!["compact", "clear"]);
        assert_eq!(matching_commands("").len(), SLASH_COMMANDS.len());
        assert!(matching_commands("zzz").is_empty());
    }

    #[test]
    fn mode_commands_must_match_the_whole_message() {
        assert_eq!(mode_command("/plan"), Some(AgentModeDto::Plan));
        assert_eq!(mode_command("  /agent\n"), Some(AgentModeDto::Agent));
        assert_eq!(mode_command("/plan now"), None);
        assert_eq!(mode_command("plan"), None);
        assert_eq!(mode_command("/compact"), None);
    }

    #[test]
    fn agent_command_is_offered_by_prefix() {
        let names: Vec<&str> = matching_commands("a")
            .iter()
            .map(|c: &SlashCommand| c.name)
            .collect();
        assert_eq!(names, vec!["agent"]);
    }

    #[test]
    fn references_are_distinct_and_ordered() {
        assert_eq!(
            references("see @a.rs and @b/c.rs again @a.rs @ alone"),
            vec!["a.rs".to_string(), "b/c.rs".to_string()]
        );
    }

    #[test]
    fn removing_a_reference_keeps_the_rest_of_the_message() {
        assert_eq!(
            remove_reference("see @a.rs and @b.rs now", "a.rs"),
            "see and @b.rs now"
        );
        assert_eq!(remove_reference("@a.rs", "a.rs"), "");
    }
}
