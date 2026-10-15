use std::collections::HashSet;
use std::fs;

use nix::sys::signal::{Signal, kill};
use nix::unistd::Pid;

const PROC_DIR: &str = "/proc";
/// Pasadas máximas de barrido: cubre procesos que hacen `fork` mientras se matan los anteriores.
const MAX_PASSES: usize = 5;

/// Campos de `/proc/<pid>/stat` que importan para localizar los procesos de una terminal.
struct ProcEntry {
    pid: u32,
    ppid: u32,
    session: u32,
}

/// Extrae `ppid` y `session` de una línea de `/proc/<pid>/stat`
/// (`pid (comm) state ppid pgrp session ...`). `comm` puede contener espacios y paréntesis,
/// por eso se parte tras el último `)`.
fn parse_stat(pid: u32, stat: &str) -> Option<ProcEntry> {
    let rest: &str = stat.get(stat.rfind(')')? + 1..)?;
    let mut fields = rest.split_whitespace().skip(1);
    let ppid: u32 = fields.next()?.parse().ok()?;
    let _pgrp: &str = fields.next()?;
    let session: u32 = fields.next()?.parse().ok()?;
    Some(ProcEntry { pid, ppid, session })
}

fn scan() -> Vec<ProcEntry> {
    let Ok(dir) = fs::read_dir(PROC_DIR) else {
        return Vec::new();
    };
    dir.filter_map(|entry: Result<fs::DirEntry, std::io::Error>| {
        let entry: fs::DirEntry = entry.ok()?;
        let pid: u32 = entry.file_name().to_str()?.parse().ok()?;
        let stat: String = fs::read_to_string(entry.path().join("stat")).ok()?;
        parse_stat(pid, &stat)
    })
    .collect()
}

/// Procesos de la terminal cuyo líder de sesión es `root`: la sesión completa (incluye procesos
/// huérfanos reparentados a init, en segundo plano o en primer plano) más todos los descendientes
/// por árbol de padres (cubre los que crearon su propia sesión con `setsid`). Excluye `root`.
fn members(root: u32, procs: &[ProcEntry]) -> Vec<u32> {
    let mut found: HashSet<u32> = procs
        .iter()
        .filter(|proc: &&ProcEntry| proc.session == root && proc.pid != root)
        .map(|proc: &ProcEntry| proc.pid)
        .collect();
    let mut frontier: Vec<u32> = found.iter().copied().chain([root]).collect();
    while let Some(parent) = frontier.pop() {
        for proc in procs.iter().filter(|proc: &&ProcEntry| proc.ppid == parent) {
            if proc.pid != root && found.insert(proc.pid) {
                frontier.push(proc.pid);
            }
        }
    }
    found.into_iter().collect()
}

/// Mata con `SIGKILL` todo lo que lanzó la shell `root` (aún viva): sesión y descendientes.
/// Debe llamarse antes de matar la shell, pues después sus hijos pasan a colgar de init.
/// La propia shell no se mata aquí.
pub fn kill_descendants(root: u32) {
    for _ in 0..MAX_PASSES {
        let pids: Vec<u32> = members(root, &scan());
        if pids.is_empty() {
            return;
        }
        for pid in pids {
            if let Ok(raw) = i32::try_from(pid) {
                // El proceso puede haber terminado entre el barrido y la señal.
                let _ = kill(Pid::from_raw(raw), Signal::SIGKILL);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(pid: u32, ppid: u32, session: u32) -> ProcEntry {
        ProcEntry { pid, ppid, session }
    }

    #[test]
    fn parse_stat_handles_comm_with_spaces_and_parentheses() {
        let parsed: Option<ProcEntry> = parse_stat(42, "42 (we ird) name) S 7 42 40 0 -1");
        let parsed: ProcEntry = parsed.expect("parsed");
        assert_eq!((parsed.pid, parsed.ppid, parsed.session), (42, 7, 40));
    }

    #[test]
    fn parse_stat_rejects_garbage() {
        assert!(parse_stat(1, "garbage").is_none());
    }

    #[test]
    fn members_include_session_orphans_and_setsid_descendants_but_not_root() {
        let procs: Vec<ProcEntry> = vec![
            entry(100, 1, 100),   // shell (root)
            entry(101, 100, 100), // hijo directo
            entry(102, 1, 100),   // huérfano de la misma sesión
            entry(103, 101, 103), // nieto con setsid
            entry(104, 103, 103), // bisnieto
            entry(200, 1, 200),   // ajeno
        ];
        let mut found: Vec<u32> = members(100, &procs);
        found.sort_unstable();
        assert_eq!(found, vec![101, 102, 103, 104]);
    }
}
