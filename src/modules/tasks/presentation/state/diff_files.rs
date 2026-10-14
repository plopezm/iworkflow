//! Pure model of a unified patch: per-file diffs with split/unified rows and a
//! tree of the changed files. No framework dependencies.

use std::collections::BTreeMap;

const DIFF_HEADER: &str = "diff --git ";
const DEV_NULL: &str = "/dev/null";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FileStatus {
    Added,
    Modified,
    Deleted,
    Renamed,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CellKind {
    Context,
    Added,
    Removed,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Cell {
    pub number: usize,
    pub text: String,
    pub kind: CellKind,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SplitRow {
    pub left: Option<Cell>,
    pub right: Option<Cell>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UnifiedLine {
    pub kind: CellKind,
    pub old_number: Option<usize>,
    pub new_number: Option<usize>,
    pub text: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Hunk {
    pub header: String,
    pub rows: Vec<SplitRow>,
    pub lines: Vec<UnifiedLine>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FileDiff {
    pub path: String,
    pub old_path: Option<String>,
    pub status: FileStatus,
    pub additions: usize,
    pub deletions: usize,
    pub binary: bool,
    pub hunks: Vec<Hunk>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TreeNodeKind {
    Dir(Vec<TreeNode>),
    File(usize),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TreeNode {
    pub name: String,
    pub path: String,
    pub kind: TreeNodeKind,
    pub additions: usize,
    pub deletions: usize,
}

pub fn status_letter(status: FileStatus) -> &'static str {
    match status {
        FileStatus::Added => "A",
        FileStatus::Modified => "M",
        FileStatus::Deleted => "D",
        FileStatus::Renamed => "R",
    }
}

/// Builder for one file section while its lines are being consumed.
struct FileBuilder {
    path: String,
    old_path: Option<String>,
    status: FileStatus,
    binary: bool,
    hunks: Vec<Hunk>,
    hunk: Option<HunkBuilder>,
    saw_old: Option<String>,
    saw_new: Option<String>,
}

struct HunkBuilder {
    header: String,
    old_next: usize,
    new_next: usize,
    removed: Vec<Cell>,
    added: Vec<Cell>,
    rows: Vec<SplitRow>,
    lines: Vec<UnifiedLine>,
}

impl HunkBuilder {
    fn flush(&mut self) {
        let removed: Vec<Cell> = std::mem::take(&mut self.removed);
        let added: Vec<Cell> = std::mem::take(&mut self.added);
        let count: usize = removed.len().max(added.len());
        let mut removed_iter = removed.into_iter();
        let mut added_iter = added.into_iter();
        for _ in 0..count {
            self.rows.push(SplitRow {
                left: removed_iter.next(),
                right: added_iter.next(),
            });
        }
    }

    fn push_line(&mut self, prefix: char, text: &str) {
        match prefix {
            '-' => {
                if !self.added.is_empty() {
                    self.flush();
                }
                let number: usize = self.old_next;
                self.old_next += 1;
                self.lines.push(UnifiedLine {
                    kind: CellKind::Removed,
                    old_number: Some(number),
                    new_number: None,
                    text: text.to_string(),
                });
                self.removed.push(Cell {
                    number,
                    text: text.to_string(),
                    kind: CellKind::Removed,
                });
            }
            '+' => {
                let number: usize = self.new_next;
                self.new_next += 1;
                self.lines.push(UnifiedLine {
                    kind: CellKind::Added,
                    old_number: None,
                    new_number: Some(number),
                    text: text.to_string(),
                });
                self.added.push(Cell {
                    number,
                    text: text.to_string(),
                    kind: CellKind::Added,
                });
            }
            _ => {
                self.flush();
                let old_number: usize = self.old_next;
                let new_number: usize = self.new_next;
                self.old_next += 1;
                self.new_next += 1;
                self.lines.push(UnifiedLine {
                    kind: CellKind::Context,
                    old_number: Some(old_number),
                    new_number: Some(new_number),
                    text: text.to_string(),
                });
                self.rows.push(SplitRow {
                    left: Some(Cell {
                        number: old_number,
                        text: text.to_string(),
                        kind: CellKind::Context,
                    }),
                    right: Some(Cell {
                        number: new_number,
                        text: text.to_string(),
                        kind: CellKind::Context,
                    }),
                });
            }
        }
    }

    fn finish(mut self) -> Hunk {
        self.flush();
        Hunk {
            header: self.header,
            rows: self.rows,
            lines: self.lines,
        }
    }
}

impl FileBuilder {
    fn from_header(line: &str) -> Self {
        let rest: &str = line.strip_prefix(DIFF_HEADER).unwrap_or(line);
        let (old, new): (String, String) = match rest.find(" b/") {
            Some(index) => (
                rest[..index].trim_start_matches("a/").to_string(),
                rest[index + 3..].to_string(),
            ),
            None => (rest.to_string(), rest.to_string()),
        };
        Self {
            path: new,
            old_path: Some(old),
            status: FileStatus::Modified,
            binary: false,
            hunks: Vec::new(),
            hunk: None,
            saw_old: None,
            saw_new: None,
        }
    }

    fn close_hunk(&mut self) {
        if let Some(hunk) = self.hunk.take() {
            self.hunks.push(hunk.finish());
        }
    }

    fn consume(&mut self, line: &str) {
        if let Some(hunk) = self.hunk.as_mut() {
            let mut chars = line.chars();
            match chars.next() {
                Some(prefix @ ('+' | '-' | ' ')) => hunk.push_line(prefix, chars.as_str()),
                Some('\\') => {}
                None => hunk.push_line(' ', ""),
                Some(_) if line.starts_with("@@") => {
                    self.close_hunk();
                    self.start_hunk(line);
                }
                Some(_) => hunk.push_line(' ', line),
            }
            return;
        }
        if line.starts_with("@@") {
            self.start_hunk(line);
        } else if line.starts_with("new file mode") {
            self.status = FileStatus::Added;
        } else if line.starts_with("deleted file mode") {
            self.status = FileStatus::Deleted;
        } else if let Some(from) = line.strip_prefix("rename from ") {
            self.status = FileStatus::Renamed;
            self.old_path = Some(from.to_string());
        } else if let Some(to) = line.strip_prefix("rename to ") {
            self.status = FileStatus::Renamed;
            self.path = to.to_string();
        } else if line.starts_with("Binary files") || line.starts_with("GIT binary patch") {
            self.binary = true;
        } else if let Some(old) = line.strip_prefix("--- ") {
            self.saw_old = strip_side(old, "a/");
        } else if let Some(new) = line.strip_prefix("+++ ") {
            self.saw_new = strip_side(new, "b/");
        }
    }

    fn start_hunk(&mut self, line: &str) {
        let (old_start, new_start): (usize, usize) = parse_hunk_starts(line).unwrap_or((1, 1));
        self.hunk = Some(HunkBuilder {
            header: line.to_string(),
            old_next: old_start,
            new_next: new_start,
            removed: Vec::new(),
            added: Vec::new(),
            rows: Vec::new(),
            lines: Vec::new(),
        });
    }

    fn finish(mut self) -> FileDiff {
        self.close_hunk();
        if self.status == FileStatus::Deleted {
            if let Some(old) = self.saw_old.clone().or_else(|| self.old_path.clone()) {
                self.path = old;
            }
        } else if let Some(new) = self.saw_new.clone() {
            self.path = new;
        }
        let additions: usize = self
            .hunks
            .iter()
            .map(|h| count_kind(h, CellKind::Added))
            .sum();
        let deletions: usize = self
            .hunks
            .iter()
            .map(|h| count_kind(h, CellKind::Removed))
            .sum();
        let old_path: Option<String> = match self.status {
            FileStatus::Renamed => self.old_path,
            _ => None,
        };
        FileDiff {
            path: self.path,
            old_path,
            status: self.status,
            additions,
            deletions,
            binary: self.binary,
            hunks: self.hunks,
        }
    }
}

fn count_kind(hunk: &Hunk, kind: CellKind) -> usize {
    hunk.lines.iter().filter(|l| l.kind == kind).count()
}

fn strip_side(value: &str, prefix: &str) -> Option<String> {
    let trimmed: &str = value.trim_end_matches('\t');
    if trimmed == DEV_NULL {
        None
    } else {
        Some(trimmed.strip_prefix(prefix).unwrap_or(trimmed).to_string())
    }
}

/// Extracts the old and new start lines from `@@ -a,b +c,d @@`.
fn parse_hunk_starts(header: &str) -> Option<(usize, usize)> {
    let rest: &str = header.strip_prefix("@@ -")?;
    let (old_part, rest): (&str, &str) = rest.split_once(" +")?;
    let new_part: &str = rest.split(' ').next()?;
    let old: usize = old_part.split(',').next()?.parse().ok()?;
    let new: usize = new_part.split(',').next()?.parse().ok()?;
    Some((old, new))
}

/// Parses a git unified patch into one `FileDiff` per file. Tolerates a patch
/// truncated at any point, including mid-line.
pub fn parse_patch(patch: &str) -> Vec<FileDiff> {
    let mut files: Vec<FileDiff> = Vec::new();
    let mut current: Option<FileBuilder> = None;
    let body: &str = patch.strip_suffix('\n').unwrap_or(patch);
    for line in body.split('\n') {
        let line: &str = line.strip_suffix('\r').unwrap_or(line);
        if line.starts_with(DIFF_HEADER) {
            if let Some(done) = current.take() {
                files.push(done.finish());
            }
            current = Some(FileBuilder::from_header(line));
        } else if let Some(builder) = current.as_mut() {
            builder.consume(line);
        }
    }
    if let Some(done) = current {
        files.push(done.finish());
    }
    files
}

#[derive(Default)]
struct DirBuilder {
    dirs: BTreeMap<String, DirBuilder>,
    files: BTreeMap<String, usize>,
}

/// Builds a folder tree: folders first, then files, alphabetical; folders
/// aggregate counters and single-child directory chains are compacted.
pub fn build_tree(files: &[FileDiff]) -> Vec<TreeNode> {
    let mut root: DirBuilder = DirBuilder::default();
    for (index, file) in files.iter().enumerate() {
        let parts: Vec<&str> = file.path.split('/').collect();
        let Some((name, dirs)) = parts.split_last() else {
            continue;
        };
        let mut node: &mut DirBuilder = &mut root;
        for dir in dirs {
            node = node.dirs.entry((*dir).to_string()).or_default();
        }
        node.files.insert((*name).to_string(), index);
    }
    materialize(&root, "", files)
}

fn materialize(dir: &DirBuilder, prefix: &str, files: &[FileDiff]) -> Vec<TreeNode> {
    let mut nodes: Vec<TreeNode> = Vec::new();
    for (name, child) in &dir.dirs {
        let mut label: String = name.clone();
        let mut path: String = format!("{prefix}{name}");
        let mut current: &DirBuilder = child;
        while current.files.is_empty() && current.dirs.len() == 1 {
            let Some((next_name, next)) = current.dirs.iter().next() else {
                break;
            };
            label = format!("{label}/{next_name}");
            path = format!("{path}/{next_name}");
            current = next;
        }
        let children: Vec<TreeNode> = materialize(current, &format!("{path}/"), files);
        let additions: usize = children.iter().map(|c| c.additions).sum();
        let deletions: usize = children.iter().map(|c| c.deletions).sum();
        nodes.push(TreeNode {
            name: label,
            path,
            kind: TreeNodeKind::Dir(children),
            additions,
            deletions,
        });
    }
    for (name, index) in &dir.files {
        let file: &FileDiff = &files[*index];
        nodes.push(TreeNode {
            name: name.clone(),
            path: file.path.clone(),
            kind: TreeNodeKind::File(*index),
            additions: file.additions,
            deletions: file.deletions,
        });
    }
    nodes
}

#[cfg(test)]
mod tests {
    use super::*;

    const MODIFY: &str = "diff --git a/src/a.rs b/src/a.rs\nindex 1..2 100644\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1,4 +1,5 @@ fn x\n one\n-two\n-three\n+TWO\n+THREE\n+extra\n four\n";

    fn only(patch: &str) -> FileDiff {
        let mut files: Vec<FileDiff> = parse_patch(patch);
        assert_eq!(files.len(), 1);
        files.remove(0)
    }

    #[test]
    fn modify_counts_and_pairs_blocks() {
        let file: FileDiff = only(MODIFY);
        assert_eq!(file.path, "src/a.rs");
        assert_eq!(file.status, FileStatus::Modified);
        assert_eq!((file.additions, file.deletions), (3, 2));
        let rows: &Vec<SplitRow> = &file.hunks[0].rows;
        assert_eq!(rows.len(), 5);
        let left: &Cell = rows[1].left.as_ref().unwrap();
        let right: &Cell = rows[1].right.as_ref().unwrap();
        assert_eq!((left.text.as_str(), left.kind), ("two", CellKind::Removed));
        assert_eq!((right.text.as_str(), right.kind), ("TWO", CellKind::Added));
        assert!(rows[3].left.is_none());
        assert_eq!(rows[3].right.as_ref().unwrap().text, "extra");
    }

    #[test]
    fn line_numbers_follow_hunk_header() {
        let file: FileDiff = only(MODIFY);
        let rows: &Vec<SplitRow> = &file.hunks[0].rows;
        assert_eq!(rows[0].left.as_ref().unwrap().number, 1);
        assert_eq!(rows[1].left.as_ref().unwrap().number, 2);
        assert_eq!(rows[2].left.as_ref().unwrap().number, 3);
        assert_eq!(rows[1].right.as_ref().unwrap().number, 2);
        assert_eq!(rows[3].right.as_ref().unwrap().number, 4);
        let last: &SplitRow = &rows[4];
        assert_eq!(last.left.as_ref().unwrap().number, 4);
        assert_eq!(last.right.as_ref().unwrap().number, 5);
        let unified: &Vec<UnifiedLine> = &file.hunks[0].lines;
        assert_eq!(unified[1].old_number, Some(2));
        assert_eq!(unified[1].new_number, None);
        assert_eq!(unified[1].text, "two");
        assert_eq!(unified[3].new_number, Some(2));
    }

    #[test]
    fn second_hunk_uses_its_own_start() {
        let patch: &str = "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,1 +1,1 @@\n-a\n+b\n@@ -20,1 +30,1 @@\n-c\n+d\n";
        let file: FileDiff = only(patch);
        assert_eq!(file.hunks.len(), 2);
        let row: &SplitRow = &file.hunks[1].rows[0];
        assert_eq!(row.left.as_ref().unwrap().number, 20);
        assert_eq!(row.right.as_ref().unwrap().number, 30);
    }

    #[test]
    fn added_file() {
        let patch: &str = "diff --git a/n.txt b/n.txt\nnew file mode 100644\nindex 0..1\n--- /dev/null\n+++ b/n.txt\n@@ -0,0 +1,2 @@\n+a\n+b\n";
        let file: FileDiff = only(patch);
        assert_eq!(file.status, FileStatus::Added);
        assert_eq!(file.path, "n.txt");
        assert_eq!((file.additions, file.deletions), (2, 0));
        assert!(file.hunks[0].rows.iter().all(|r| r.left.is_none()));
    }

    #[test]
    fn deleted_file_uses_old_path() {
        let patch: &str = "diff --git a/d.txt b/d.txt\ndeleted file mode 100644\nindex 1..0\n--- a/d.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n";
        let file: FileDiff = only(patch);
        assert_eq!(file.status, FileStatus::Deleted);
        assert_eq!(file.path, "d.txt");
        assert_eq!((file.additions, file.deletions), (0, 2));
    }

    #[test]
    fn rename_with_and_without_changes() {
        let patch: &str = "diff --git a/old.rs b/new.rs\nsimilarity index 90%\nrename from old.rs\nrename to new.rs\nindex 1..2\n--- a/old.rs\n+++ b/new.rs\n@@ -1 +1 @@\n-x\n+y\ndiff --git a/p b/q\nsimilarity index 100%\nrename from p\nrename to q\n";
        let files: Vec<FileDiff> = parse_patch(patch);
        assert_eq!(files.len(), 2);
        assert_eq!(files[0].status, FileStatus::Renamed);
        assert_eq!(files[0].path, "new.rs");
        assert_eq!(files[0].old_path.as_deref(), Some("old.rs"));
        assert_eq!(files[0].additions, 1);
        assert_eq!(files[1].path, "q");
        assert_eq!(files[1].old_path.as_deref(), Some("p"));
        assert!(files[1].hunks.is_empty());
    }

    #[test]
    fn binary_file() {
        let patch: &str = "diff --git a/i.png b/i.png\nindex 1..2 100644\nBinary files a/i.png and b/i.png differ\n";
        let file: FileDiff = only(patch);
        assert!(file.binary);
        assert!(file.hunks.is_empty());
        assert_eq!(file.status, FileStatus::Modified);
    }

    #[test]
    fn pure_mode_change() {
        let patch: &str = "diff --git a/s.sh b/s.sh\nold mode 100644\nnew mode 100755\n";
        let file: FileDiff = only(patch);
        assert_eq!(file.status, FileStatus::Modified);
        assert_eq!(file.path, "s.sh");
        assert!(file.hunks.is_empty());
    }

    #[test]
    fn no_newline_marker_is_ignored() {
        let patch: &str = "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n";
        let file: FileDiff = only(patch);
        assert_eq!((file.additions, file.deletions), (1, 1));
        assert_eq!(file.hunks[0].lines.len(), 2);
        assert_eq!(file.hunks[0].rows.len(), 1);
    }

    #[test]
    fn truncated_patch_keeps_partial_hunk() {
        let patch: &str = "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n-b\n+par";
        let file: FileDiff = only(patch);
        assert_eq!((file.additions, file.deletions), (1, 1));
        assert_eq!(file.hunks[0].rows[1].right.as_ref().unwrap().text, "par");
        assert!(parse_patch("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3").len() == 1);
        assert!(parse_patch("").is_empty());
    }

    #[test]
    fn status_letters() {
        assert_eq!(status_letter(FileStatus::Added), "A");
        assert_eq!(status_letter(FileStatus::Modified), "M");
        assert_eq!(status_letter(FileStatus::Deleted), "D");
        assert_eq!(status_letter(FileStatus::Renamed), "R");
    }

    fn file(path: &str, additions: usize, deletions: usize) -> FileDiff {
        FileDiff {
            path: path.to_string(),
            old_path: None,
            status: FileStatus::Modified,
            additions,
            deletions,
            binary: false,
            hunks: Vec::new(),
        }
    }

    #[test]
    fn tree_orders_aggregates_and_compacts() {
        let files: Vec<FileDiff> = vec![
            file("zeta.md", 1, 0),
            file("src/modules/b.rs", 2, 1),
            file("src/modules/a.rs", 3, 0),
            file("src/main.rs", 4, 4),
            file("docs/deep/only/x.md", 5, 0),
            file("alpha.md", 1, 1),
        ];
        let tree: Vec<TreeNode> = build_tree(&files);
        let names: Vec<&str> = tree.iter().map(|n| n.name.as_str()).collect();
        assert_eq!(names, vec!["docs/deep/only", "src", "alpha.md", "zeta.md"]);
        assert_eq!(tree[0].path, "docs/deep/only");
        let src: &TreeNode = &tree[1];
        assert_eq!((src.additions, src.deletions), (9, 5));
        let TreeNodeKind::Dir(children) = &src.kind else {
            panic!("src must be a dir");
        };
        let child_names: Vec<&str> = children.iter().map(|n| n.name.as_str()).collect();
        assert_eq!(child_names, vec!["modules", "main.rs"]);
        assert_eq!(children[0].path, "src/modules");
        let TreeNodeKind::Dir(leaves) = &children[0].kind else {
            panic!("modules must be a dir");
        };
        assert_eq!(leaves[0].kind, TreeNodeKind::File(2));
        assert_eq!(leaves[1].kind, TreeNodeKind::File(1));
    }

    #[test]
    fn tree_compacts_root_chain() {
        let files: Vec<FileDiff> = vec![file("src/modules/a.rs", 1, 0)];
        let tree: Vec<TreeNode> = build_tree(&files);
        assert_eq!(tree.len(), 1);
        assert_eq!(tree[0].name, "src/modules");
        assert_eq!(tree[0].path, "src/modules");
    }
}
