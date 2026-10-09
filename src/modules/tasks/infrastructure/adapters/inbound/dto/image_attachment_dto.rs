use serde::{Deserialize, Serialize};

/// Imagen adjunta que viaja cliente -> servidor; `data` está codificada en base64.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ImageAttachmentDto {
    pub name: String,
    pub mime_type: String,
    pub data: String,
}

/// Imagen adjunta ya guardada que viaja servidor -> cliente; `data` está codificada en base64.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskAttachmentDto {
    pub id: String,
    pub name: String,
    pub mime_type: String,
    pub data: String,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::{ImageAttachmentDto, TaskAttachmentDto};
    use crate::modules::tasks::domain::models::task_attachment_content::TaskAttachmentContent;

    impl From<TaskAttachmentContent> for TaskAttachmentDto {
        fn from(attachment: TaskAttachmentContent) -> Self {
            Self {
                id: attachment.id,
                name: attachment.name,
                mime_type: attachment.mime_type,
                data: attachment.data,
            }
        }
    }

    use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;

    impl From<ImageAttachmentDto> for ImageAttachment {
        fn from(dto: ImageAttachmentDto) -> Self {
            Self {
                name: dto.name,
                mime_type: dto.mime_type,
                data: dto.data,
            }
        }
    }

    impl From<ImageAttachment> for ImageAttachmentDto {
        fn from(image: ImageAttachment) -> Self {
            Self {
                name: image.name,
                mime_type: image.mime_type,
                data: image.data,
            }
        }
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
    use crate::modules::tasks::domain::models::task::NewTask;
    use crate::modules::tasks::domain::models::task_attachment_content::TaskAttachmentContent;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::CreateTaskDto;

    fn dto() -> ImageAttachmentDto {
        ImageAttachmentDto {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        }
    }

    #[test]
    fn dto_round_trips_through_the_domain() {
        let original: ImageAttachmentDto = dto();

        let domain: ImageAttachment = original.clone().into();
        let back: ImageAttachmentDto = domain.clone().into();

        assert_eq!(domain.name, "a.png");
        assert_eq!(domain.mime_type, "image/png");
        assert_eq!(domain.data, "aGVsbG8=");
        assert_eq!(back, original);
    }

    #[test]
    fn task_attachment_dto_maps_from_the_domain() {
        let dto: TaskAttachmentDto = TaskAttachmentContent {
            id: "id-1".to_string(),
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        }
        .into();

        assert_eq!(
            dto,
            TaskAttachmentDto {
                id: "id-1".to_string(),
                name: "a.png".to_string(),
                mime_type: "image/png".to_string(),
                data: "aGVsbG8=".to_string(),
            }
        );
    }

    #[test]
    fn missing_images_in_create_payload_default_to_empty() {
        let create: CreateTaskDto =
            serde_json::from_str(r#"{"workspace_id":"w","title":"t","prompt":"p"}"#)
                .expect("create");

        assert!(create.images.is_empty());
    }

    #[test]
    fn create_task_dto_maps_images_to_new_task() {
        let json: String = serde_json::to_string(&CreateTaskDto {
            images: vec![dto()],
            ..Default::default()
        })
        .expect("json");
        let parsed: CreateTaskDto = serde_json::from_str(&json).expect("parse");

        let new_task: NewTask = parsed.into();

        assert_eq!(new_task.images.len(), 1);
        assert_eq!(new_task.images[0].name, "a.png");
    }
}
