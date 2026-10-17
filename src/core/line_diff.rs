//! Diff por líneas entre un texto base y el actual, para marcar cambios en el margen del editor.

/// Marca de cambio de una línea del texto actual.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum LineMark {
    Added,
    Modified,
    /// Hay líneas borradas justo antes de esta línea (o tras la última).
    Deleted,
}

/// Por encima de `n * m` celdas no se calcula la LCS: el tramo central entero cuenta como modificado.
const MAX_LCS_CELLS: usize = 4_000_000;

/// Una marca por línea de `current` (se parte por `\n`, siempre al menos una línea) (pura).
pub fn line_marks(base: &str, current: &str) -> Vec<Option<LineMark>> {
    let old: Vec<&str> = base.split('\n').collect();
    let new: Vec<&str> = current.split('\n').collect();
    let mut marks: Vec<Option<LineMark>> = vec![None; new.len()];

    let prefix: usize = old
        .iter()
        .zip(new.iter())
        .take_while(|(a, b)| a == b)
        .count();
    let max_suffix: usize = old.len().min(new.len()) - prefix;
    let suffix: usize = old
        .iter()
        .rev()
        .zip(new.iter().rev())
        .take(max_suffix)
        .take_while(|(a, b)| a == b)
        .count();
    let a: &[&str] = &old[prefix..old.len() - suffix];
    let b: &[&str] = &new[prefix..new.len() - suffix];
    if a.is_empty() && b.is_empty() {
        return marks;
    }

    if a.len().saturating_mul(b.len()) > MAX_LCS_CELLS {
        for mark in &mut marks[prefix..prefix + b.len()] {
            *mark = Some(LineMark::Modified);
        }
        return marks;
    }

    let (mut i, mut j): (usize, usize) = (0, 0);
    let table: Vec<Vec<u32>> = lcs_table(a, b);
    while i < a.len() || j < b.len() {
        if i < a.len() && j < b.len() && a[i] == b[j] {
            i += 1;
            j += 1;
            continue;
        }
        let hunk_start: usize = j;
        let mut removed: usize = 0;
        while (i < a.len() || j < b.len()) && !(i < a.len() && j < b.len() && a[i] == b[j]) {
            if j == b.len() || (i < a.len() && table[i + 1][j] >= table[i][j + 1]) {
                removed += 1;
                i += 1;
            } else {
                j += 1;
            }
        }
        apply_hunk(&mut marks, prefix + hunk_start, removed, j - hunk_start);
    }
    marks
}

/// `table[i][j]` = longitud de la LCS de `a[i..]` y `b[j..]`.
fn lcs_table(a: &[&str], b: &[&str]) -> Vec<Vec<u32>> {
    let mut table: Vec<Vec<u32>> = vec![vec![0; b.len() + 1]; a.len() + 1];
    for i in (0..a.len()).rev() {
        for j in (0..b.len()).rev() {
            table[i][j] = if a[i] == b[j] {
                table[i + 1][j + 1] + 1
            } else {
                table[i + 1][j].max(table[i][j + 1])
            };
        }
    }
    table
}

/// Marca un hunk que empieza en `start` (índice en el texto actual).
fn apply_hunk(marks: &mut [Option<LineMark>], start: usize, removed: usize, added: usize) {
    if added == 0 {
        if let Some(last) = marks.len().checked_sub(1) {
            marks[start.min(last)] = Some(LineMark::Deleted);
        }
        return;
    }
    for (offset, mark) in marks[start..start + added].iter_mut().enumerate() {
        *mark = Some(if offset < removed {
            LineMark::Modified
        } else {
            LineMark::Added
        });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    use LineMark::{Added, Deleted, Modified};

    #[test]
    fn identical_text_has_no_marks() {
        assert_eq!(line_marks("a\nb\nc", "a\nb\nc"), vec![None, None, None]);
        assert_eq!(line_marks("", ""), vec![None]);
    }

    #[test]
    fn inserted_lines_are_added() {
        assert_eq!(
            line_marks("a\nd", "a\nb\nc\nd"),
            vec![None, Some(Added), Some(Added), None]
        );
    }

    #[test]
    fn changed_line_is_modified() {
        assert_eq!(
            line_marks("a\nb\nc", "a\nB\nc"),
            vec![None, Some(Modified), None]
        );
    }

    #[test]
    fn deletion_marks_the_following_line() {
        assert_eq!(line_marks("a\nb\nc", "a\nc"), vec![None, Some(Deleted)]);
    }

    #[test]
    fn deletion_at_the_end_marks_the_last_line() {
        assert_eq!(line_marks("a\nb\nc", "a\nb"), vec![None, Some(Deleted)]);
        assert_eq!(line_marks("a\nb", "a"), vec![Some(Deleted)]);
    }

    #[test]
    fn replaced_block_with_extra_lines_is_modified_then_added() {
        assert_eq!(
            line_marks("a\nb\nc\nz", "a\nX\nY\nZ\nz"),
            vec![None, Some(Modified), Some(Modified), Some(Added), None]
        );
    }

    #[test]
    fn replaced_block_with_fewer_lines_is_all_modified() {
        assert_eq!(
            line_marks("a\nb\nc\nd\nz", "a\nX\nz"),
            vec![None, Some(Modified), None]
        );
    }

    #[test]
    fn empty_base_marks_the_single_empty_line_as_modified() {
        // `""` se parte en una línea vacía: la primera línea actual la sustituye.
        assert_eq!(line_marks("", "a\nb"), vec![Some(Modified), Some(Added)]);
    }

    #[test]
    fn trailing_newline_adds_an_empty_last_line() {
        assert_eq!(line_marks("a", "a\n"), vec![None, Some(Added)]);
        assert_eq!(line_marks("a\n", "a\n"), vec![None, None]);
        assert_eq!(line_marks("a\n", "a"), vec![Some(Deleted)]);
    }

    #[test]
    fn huge_middle_part_is_marked_modified_without_lcs() {
        let old: String = (0..2500)
            .map(|n: u32| format!("o{n}"))
            .collect::<Vec<String>>()
            .join("\n");
        let new: String = (0..2500)
            .map(|n: u32| format!("n{n}"))
            .collect::<Vec<String>>()
            .join("\n");
        let marks: Vec<Option<LineMark>> = line_marks(&old, &new);
        assert_eq!(marks.len(), 2500);
        assert!(
            marks
                .iter()
                .all(|m: &Option<LineMark>| *m == Some(Modified))
        );
    }
}
