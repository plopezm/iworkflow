use serde::{Deserialize, Serialize};

/// Imagen adjunta que viaja cliente -> servidor; `data` está codificada en base64.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ImageAttachmentDto {
    pub name: String,
    pub mime_type: String,
    pub data: String,
}

/// Ruta pública (relativa al origen) con el contenido de una imagen adjunta.
pub fn attachment_url(task_id: &str, attachment_id: &str) -> String {
    format!("/api/tasks/{task_id}/attachments/{attachment_id}")
}

/// Metadatos de una imagen adjunta ya guardada (servidor -> cliente); el contenido se pide
/// a `url`, que el navegador cachea.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskAttachmentDto {
    pub id: String,
    pub name: String,
    pub mime_type: String,
    pub size_bytes: u64,
    pub url: String,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::attachment_url;
    use super::{ImageAttachmentDto, TaskAttachmentDto};
    use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;

    impl TaskAttachmentDto {
        pub fn from_stored(task_id: &str, attachment: StoredAttachment) -> Self {
            Self {
                url: attachment_url(task_id, &attachment.id),
                id: attachment.id,
                name: attachment.name,
                mime_type: attachment.mime_type,
                size_bytes: attachment.size_bytes,
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
    use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
    use crate::modules::tasks::domain::models::task::NewTask;
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
        let dto: TaskAttachmentDto = TaskAttachmentDto::from_stored(
            "task-1",
            StoredAttachment {
                id: "id-1".to_string(),
                name: "a.png".to_string(),
                mime_type: "image/png".to_string(),
                path: "task-1/id-1.png".to_string(),
                size_bytes: 5,
            },
        );

        assert_eq!(
            dto,
            TaskAttachmentDto {
                id: "id-1".to_string(),
                name: "a.png".to_string(),
                mime_type: "image/png".to_string(),
                size_bytes: 5,
                url: "/api/tasks/task-1/attachments/id-1".to_string(),
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
