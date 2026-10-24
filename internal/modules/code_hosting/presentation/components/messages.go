package components

import "iworkflow/internal/core/i18n"

// msgLoading is the placeholder of the lazy task panel while its body is read. It copies the Rust
// constant of the same meaning (LOADING in task_merge_request_panel.rs).
var msgLoading = i18n.Msg{En: "Loading…", Es: "Cargando…", Zh: "加载中…", De: "Wird geladen…", Fr: "Chargement…"}
