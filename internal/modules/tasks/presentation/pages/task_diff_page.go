package pages

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// msgDiffTabTitle titles the diff tab of a task, copied from the Rust page (src/modules/tasks/presentation/pages/task_diff_page.rs).
var msgDiffTabTitle = i18n.Msg{En: "Diff · {title}", Es: "Diff · {title}", Zh: "差异 · {title}", De: "Diff · {title}", Fr: "Diff · {title}"}

// DiffTabTitle is the title of the diff tab of a task titled title, in the language of the request.
func DiffTabTitle(ctx context.Context, title string) string {
	return msgDiffTabTitle.Fmt(ctx, map[string]string{"title": title})
}
