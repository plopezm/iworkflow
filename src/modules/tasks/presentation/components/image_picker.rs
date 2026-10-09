use leptos::ev::{ClipboardEvent, Event};
use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;
use crate::modules::tasks::presentation::state::image_attachments::{
    ACCEPT_ATTRIBUTE, ImageAttachments, hint_text, preview_data_url,
};

/// Lectura de ficheros del navegador (solo hidratación).
#[cfg(feature = "hydrate")]
mod browser {
    use leptos::prelude::Set;
    use wasm_bindgen::JsCast;
    use wasm_bindgen::closure::Closure;
    use web_sys::{File, FileList, FileReader};

    use super::ImageAttachments;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;
    use crate::modules::tasks::presentation::state::image_attachments::{
        ImageRejection, base64_from_data_url,
    };

    pub fn read_files(files: FileList, attachments: ImageAttachments) {
        attachments.error.set(None);
        for index in 0..files.length() {
            if let Some(file) = files.get(index) {
                read_file(file, attachments);
            }
        }
    }

    fn read_file(file: File, attachments: ImageAttachments) {
        let name: String = file.name();
        let mime_type: String = file.type_();
        // `size` es un f64 en JS; el valor es un entero no negativo.
        let size: usize = file.size() as usize;
        if let Err(rejection) = attachments.check_file(&name, &mime_type, size) {
            attachments.reject(rejection);
            return;
        }
        let reader: FileReader = match FileReader::new() {
            Ok(reader) => reader,
            Err(_) => {
                attachments.reject(ImageRejection::Unreadable(name));
                return;
            }
        };
        let loaded_reader: FileReader = reader.clone();
        let loaded_name: String = name.clone();
        let loaded_mime: String = mime_type.clone();
        let on_load = Closure::once_into_js(move || {
            let data: Option<String> = loaded_reader
                .result()
                .ok()
                .and_then(|value| value.as_string())
                .and_then(|url: String| base64_from_data_url(&url).map(str::to_string));
            match data {
                Some(data) => attachments.push(ImageAttachmentDto {
                    name: loaded_name,
                    mime_type: loaded_mime,
                    data,
                }),
                None => attachments.reject(ImageRejection::Unreadable(loaded_name)),
            }
        });
        let error_name: String = name.clone();
        let on_error = Closure::once_into_js(move || {
            attachments.reject(ImageRejection::Unreadable(error_name));
        });
        reader.set_onload(Some(on_load.unchecked_ref()));
        reader.set_onerror(Some(on_error.unchecked_ref()));
        if reader.read_as_data_url(&file).is_err() {
            attachments.reject(ImageRejection::Unreadable(name));
        }
    }
}

/// Lee los ficheros de un `<input type=file>` y limpia el input para poder repetir selección.
#[cfg(feature = "hydrate")]
fn handle_input_change(ev: Event, attachments: ImageAttachments) {
    use wasm_bindgen::JsCast;
    let input: Option<web_sys::HtmlInputElement> = ev
        .target()
        .and_then(|t: web_sys::EventTarget| t.dyn_into().ok());
    if let Some(input) = input {
        if let Some(files) = input.files() {
            browser::read_files(files, attachments);
        }
        input.set_value("");
    }
}

#[cfg(not(feature = "hydrate"))]
fn handle_input_change(_ev: Event, _attachments: ImageAttachments) {}

/// Adjunta las imágenes pegadas desde el portapapeles; el texto pegado sigue su curso normal.
#[cfg(feature = "hydrate")]
pub fn handle_paste_images(ev: ClipboardEvent, attachments: ImageAttachments) {
    let files: Option<web_sys::FileList> = ev
        .clipboard_data()
        .and_then(|data: web_sys::DataTransfer| data.files());
    if let Some(files) = files
        && files.length() > 0
    {
        ev.prevent_default();
        browser::read_files(files, attachments);
    }
}

#[cfg(not(feature = "hydrate"))]
pub fn handle_paste_images(_ev: ClipboardEvent, _attachments: ImageAttachments) {}

/// Callback listo para `on:paste` de cualquier textarea asociado a `attachments`.
pub fn paste_callback(attachments: ImageAttachments) -> Callback<ClipboardEvent> {
    Callback::new(move |ev: ClipboardEvent| handle_paste_images(ev, attachments))
}

/// Botón compacto de adjuntar: icono, contador y límite de tamaño en una sola línea.
#[component]
pub fn ImageAttachButton(attachments: ImageAttachments) -> impl IntoView {
    let limits = attachments.limits;
    let at_limit: Signal<bool> = Signal::derive(move || attachments.total() >= limits.max_images);

    view! {
        <label
            class="btn-secondary cursor-pointer"
            class:opacity-50=move || at_limit.get()
            title=format!("Adjuntar imágenes (máx. {} MiB cada una)", limits.max_bytes / (1024 * 1024))
        >
            <svg
                class="h-4 w-4 shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
            >
                <path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
            </svg>
            <span class="sr-only sm:not-sr-only">"Adjuntar"</span>
            <span class="text-[10px] font-normal tracking-normal text-muted normal-case">
                {move || hint_text(attachments.total(), limits)}
            </span>
            <input
                type="file"
                class="sr-only"
                accept=ACCEPT_ATTRIBUTE
                multiple
                disabled=move || at_limit.get()
                on:change=move |ev: Event| handle_input_change(ev, attachments)
            />
        </label>
    }
}

/// Error en línea y miniaturas con quitar; no ocupa espacio si no hay nada que mostrar.
#[component]
pub fn ImagePreviews(attachments: ImageAttachments) -> impl IntoView {
    view! {
        <Show when=move || attachments.error.with(Option::is_some)>
            <p class="text-sm text-error" role="alert">
                {move || attachments.error.get().unwrap_or_default()}
            </p>
        </Show>
        <Show when=move || !attachments.is_empty()>
            <ul class="flex flex-wrap gap-2">
                {move || {
                    attachments
                        .images
                        .get()
                        .into_iter()
                        .enumerate()
                        .map(|(index, image): (usize, ImageAttachmentDto)| {
                            view! {
                                <li class="relative h-20 w-20 overflow-hidden rounded-md border border-border bg-surface">
                                    <img
                                        src=preview_data_url(&image)
                                        alt=image.name.clone()
                                        class="h-full w-full object-cover"
                                    />
                                    <button
                                        type="button"
                                        class="absolute top-0 right-0 flex h-7 w-7 items-center justify-center bg-bg/80 text-fg hover:text-error"
                                        aria-label=format!("Quitar {}", image.name)
                                        on:click=move |_| attachments.remove(index)
                                    >
                                        "×"
                                    </button>
                                </li>
                            }
                        })
                        .collect_view()
                }}
            </ul>
        </Show>
    }
}

/// Selector completo (botón + previsualizaciones) para formularios sin barra de acciones propia.
#[component]
pub fn ImagePicker(attachments: ImageAttachments) -> impl IntoView {
    view! {
        <div class="stack gap-2">
            <div class="row">
                <ImageAttachButton attachments />
            </div>
            <ImagePreviews attachments />
        </div>
    }
}
