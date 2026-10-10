use std::path::{Path, PathBuf};

use tokio::io::AsyncWriteExt;

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::models::agent_role::{
    AgentRole, NewAgentRole, RoleName,
};
use crate::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;

const FRONTMATTER_DELIMITER: &str = "---";
const ROLE_EXTENSION: &str = "md";

/// Roles = ficheros `<name>.md` con frontmatter en un directorio de agentes.
pub struct AgentRoleDirAdapter {
    dir: PathBuf,
}

impl AgentRoleDirAdapter {
    pub fn new(agents_dir: PathBuf) -> Self {
        Self { dir: agents_dir }
    }
}

#[derive(Debug, Default, PartialEq, Eq)]
struct Frontmatter {
    name: Option<String>,
    description: Option<String>,
    model: Option<String>,
    thinking: Option<String>,
}

fn unquote(value: &str) -> String {
    let trimmed: &str = value.trim();
    if trimmed.len() >= 2 && trimmed.starts_with('"') && trimmed.ends_with('"') {
        return serde_json::from_str::<String>(trimmed)
            .unwrap_or_else(|_| trimmed[1..trimmed.len() - 1].to_string());
    }
    if trimmed.len() >= 2 && trimmed.starts_with('\'') && trimmed.ends_with('\'') {
        return trimmed[1..trimmed.len() - 1].replace("''", "'");
    }
    trimmed.to_string()
}

/// Parser mínimo: solo claves escalares de primer nivel entre los dos `---`.
fn parse_frontmatter(content: &str) -> Frontmatter {
    let mut lines: std::str::Lines<'_> = content.lines();
    let mut frontmatter: Frontmatter = Frontmatter::default();
    if lines.next().map(str::trim) != Some(FRONTMATTER_DELIMITER) {
        return frontmatter;
    }
    for line in lines {
        if line.trim() == FRONTMATTER_DELIMITER {
            break;
        }
        if line.starts_with(char::is_whitespace) {
            continue;
        }
        let Some((key, value)) = line.split_once(':') else {
            continue;
        };
        let value: String = unquote(value);
        match key.trim() {
            "name" => frontmatter.name = Some(value),
            "description" => frontmatter.description = Some(value),
            "model" if !value.is_empty() => frontmatter.model = Some(value),
            "thinking" if !value.is_empty() => frontmatter.thinking = Some(value),
            _ => {}
        }
    }
    frontmatter
}

fn role_from_file(stem: &str, content: &str) -> Option<AgentRole> {
    let frontmatter: Frontmatter = parse_frontmatter(content);
    let name: RoleName = frontmatter
        .name
        .as_deref()
        .and_then(|n: &str| RoleName::parse(n).ok())
        .or_else(|| RoleName::parse(stem).ok())?;
    Some(AgentRole {
        name,
        description: frontmatter.description.unwrap_or_default(),
        model: frontmatter.model,
        thinking: frontmatter.thinking,
    })
}

fn needs_quotes(value: &str) -> bool {
    let first: Option<char> = value.chars().next();
    value.is_empty()
        || value != value.trim()
        || value.contains(": ")
        || value.contains(" #")
        || value.ends_with(':')
        || first.is_some_and(|c: char| "\"'#&*!|>%@`[]{},-?".contains(c))
}

fn single_line_value(label: &str, value: &str) -> Result<String, AgentProfilesError> {
    if value.contains('\n') || value.contains('\r') {
        return Err(AgentProfilesError::Store(format!(
            "{label} no puede contener saltos de línea"
        )));
    }
    if needs_quotes(value) {
        return serde_json::to_string(value)
            .map_err(|e: serde_json::Error| AgentProfilesError::Store(e.to_string()));
    }
    Ok(value.to_string())
}

fn render_role(role: &NewAgentRole) -> Result<String, AgentProfilesError> {
    let description: String = single_line_value("la descripción", &role.description)?;
    let mut out: String = format!(
        "{FRONTMATTER_DELIMITER}\nname: {}\ndescription: {description}\n{FRONTMATTER_DELIMITER}\n",
        role.name
    );
    let prompt: &str = role.prompt.trim_end();
    if !prompt.is_empty() {
        out.push('\n');
        out.push_str(prompt);
        out.push('\n');
    }
    Ok(out)
}

fn io_error(e: std::io::Error) -> AgentProfilesError {
    AgentProfilesError::Io(e.to_string())
}

async fn read_role_file(path: &Path) -> Result<Option<AgentRole>, AgentProfilesError> {
    let is_md: bool = path
        .extension()
        .is_some_and(|ext: &std::ffi::OsStr| ext == ROLE_EXTENSION);
    let stem: Option<String> = path
        .file_stem()
        .map(|s: &std::ffi::OsStr| s.to_string_lossy().to_string());
    let Some(stem) = stem.filter(|_: &String| is_md) else {
        return Ok(None);
    };
    let content: String = match tokio::fs::read_to_string(path).await {
        Ok(content) => content,
        Err(e) if e.kind() == std::io::ErrorKind::InvalidData => return Ok(None),
        Err(e) => return Err(io_error(e)),
    };
    Ok(role_from_file(&stem, &content))
}

#[async_trait::async_trait]
impl AgentRolePort for AgentRoleDirAdapter {
    async fn list(&self) -> Result<Vec<AgentRole>, AgentProfilesError> {
        let mut entries: tokio::fs::ReadDir = match tokio::fs::read_dir(&self.dir).await {
            Ok(entries) => entries,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(Vec::new()),
            Err(e) => return Err(io_error(e)),
        };
        let mut roles: Vec<AgentRole> = Vec::new();
        while let Some(entry) = entries.next_entry().await.map_err(io_error)? {
            let file_type: std::fs::FileType = entry.file_type().await.map_err(io_error)?;
            if !file_type.is_file() && !file_type.is_symlink() {
                continue;
            }
            if let Some(role) = read_role_file(&entry.path()).await? {
                roles.push(role);
            }
        }
        roles.sort_by(|a: &AgentRole, b: &AgentRole| a.name.cmp(&b.name));
        Ok(roles)
    }

    async fn create(&self, role: &NewAgentRole) -> Result<(), AgentProfilesError> {
        let content: String = render_role(role)?;
        tokio::fs::create_dir_all(&self.dir)
            .await
            .map_err(io_error)?;
        let path: PathBuf = self.dir.join(format!("{}.{ROLE_EXTENSION}", role.name));
        let mut file: tokio::fs::File = match tokio::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&path)
            .await
        {
            Ok(file) => file,
            Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => {
                return Err(AgentProfilesError::RoleAlreadyExists(role.name.to_string()));
            }
            Err(e) => return Err(io_error(e)),
        };
        file.write_all(content.as_bytes()).await.map_err(io_error)?;
        file.flush().await.map_err(io_error)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn new_role(description: &str, prompt: &str) -> NewAgentRole {
        NewAgentRole {
            name: RoleName::parse("r").expect("name"),
            description: description.to_string(),
            prompt: prompt.to_string(),
        }
    }

    #[test]
    fn parses_scalar_keys_and_ignores_lists_and_body() {
        let fm: Frontmatter = parse_frontmatter(
            "---\nname: x\ndescription: 'It''s ok'\nmodel: p/m\nthinking:\ntools:\n  - read\n---\nname: nope\n",
        );
        assert_eq!(fm.name.as_deref(), Some("x"));
        assert_eq!(fm.description.as_deref(), Some("It's ok"));
        assert_eq!(fm.model.as_deref(), Some("p/m"));
        assert_eq!(fm.thinking, None);
    }

    #[test]
    fn no_frontmatter_yields_defaults() {
        assert_eq!(
            parse_frontmatter("hello\n---\nname: x"),
            Frontmatter::default()
        );
    }

    #[test]
    fn invalid_frontmatter_name_falls_back_to_stem() {
        let role: AgentRole = role_from_file("stem", "---\nname: bad name\n---\n").expect("role");
        assert_eq!(role.name.as_str(), "stem");
        assert!(role_from_file("bad stem", "no frontmatter").is_none());
    }

    #[test]
    fn render_quotes_risky_descriptions_and_rejects_newlines() {
        let rendered: String = render_role(&new_role("Does: things", "body")).expect("render");
        assert!(rendered.contains("description: \"Does: things\"\n"));
        let parsed: Frontmatter = parse_frontmatter(&rendered);
        assert_eq!(parsed.description.as_deref(), Some("Does: things"));
        assert!(render_role(&new_role("a\nb", "x")).is_err());
        assert!(render_role(&new_role("a\rb", "x")).is_err());
    }

    #[test]
    fn render_appends_prompt_with_single_trailing_newline() {
        let rendered: String = render_role(&new_role("d", "body\n\n")).expect("render");
        assert!(rendered.ends_with("---\n\nbody\n"));
    }
}
