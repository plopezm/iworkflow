use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStageDto;

/// Sufijo de clase CSS (`badge-{sufijo}`) asociado a una etapa del tablero.
pub fn stage_css_variant(stage: TaskStageDto) -> &'static str {
    match stage {
        TaskStageDto::Backlog => "backlog",
        TaskStageDto::Working => "working",
        TaskStageDto::Attention => "attention",
        TaskStageDto::Review => "review",
        TaskStageDto::Done => "done",
    }
}

/// Insignia de etapa con un punto animado cuando la tarea está trabajando.
#[component]
pub fn StageBadge(stage: TaskStageDto) -> impl IntoView {
    let variant: &'static str = stage_css_variant(stage);
    view! {
        <span class=format!("badge badge-{variant}")>
            {(stage == TaskStageDto::Working)
                .then(|| view! { <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-current"></span> })}
            {stage.label()}
        </span>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn maps_every_stage_to_a_css_variant() {
        assert_eq!(stage_css_variant(TaskStageDto::Backlog), "backlog");
        assert_eq!(stage_css_variant(TaskStageDto::Working), "working");
        assert_eq!(stage_css_variant(TaskStageDto::Attention), "attention");
        assert_eq!(stage_css_variant(TaskStageDto::Review), "review");
        assert_eq!(stage_css_variant(TaskStageDto::Done), "done");
    }
}
