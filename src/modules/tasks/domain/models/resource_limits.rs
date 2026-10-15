use serde::{Deserialize, Serialize};

use crate::modules::tasks::domain::errors::task_error::TaskError;

/// Límites de recursos de la ejecución de un agente. `None` deja el recurso sin límite.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ResourceLimits {
    /// Tope de memoria RAM en MiB.
    pub memory_mb: Option<u32>,
    /// Tope de CPU en porcentaje de un núcleo (100 = un núcleo completo).
    pub cpu_percent: Option<u32>,
}

impl ResourceLimits {
    /// `true` si no hay ningún límite que aplicar.
    pub fn is_unlimited(&self) -> bool {
        self.memory_mb.is_none() && self.cpu_percent.is_none()
    }

    /// Combina límites: cada recurso definido en `self` prevalece sobre `fallback`.
    pub fn or(self, fallback: ResourceLimits) -> ResourceLimits {
        ResourceLimits {
            memory_mb: self.memory_mb.or(fallback.memory_mb),
            cpu_percent: self.cpu_percent.or(fallback.cpu_percent),
        }
    }

    /// Rechaza límites definidos a cero (no tienen sentido para cgroups).
    pub fn validated(self) -> Result<Self, TaskError> {
        if self.memory_mb == Some(0) || self.cpu_percent == Some(0) {
            return Err(TaskError::InvalidLimits);
        }
        Ok(self)
    }

    /// Propiedades de `systemd-run -p` equivalentes a los límites definidos.
    pub fn systemd_properties(&self) -> Vec<String> {
        let memory: Option<String> = self.memory_mb.map(|mb: u32| format!("MemoryMax={mb}M"));
        let cpu: Option<String> = self
            .cpu_percent
            .map(|percent: u32| format!("CPUQuota={percent}%"));
        memory.into_iter().chain(cpu).collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_is_unlimited() {
        assert!(ResourceLimits::default().is_unlimited());
    }

    #[test]
    fn systemd_properties_cover_defined_resources_only() {
        let limits: ResourceLimits = ResourceLimits {
            memory_mb: Some(2048),
            cpu_percent: None,
        };
        assert_eq!(limits.systemd_properties(), vec!["MemoryMax=2048M"]);
        let both: ResourceLimits = ResourceLimits {
            memory_mb: Some(512),
            cpu_percent: Some(150),
        };
        assert_eq!(
            both.systemd_properties(),
            vec!["MemoryMax=512M", "CPUQuota=150%"]
        );
    }

    #[test]
    fn zero_limits_are_invalid() {
        for limits in [
            ResourceLimits {
                memory_mb: Some(0),
                cpu_percent: None,
            },
            ResourceLimits {
                memory_mb: None,
                cpu_percent: Some(0),
            },
        ] {
            assert_eq!(limits.validated(), Err(TaskError::InvalidLimits));
        }
        assert!(ResourceLimits::default().validated().is_ok());
    }

    #[test]
    fn or_prefers_own_values_per_resource() {
        let task: ResourceLimits = ResourceLimits {
            memory_mb: Some(1024),
            cpu_percent: None,
        };
        let global: ResourceLimits = ResourceLimits {
            memory_mb: Some(4096),
            cpu_percent: Some(200),
        };
        assert_eq!(
            task.or(global),
            ResourceLimits {
                memory_mb: Some(1024),
                cpu_percent: Some(200)
            }
        );
    }
}
