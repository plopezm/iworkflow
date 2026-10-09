use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::TaskAttachmentDto;
use crate::modules::tasks::presentation::state::image_attachments::data_url;

/// Miniaturas de imágenes ya guardadas con botón para quitarlas; no ocupa espacio si no hay.
#[component]
pub fn ExistingAttachments(
    #[prop(into)] items: Signal<Vec<TaskAttachmentDto>>,
    #[prop(into)] on_remove: Callback<String>,
) -> impl IntoView {
    view! {
        <Show when=move || items.with(|list: &Vec<TaskAttachmentDto>| !list.is_empty())>
            <ul class="flex flex-wrap gap-2" aria-label="Imágenes adjuntas">
                {move || {
                    items
                        .get()
                        .into_iter()
                        .map(|item: TaskAttachmentDto| {
                            let id: String = item.id.clone();
                            view! {
                                <li class="relative h-20 w-20 overflow-hidden rounded-md border border-border bg-surface">
                                    <img
                                        src=data_url(&item.mime_type, &item.data)
                                        alt=item.name.clone()
                                        title=item.name.clone()
                                        class="h-full w-full object-cover"
                                    />
                                    <button
                                        type="button"
                                        class="absolute top-0 right-0 flex h-7 w-7 items-center justify-center bg-bg/80 text-fg hover:text-error"
                                        aria-label=format!("Quitar imagen {}", item.name)
                                        on:click=move |_| on_remove.run(id.clone())
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
