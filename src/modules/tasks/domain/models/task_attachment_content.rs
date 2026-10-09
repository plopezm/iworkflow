/// Imagen adjunta de una tarea con su contenido; `data` está codificada en base64.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskAttachmentContent {
    pub id: String,
    pub name: String,
    pub mime_type: String,
    pub data: String,
}
