/// Actividad de los subagentes lanzados por el agente principal de una tarea.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct SubagentActivity {
    /// Subagentes en ejecución o en cola.
    pub active: u32,
    /// Subagentes bloqueados esperando una respuesta del usuario.
    pub waiting: u32,
}

impl SubagentActivity {
    pub fn new(active: u32, waiting: u32) -> Self {
        Self { active, waiting }
    }

    pub fn is_working(&self) -> bool {
        self.active > 0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_is_not_working() {
        assert!(!SubagentActivity::default().is_working());
    }

    #[test]
    fn active_subagents_mean_working() {
        assert!(SubagentActivity::new(1, 0).is_working());
        assert!(!SubagentActivity::new(0, 2).is_working());
    }
}
