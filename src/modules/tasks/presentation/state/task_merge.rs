use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStageDto,
};

/// Reemplaza en su sitio la tarea con el mismo id o, si no está, la añade al final.
pub fn upsert_task(list: &mut Vec<TaskDto>, task: TaskDto) {
    match list.iter_mut().find(|t: &&mut TaskDto| t.id == task.id) {
        Some(slot) => *slot = task,
        None => list.push(task),
    }
}

/// Quita la tarea `id`; devuelve si estaba en la lista.
pub fn remove_task(list: &mut Vec<TaskDto>, id: &str) -> bool {
    let before: usize = list.len();
    list.retain(|t: &TaskDto| t.id != id);
    list.len() != before
}

/// Pone la tarea al principio descartando cualquier copia previa con su id.
pub fn prepend_unique(list: &mut Vec<TaskDto>, task: TaskDto) {
    list.retain(|t: &TaskDto| t.id != task.id);
    list.insert(0, task);
}

/// Aplica la versión fresca de una tarea a las listas de activas y finalizadas (función pura).
/// Una tarea finalizada sale de las activas; entra al principio de las finalizadas solo si
/// `track_done` (el tablero muestra las finalizadas) y no estaba ya, en cuyo caso se
/// reemplaza en su sitio. Una tarea no finalizada se inserta/actualiza en las activas y
/// desaparece de las finalizadas.
pub fn apply_task_change(
    active: &mut Vec<TaskDto>,
    done: &mut Vec<TaskDto>,
    task: TaskDto,
    track_done: bool,
) {
    if task.stage != TaskStageDto::Done {
        remove_task(done, &task.id);
        upsert_task(active, task);
        return;
    }
    remove_task(active, &task.id);
    if !track_done {
        return;
    }
    if done.iter().any(|t: &TaskDto| t.id == task.id) {
        upsert_task(done, task);
    } else {
        prepend_unique(done, task);
    }
}

/// Descarta una tarea eliminada de ambas listas.
pub fn apply_task_removal(active: &mut Vec<TaskDto>, done: &mut Vec<TaskDto>, id: &str) {
    remove_task(active, id);
    remove_task(done, id);
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;

    fn task(id: &str, stage: TaskStageDto, title: &str) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: "w".to_string(),
            title: title.to_string(),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: TaskStatusDto::Running,
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            actions: Default::default(),
            stage,
            needs_attention: false,
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    fn ids(list: &[TaskDto]) -> Vec<&str> {
        list.iter().map(|t: &TaskDto| t.id.as_str()).collect()
    }

    #[test]
    fn upsert_replaces_in_place_or_appends() {
        let mut list: Vec<TaskDto> = vec![
            task("a", TaskStageDto::Working, "a"),
            task("b", TaskStageDto::Working, "b"),
        ];
        upsert_task(&mut list, task("a", TaskStageDto::Attention, "a2"));
        upsert_task(&mut list, task("c", TaskStageDto::Working, "c"));
        assert_eq!(ids(&list), ["a", "b", "c"]);
        assert_eq!(list[0].title, "a2");
    }

    #[test]
    fn remove_reports_whether_it_was_present() {
        let mut list: Vec<TaskDto> = vec![task("a", TaskStageDto::Working, "a")];
        assert!(!remove_task(&mut list, "zz"));
        assert!(remove_task(&mut list, "a"));
        assert!(list.is_empty());
    }

    #[test]
    fn prepend_dedupes() {
        let mut list: Vec<TaskDto> = vec![
            task("a", TaskStageDto::Done, "a"),
            task("b", TaskStageDto::Done, "b"),
        ];
        prepend_unique(&mut list, task("b", TaskStageDto::Done, "b2"));
        assert_eq!(ids(&list), ["b", "a"]);
        assert_eq!(list[0].title, "b2");
    }

    #[test]
    fn active_change_updates_active_and_leaves_done() {
        let mut active: Vec<TaskDto> = vec![task("a", TaskStageDto::Working, "a")];
        let mut done: Vec<TaskDto> = vec![task("d", TaskStageDto::Done, "d")];
        apply_task_change(
            &mut active,
            &mut done,
            task("a", TaskStageDto::Attention, "a2"),
            true,
        );
        assert_eq!(active[0].title, "a2");
        assert_eq!(ids(&done), ["d"]);
    }

    #[test]
    fn finished_task_leaves_active_and_is_prepended_to_done_when_tracked() {
        let mut active: Vec<TaskDto> = vec![task("a", TaskStageDto::Review, "a")];
        let mut done: Vec<TaskDto> = vec![task("d", TaskStageDto::Done, "d")];
        apply_task_change(
            &mut active,
            &mut done,
            task("a", TaskStageDto::Done, "a"),
            true,
        );
        assert!(active.is_empty());
        assert_eq!(ids(&done), ["a", "d"]);
    }

    #[test]
    fn finished_task_is_not_added_to_done_when_untracked() {
        let mut active: Vec<TaskDto> = vec![task("a", TaskStageDto::Review, "a")];
        let mut done: Vec<TaskDto> = Vec::new();
        apply_task_change(
            &mut active,
            &mut done,
            task("a", TaskStageDto::Done, "a"),
            false,
        );
        assert!(active.is_empty());
        assert!(done.is_empty());
    }

    #[test]
    fn already_done_task_is_replaced_in_place() {
        let mut active: Vec<TaskDto> = Vec::new();
        let mut done: Vec<TaskDto> = vec![
            task("x", TaskStageDto::Done, "x"),
            task("d", TaskStageDto::Done, "d"),
        ];
        apply_task_change(
            &mut active,
            &mut done,
            task("d", TaskStageDto::Done, "d2"),
            true,
        );
        assert_eq!(ids(&done), ["x", "d"]);
        assert_eq!(done[1].title, "d2");
    }

    #[test]
    fn reopened_task_moves_from_done_to_active() {
        let mut active: Vec<TaskDto> = Vec::new();
        let mut done: Vec<TaskDto> = vec![task("d", TaskStageDto::Done, "d")];
        apply_task_change(
            &mut active,
            &mut done,
            task("d", TaskStageDto::Working, "d"),
            true,
        );
        assert_eq!(ids(&active), ["d"]);
        assert!(done.is_empty());
    }

    #[test]
    fn removal_drops_from_both_lists() {
        let mut active: Vec<TaskDto> = vec![task("a", TaskStageDto::Working, "a")];
        let mut done: Vec<TaskDto> = vec![task("d", TaskStageDto::Done, "d")];
        apply_task_removal(&mut active, &mut done, "a");
        apply_task_removal(&mut active, &mut done, "d");
        assert!(active.is_empty() && done.is_empty());
    }
}
