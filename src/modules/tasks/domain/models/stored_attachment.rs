/// Metadatos de una imagen guardada en disco; `path` es relativo al directorio de adjuntos.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StoredAttachment {
    pub id: String,
    pub name: String,
    pub mime_type: String,
    pub path: String,
    pub size_bytes: u64,
}
