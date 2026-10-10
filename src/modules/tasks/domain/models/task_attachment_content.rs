/// Contenido binario de una imagen adjunta de una tarea, listo para servirse tal cual.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskAttachmentContent {
    pub mime_type: String,
    pub bytes: Vec<u8>,
}
