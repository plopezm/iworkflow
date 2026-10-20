use std::sync::Arc;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::project_scope::ProjectScope;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::domain::ports::outbound::file_search_port::FileSearchPort;
use crate::modules::project_tree::domain::ports::outbound::workspace_root_port::WorkspaceRootPort;

/// Maximum number of suggestions returned to the caller.
pub const MAX_RESULTS: usize = 20;

/// Finds project files whose path matches a typed fragment (for `@` references).
pub struct SearchProjectFiles {
    roots: Arc<dyn WorkspaceRootPort>,
    search: Arc<dyn FileSearchPort>,
}

/// Lower is better; `None` when `path` does not match `query` (pure, case-insensitive).
fn score(path: &str, query: &str) -> Option<(u8, usize, usize)> {
    let lower: String = path.to_lowercase();
    let name_start: usize = lower.rfind('/').map_or(0, |i: usize| i + 1);
    let needle: String = query.to_lowercase();
    let name: &str = &lower[name_start..];
    // La coincidencia en el nombre del fichero gana a la de un directorio, aunque esta aparezca antes.
    let (rank, at): (u8, usize) = match name.find(&needle) {
        Some(0) => (0, name_start),
        Some(offset) => (1, name_start + offset),
        None => (2, lower.find(&needle)?),
    };
    Some((rank, path.len(), at))
}

/// Ranks `files` against `query` and keeps the best `limit` (pure).
pub fn rank_files(files: Vec<String>, query: &str, limit: usize) -> Vec<String> {
    let mut scored: Vec<((u8, usize, usize), String)> = files
        .into_iter()
        .filter_map(|path: String| score(&path, query).map(|s: (u8, usize, usize)| (s, path)))
        .collect();
    scored.sort();
    scored
        .into_iter()
        .take(limit)
        .map(|(_, path): ((u8, usize, usize), String)| path)
        .collect()
}

impl SearchProjectFiles {
    pub fn new(roots: Arc<dyn WorkspaceRootPort>, search: Arc<dyn FileSearchPort>) -> Self {
        Self { roots, search }
    }

    pub async fn execute(
        &self,
        scope: ProjectScope,
        query: &str,
    ) -> Result<Vec<String>, ProjectTreeError> {
        let root: WorkspaceRoot = self.roots.resolve(scope).await?;
        let files: Vec<String> = self.search.files(&root.path).await?;
        Ok(rank_files(files, query, MAX_RESULTS))
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use uuid::Uuid;

    use super::*;
    use crate::modules::project_tree::mocks::{MockFileSearch, MockWorkspaceRoot};

    fn names(items: &[&str]) -> Vec<String> {
        items.iter().map(|s: &&str| s.to_string()).collect()
    }

    #[test]
    fn file_name_prefix_beats_other_matches() {
        let ranked: Vec<String> = rank_files(
            names(&["docs/main_notes.md", "src/main.rs", "src/lib/model.rs"]),
            "main",
            10,
        );
        assert_eq!(ranked, names(&["src/main.rs", "docs/main_notes.md"]));
    }

    #[test]
    fn a_name_match_beats_an_earlier_directory_match() {
        let ranked: Vec<String> =
            rank_files(names(&["main_docs/readme.md", "src/main.rs"]), "main", 10);
        assert_eq!(ranked, names(&["src/main.rs", "main_docs/readme.md"]));
    }

    #[test]
    fn empty_query_lists_shortest_paths_up_to_limit() {
        let ranked: Vec<String> = rank_files(names(&["a/b/c.rs", "z.rs", "m/n.rs"]), "", 2);
        assert_eq!(ranked, names(&["z.rs", "m/n.rs"]));
    }

    #[test]
    fn matches_directory_fragments_case_insensitively() {
        let ranked: Vec<String> = rank_files(names(&["Src/Lib.rs", "other.txt"]), "src/l", 10);
        assert_eq!(ranked, names(&["Src/Lib.rs"]));
    }

    #[tokio::test]
    async fn resolves_root_then_searches() {
        let mut roots: MockWorkspaceRoot = MockWorkspaceRoot::new();
        roots.expect_resolve().times(1).returning(|_| {
            Ok(WorkspaceRoot {
                name: "demo".to_string(),
                path: PathBuf::from("/tmp/demo"),
            })
        });
        let mut search: MockFileSearch = MockFileSearch::new();
        search
            .expect_files()
            .times(1)
            .returning(|_| Ok(vec!["src/main.rs".to_string(), "README.md".to_string()]));
        let usecase: SearchProjectFiles =
            SearchProjectFiles::new(Arc::new(roots), Arc::new(search));
        let found: Vec<String> = usecase
            .execute(ProjectScope::Task(Uuid::new_v4()), "read")
            .await
            .unwrap();
        assert_eq!(found, names(&["README.md"]));
    }
}
