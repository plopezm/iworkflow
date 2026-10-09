use crate::modules::tasks::domain::errors::task_error::TaskError;

/// Máximo de imágenes adjuntas por mensaje.
pub const MAX_IMAGES_PER_MESSAGE: usize = 5;
/// Tamaño máximo (decodificado) de cada imagen: 5 MiB.
pub const MAX_IMAGE_BYTES: usize = 5 * 1024 * 1024;
/// Tipos MIME de imagen admitidos.
pub const ALLOWED_IMAGE_MIME_TYPES: [&str; 4] =
    ["image/png", "image/jpeg", "image/gif", "image/webp"];

/// Imagen adjunta a un mensaje del usuario; `data` está codificada en base64.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ImageAttachment {
    pub name: String,
    pub mime_type: String,
    pub data: String,
}

impl ImageAttachment {
    /// Valida una imagen individual y la devuelve normalizada.
    pub fn validated(self) -> Result<Self, TaskError> {
        if !ALLOWED_IMAGE_MIME_TYPES.contains(&self.mime_type.as_str()) {
            return Err(TaskError::InvalidAttachment(format!(
                "{}: tipo de imagen no admitido ({})",
                self.name, self.mime_type
            )));
        }
        if self.data.is_empty() {
            return Err(TaskError::InvalidAttachment(format!(
                "{}: la imagen está vacía",
                self.name
            )));
        }
        if self.decoded_len() > MAX_IMAGE_BYTES {
            return Err(TaskError::InvalidAttachment(format!(
                "{}: la imagen supera los 5 MiB",
                self.name
            )));
        }
        Ok(self)
    }

    /// Valida el conjunto de imágenes de un mensaje.
    pub fn validate_all(images: Vec<Self>) -> Result<Vec<Self>, TaskError> {
        if images.len() > MAX_IMAGES_PER_MESSAGE {
            return Err(TaskError::InvalidAttachment(format!(
                "máximo {MAX_IMAGES_PER_MESSAGE} imágenes por mensaje"
            )));
        }
        images.into_iter().map(Self::validated).collect()
    }

    /// Tamaño decodificado estimado a partir de la longitud del base64.
    fn decoded_len(&self) -> usize {
        let padding: usize = self
            .data
            .bytes()
            .rev()
            .take_while(|b: &u8| *b == b'=')
            .count();
        (self.data.len() / 4 * 3 + (self.data.len() % 4) * 3 / 4).saturating_sub(padding)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn image(mime: &str, data: &str) -> ImageAttachment {
        ImageAttachment {
            name: "a.png".to_string(),
            mime_type: mime.to_string(),
            data: data.to_string(),
        }
    }

    #[test]
    fn accepts_supported_mime_types() {
        for mime in ALLOWED_IMAGE_MIME_TYPES {
            assert!(image(mime, "aGk=").validated().is_ok(), "{mime}");
        }
    }

    #[test]
    fn rejects_unsupported_mime_and_empty_data() {
        assert!(matches!(
            image("image/svg+xml", "aGk=").validated(),
            Err(TaskError::InvalidAttachment(_))
        ));
        assert!(matches!(
            image("image/png", "").validated(),
            Err(TaskError::InvalidAttachment(_))
        ));
    }

    #[test]
    fn enforces_decoded_size_limit() {
        let at_limit: String = "A".repeat(MAX_IMAGE_BYTES / 3 * 4);
        assert!(image("image/png", &at_limit).validated().is_ok());
        let over: String = "A".repeat(MAX_IMAGE_BYTES / 3 * 4 + 4);
        assert!(matches!(
            image("image/png", &over).validated(),
            Err(TaskError::InvalidAttachment(_))
        ));
    }

    #[test]
    fn enforces_max_images_per_message() {
        let ok: Vec<ImageAttachment> = vec![image("image/png", "aGk="); MAX_IMAGES_PER_MESSAGE];
        assert_eq!(ImageAttachment::validate_all(ok).map(|v| v.len()), Ok(5));
        let too_many: Vec<ImageAttachment> =
            vec![image("image/png", "aGk="); MAX_IMAGES_PER_MESSAGE + 1];
        assert!(matches!(
            ImageAttachment::validate_all(too_many),
            Err(TaskError::InvalidAttachment(_))
        ));
    }
}
