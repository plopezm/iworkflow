use leptos::prelude::*;
use leptos::server_fn::codec::{MultipartData, MultipartFormData};

use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::{
    AgentsFileDto, ApplyAgentsFileDto, ApplyAgentsFileOutcomeDto, ProjectOptionDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
    use crate::modules::agents_files::domain::models::agents_file::{
        MAX_CONTENT_BYTES, MAX_DESCRIPTION_CHARS, NewAgentsFile,
    };
    use crate::modules::agents_files::infrastructure::di::agents_files_context::AgentsFilesContext;

    /// Margen para descripciones con caracteres de varios bytes (UTF-8: hasta 4 por carácter).
    const MAX_DESCRIPTION_BYTES: usize = MAX_DESCRIPTION_CHARS * 4;
    const MAX_FIELDS: usize = 8;

    #[derive(Debug, PartialEq, Eq, thiserror::Error)]
    pub enum UploadError {
        #[error(transparent)]
        Domain(#[from] AgentsFileError),
        #[error("formulario de subida inválido: {0}")]
        Malformed(String),
    }

    pub fn context() -> Result<AgentsFilesContext, ServerFnError> {
        use_context::<AgentsFilesContext>()
            .ok_or_else(|| ServerFnError::new("AgentsFilesContext no disponible"))
    }

    pub fn to_server_error(error: AgentsFileError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    pub fn upload_to_server_error(error: UploadError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    /// Lee el campo por trozos y corta en cuanto supera `limit` bytes (`Ok(None)`).
    async fn read_capped(
        field: &mut multer::Field<'static>,
        limit: usize,
    ) -> Result<Option<Vec<u8>>, UploadError> {
        let mut buffer: Vec<u8> = Vec::new();
        while let Some(chunk) = field
            .chunk()
            .await
            .map_err(|e: multer::Error| UploadError::Malformed(e.to_string()))?
        {
            if buffer.len() + chunk.len() > limit {
                return Ok(None);
            }
            buffer.extend_from_slice(&chunk);
        }
        Ok(Some(buffer))
    }

    /// Extrae `description` y `file` del formulario, ignorando cualquier otro campo.
    pub async fn parse_upload(
        mut multipart: multer::Multipart<'static>,
    ) -> Result<NewAgentsFile, UploadError> {
        let mut description: Option<String> = None;
        let mut content: Option<Vec<u8>> = None;
        let mut seen: usize = 0;
        while let Some(mut field) = multipart
            .next_field()
            .await
            .map_err(|e: multer::Error| UploadError::Malformed(e.to_string()))?
        {
            seen += 1;
            if seen > MAX_FIELDS {
                return Err(UploadError::Malformed("demasiados campos".to_string()));
            }
            let name: Option<String> = field.name().map(str::to_string);
            match name.as_deref() {
                Some("description") => {
                    let bytes: Vec<u8> = read_capped(&mut field, MAX_DESCRIPTION_BYTES)
                        .await?
                        .ok_or(AgentsFileError::InvalidDescription)?;
                    description = Some(
                        String::from_utf8(bytes)
                            .map_err(|_| AgentsFileError::InvalidDescription)?,
                    );
                }
                Some("file") => {
                    content = Some(
                        read_capped(&mut field, MAX_CONTENT_BYTES)
                            .await?
                            .ok_or(AgentsFileError::TooLarge)?,
                    );
                }
                _ => {}
            }
        }
        Ok(NewAgentsFile {
            description: description.ok_or(AgentsFileError::InvalidDescription)?,
            content: content.ok_or(AgentsFileError::EmptyContent)?,
        })
    }

    #[cfg(test)]
    mod tests {
        use std::convert::Infallible;

        use super::*;

        const BOUNDARY: &str = "XBOUNDARY";

        fn multipart(parts: &[(&str, Vec<u8>)]) -> multer::Multipart<'static> {
            let mut body: Vec<u8> = Vec::new();
            for (name, data) in parts {
                body.extend_from_slice(
                    format!(
                        "--{BOUNDARY}\r\nContent-Disposition: form-data; name=\"{name}\"\r\n\r\n"
                    )
                    .as_bytes(),
                );
                body.extend_from_slice(data);
                body.extend_from_slice(b"\r\n");
            }
            body.extend_from_slice(format!("--{BOUNDARY}--\r\n").as_bytes());
            let stream = futures::stream::iter(vec![Ok::<Vec<u8>, Infallible>(body)]);
            multer::Multipart::new(stream, BOUNDARY)
        }

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError = to_server_error(AgentsFileError::EmptyContent);
            assert_eq!(
                error,
                ServerFnError::ServerError("el fichero está vacío".to_string())
            );
        }

        #[tokio::test]
        async fn parses_description_and_file_ignoring_other_fields() {
            let form: multer::Multipart<'static> = multipart(&[
                ("other", b"zzz".to_vec()),
                ("description", b"Reglas".to_vec()),
                ("file", b"# hola".to_vec()),
            ]);

            let parsed: NewAgentsFile = parse_upload(form).await.expect("parse");

            assert_eq!(parsed.description, "Reglas");
            assert_eq!(parsed.content, b"# hola".to_vec());
        }

        #[tokio::test]
        async fn rejects_missing_fields() {
            let no_file: Result<NewAgentsFile, UploadError> =
                parse_upload(multipart(&[("description", b"d".to_vec())])).await;
            let no_description: Result<NewAgentsFile, UploadError> =
                parse_upload(multipart(&[("file", b"x".to_vec())])).await;

            assert_eq!(
                no_file,
                Err(UploadError::Domain(AgentsFileError::EmptyContent))
            );
            assert_eq!(
                no_description,
                Err(UploadError::Domain(AgentsFileError::InvalidDescription))
            );
        }

        #[tokio::test]
        async fn stops_reading_files_over_the_cap() {
            let big: Vec<u8> = vec![b'a'; MAX_CONTENT_BYTES + 1];

            let result: Result<NewAgentsFile, UploadError> =
                parse_upload(multipart(&[("description", b"d".to_vec()), ("file", big)])).await;

            assert_eq!(result, Err(UploadError::Domain(AgentsFileError::TooLarge)));
        }

        #[tokio::test]
        async fn rejects_oversized_description() {
            let long: Vec<u8> = vec![b'a'; MAX_DESCRIPTION_BYTES + 1];

            let result: Result<NewAgentsFile, UploadError> =
                parse_upload(multipart(&[("description", long), ("file", b"x".to_vec())])).await;

            assert_eq!(
                result,
                Err(UploadError::Domain(AgentsFileError::InvalidDescription))
            );
        }
    }
}

#[server]
pub async fn list_agents_files() -> Result<Vec<AgentsFileDto>, ServerFnError> {
    use crate::modules::agents_files::domain::models::agents_file::AgentsFile;

    let files: Vec<AgentsFile> = support::context()?
        .list
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(files.into_iter().map(AgentsFileDto::from).collect())
}

#[server(input = MultipartFormData)]
pub async fn upload_agents_file(data: MultipartData) -> Result<AgentsFileDto, ServerFnError> {
    use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, NewAgentsFile};

    let multipart: multer::Multipart<'static> = data
        .into_inner()
        .ok_or_else(|| ServerFnError::new("formulario de subida no disponible"))?;
    let input: NewAgentsFile = support::parse_upload(multipart)
        .await
        .map_err(support::upload_to_server_error)?;
    let file: AgentsFile = support::context()?
        .upload
        .execute(input)
        .await
        .map_err(support::to_server_error)?;
    Ok(AgentsFileDto::from(file))
}

#[server]
pub async fn delete_agents_file(id: String) -> Result<(), ServerFnError> {
    use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;

    let id: AgentsFileId = AgentsFileId::parse(&id).map_err(support::to_server_error)?;
    support::context()?
        .delete
        .execute(id)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn list_agents_file_projects() -> Result<Vec<ProjectOptionDto>, ServerFnError> {
    use crate::modules::agents_files::domain::models::project_ref::ProjectRef;

    let projects: Vec<ProjectRef> = support::context()?
        .list_projects
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(projects.into_iter().map(ProjectOptionDto::from).collect())
}

#[server]
pub async fn apply_agents_file(
    input: ApplyAgentsFileDto,
) -> Result<ApplyAgentsFileOutcomeDto, ServerFnError> {
    use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
    use crate::modules::agents_files::domain::models::agents_file::AgentsFileId;
    use crate::modules::agents_files::domain::models::install_outcome::InstallOutcome;
    use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::apply_outcome;

    let id: AgentsFileId =
        AgentsFileId::parse(&input.agents_file_id).map_err(support::to_server_error)?;
    let result: Result<InstallOutcome, AgentsFileError> = support::context()?
        .apply
        .execute(id, &input.project_id, input.overwrite)
        .await;
    apply_outcome(result, input.overwrite).map_err(support::to_server_error)
}
