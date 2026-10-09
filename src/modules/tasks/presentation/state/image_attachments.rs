use leptos::prelude::*;
use thiserror::Error;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;

/// Máximo de imágenes por mensaje (refleja `MAX_IMAGES_PER_MESSAGE` del dominio).
pub const MAX_IMAGES: usize = 5;
/// Tamaño máximo de cada imagen en bytes (refleja `MAX_IMAGE_BYTES` del dominio): 4 MiB.
pub const MAX_IMAGE_BYTES: usize = 4 * 1024 * 1024;
/// Tipos MIME admitidos (refleja `ALLOWED_IMAGE_MIME_TYPES` del dominio).
pub const ALLOWED_MIME_TYPES: [&str; 4] = ["image/png", "image/jpeg", "image/gif", "image/webp"];
/// Valor del atributo `accept` del selector de ficheros.
pub const ACCEPT_ATTRIBUTE: &str = "image/png,image/jpeg,image/gif,image/webp";

/// Límites aplicados a las imágenes de un mensaje.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ImageLimits {
    pub max_images: usize,
    pub max_bytes: usize,
}

impl Default for ImageLimits {
    fn default() -> Self {
        Self {
            max_images: MAX_IMAGES,
            max_bytes: MAX_IMAGE_BYTES,
        }
    }
}

/// Motivo por el que se rechaza una imagen en el cliente.
#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum ImageRejection {
    #[error("{0}: tipo de imagen no admitido (usa PNG, JPEG, GIF o WebP)")]
    UnsupportedType(String),
    #[error("{0}: la imagen está vacía")]
    Empty(String),
    #[error("{name}: la imagen supera los {max_mib} MiB")]
    TooLarge { name: String, max_mib: usize },
    #[error("Máximo {0} imágenes por mensaje")]
    TooMany(usize),
    #[error("{0}: no se pudo leer la imagen")]
    Unreadable(String),
}

/// Valida un fichero antes de leerlo (tipo, tamaño y hueco disponible).
pub fn validate_file(
    name: &str,
    mime_type: &str,
    size_bytes: usize,
    current_count: usize,
    limits: ImageLimits,
) -> Result<(), ImageRejection> {
    if !ALLOWED_MIME_TYPES.contains(&mime_type) {
        return Err(ImageRejection::UnsupportedType(name.to_string()));
    }
    if size_bytes == 0 {
        return Err(ImageRejection::Empty(name.to_string()));
    }
    if size_bytes > limits.max_bytes {
        return Err(ImageRejection::TooLarge {
            name: name.to_string(),
            max_mib: limits.max_bytes / (1024 * 1024),
        });
    }
    if current_count >= limits.max_images {
        return Err(ImageRejection::TooMany(limits.max_images));
    }
    Ok(())
}

/// Tamaño decodificado estimado a partir de la longitud del base64.
fn decoded_len(data: &str) -> usize {
    let padding: usize = data.bytes().rev().take_while(|b: &u8| *b == b'=').count();
    (data.len() / 4 * 3).saturating_sub(padding)
}

/// Añade una imagen ya leída a la lista tras validarla.
pub fn add_image(
    images: &mut Vec<ImageAttachmentDto>,
    image: ImageAttachmentDto,
    limits: ImageLimits,
) -> Result<(), ImageRejection> {
    validate_file(
        &image.name,
        &image.mime_type,
        decoded_len(&image.data),
        images.len(),
        limits,
    )?;
    images.push(image);
    Ok(())
}

/// Quita la imagen en `index`; ignora índices fuera de rango.
pub fn remove_image(images: &mut Vec<ImageAttachmentDto>, index: usize) {
    if index < images.len() {
        images.remove(index);
    }
}

/// Extrae el base64 de una URL `data:<mime>;base64,<datos>`.
pub fn base64_from_data_url(url: &str) -> Option<&str> {
    url.strip_prefix("data:")?
        .split_once(";base64,")
        .map(|(_, data): (&str, &str)| data)
}

/// URL `data:` para previsualizar una imagen.
pub fn preview_data_url(image: &ImageAttachmentDto) -> String {
    format!("data:{};base64,{}", image.mime_type, image.data)
}

/// Texto de ayuda compacto: contador y tamaño máximo por imagen.
pub fn hint_text(count: usize, limits: ImageLimits) -> String {
    format!(
        "{count}/{} · {} MiB",
        limits.max_images,
        limits.max_bytes / (1024 * 1024)
    )
}

/// Estado de las imágenes adjuntas a un mensaje: lista, error en línea y límites.
#[derive(Clone, Copy)]
pub struct ImageAttachments {
    pub images: RwSignal<Vec<ImageAttachmentDto>>,
    pub error: RwSignal<Option<String>>,
    pub limits: ImageLimits,
}

impl ImageAttachments {
    pub fn new() -> Self {
        Self::with_limits(ImageLimits::default())
    }

    pub fn with_limits(limits: ImageLimits) -> Self {
        Self {
            images: RwSignal::new(Vec::new()),
            error: RwSignal::new(None),
            limits,
        }
    }

    pub fn is_empty(&self) -> bool {
        self.images.with(Vec::is_empty)
    }

    pub fn count(&self) -> usize {
        self.images.with(Vec::len)
    }

    /// Descarta imágenes y error.
    pub fn clear(&self) {
        self.images.set(Vec::new());
        self.error.set(None);
    }

    /// Entrega las imágenes actuales y vacía el estado.
    pub fn take(&self) -> Vec<ImageAttachmentDto> {
        self.error.set(None);
        self.images.get_untracked_and_clear()
    }

    pub fn remove(&self, index: usize) {
        self.images
            .update(|list: &mut Vec<ImageAttachmentDto>| remove_image(list, index));
    }

    pub fn reject(&self, rejection: ImageRejection) {
        let _ = self.error.try_set(Some(rejection.to_string()));
    }

    /// Valida un fichero contra el estado actual (antes de leerlo).
    pub fn check_file(
        &self,
        name: &str,
        mime_type: &str,
        size_bytes: usize,
    ) -> Result<(), ImageRejection> {
        validate_file(name, mime_type, size_bytes, self.count(), self.limits)
    }

    /// Añade una imagen leída; el error queda en línea. Seguro si el estado ya se liberó.
    pub fn push(&self, image: ImageAttachmentDto) {
        let limits: ImageLimits = self.limits;
        let outcome: Option<Result<(), ImageRejection>> = self
            .images
            .try_update(|list: &mut Vec<ImageAttachmentDto>| add_image(list, image, limits));
        match outcome {
            Some(Ok(())) => {
                let _ = self.error.try_set(None);
            }
            Some(Err(rejection)) => self.reject(rejection),
            None => {}
        }
    }
}

trait TakeImages {
    fn get_untracked_and_clear(&self) -> Vec<ImageAttachmentDto>;
}

impl TakeImages for RwSignal<Vec<ImageAttachmentDto>> {
    fn get_untracked_and_clear(&self) -> Vec<ImageAttachmentDto> {
        self.try_update(std::mem::take).unwrap_or_default()
    }
}

impl Default for ImageAttachments {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn image(name: &str) -> ImageAttachmentDto {
        ImageAttachmentDto {
            name: name.to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        }
    }

    #[cfg(feature = "ssr")]
    #[test]
    fn limits_mirror_domain_constants() {
        use crate::modules::tasks::domain::models::image_attachment as domain;
        assert_eq!(MAX_IMAGES, domain::MAX_IMAGES_PER_MESSAGE);
        assert_eq!(MAX_IMAGE_BYTES, domain::MAX_IMAGE_BYTES);
        assert_eq!(ALLOWED_MIME_TYPES, domain::ALLOWED_IMAGE_MIME_TYPES);
        assert_eq!(ACCEPT_ATTRIBUTE, ALLOWED_MIME_TYPES.join(","));
    }

    #[test]
    fn validate_file_accepts_valid_image() {
        let result: Result<(), ImageRejection> =
            validate_file("a.png", "image/png", 10, 0, ImageLimits::default());
        assert_eq!(result, Ok(()));
    }

    #[test]
    fn validate_file_rejects_wrong_type_empty_big_and_full() {
        let limits: ImageLimits = ImageLimits::default();
        assert_eq!(
            validate_file("a.svg", "image/svg+xml", 10, 0, limits),
            Err(ImageRejection::UnsupportedType("a.svg".to_string()))
        );
        assert_eq!(
            validate_file("a.png", "image/png", 0, 0, limits),
            Err(ImageRejection::Empty("a.png".to_string()))
        );
        assert_eq!(
            validate_file("a.png", "image/png", MAX_IMAGE_BYTES + 1, 0, limits),
            Err(ImageRejection::TooLarge {
                name: "a.png".to_string(),
                max_mib: 4
            })
        );
        assert_eq!(
            validate_file("a.png", "image/png", MAX_IMAGE_BYTES, 0, limits),
            Ok(())
        );
        assert_eq!(
            validate_file("a.png", "image/png", 10, MAX_IMAGES, limits),
            Err(ImageRejection::TooMany(MAX_IMAGES))
        );
    }

    #[test]
    fn add_stops_at_the_limit_and_remove_shifts() {
        let limits: ImageLimits = ImageLimits {
            max_images: 2,
            max_bytes: 1024,
        };
        let mut images: Vec<ImageAttachmentDto> = Vec::new();
        assert!(add_image(&mut images, image("1"), limits).is_ok());
        assert!(add_image(&mut images, image("2"), limits).is_ok());
        assert_eq!(
            add_image(&mut images, image("3"), limits),
            Err(ImageRejection::TooMany(2))
        );
        remove_image(&mut images, 0);
        remove_image(&mut images, 9);
        let names: Vec<&str> = images
            .iter()
            .map(|i: &ImageAttachmentDto| i.name.as_str())
            .collect();
        assert_eq!(names, vec!["2"]);
    }

    #[test]
    fn add_rejects_oversized_decoded_payload() {
        let limits: ImageLimits = ImageLimits {
            max_images: 5,
            max_bytes: 3,
        };
        let mut images: Vec<ImageAttachmentDto> = Vec::new();
        assert!(matches!(
            add_image(&mut images, image("big"), limits),
            Err(ImageRejection::TooLarge { .. })
        ));
        assert!(images.is_empty());
    }

    #[test]
    fn data_url_round_trip() {
        let dto: ImageAttachmentDto = image("a.png");
        let url: String = preview_data_url(&dto);
        assert_eq!(url, "data:image/png;base64,aGVsbG8=");
        assert_eq!(base64_from_data_url(&url), Some("aGVsbG8="));
        assert_eq!(base64_from_data_url("nope"), None);
    }

    #[test]
    fn max_image_size_is_four_mib() {
        assert_eq!(MAX_IMAGE_BYTES, 4 * 1024 * 1024);
    }

    #[test]
    fn hint_shows_count_and_size() {
        assert_eq!(hint_text(2, ImageLimits::default()), "2/5 · 4 MiB");
    }
}
