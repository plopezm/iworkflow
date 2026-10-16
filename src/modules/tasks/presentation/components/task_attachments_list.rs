use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::TaskAttachmentDto;

/// Imágenes adjuntas de una tarea en modo lectura; cada miniatura abre la imagen completa.
/// No ocupa espacio si no hay adjuntos.
#[component]
pub fn TaskAttachmentsList(items: Vec<TaskAttachmentDto>) -> impl IntoView {
    (!items.is_empty()).then(|| {
        view! {
            <section class="card">
                <h3 class="card-title">"Adjuntos"</h3>
                <ul class="flex flex-wrap gap-2" aria-label="Imágenes adjuntas de la tarea">
                    {items
                        .into_iter()
                        .map(|item: TaskAttachmentDto| {
                            view! {
                                <li class="h-20 w-20 overflow-hidden rounded-md border border-border bg-surface">
                                    <a href=item.url.clone() target="_blank" rel="noopener">
                                        <img
                                            src=item.url.clone()
                                            alt=item.name.clone()
                                            title=item.name.clone()
                                            class="h-full w-full object-cover"
                                        />
                                    </a>
                                </li>
                            }
                        })
                        .collect_view()}
                </ul>
            </section>
        }
    })
}
