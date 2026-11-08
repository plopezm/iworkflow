package components

import "context"

// TabTitle is the title of the Hooks tab, in the language of the request.
func TabTitle(ctx context.Context) string { return msgTitle.T(ctx) }
