package outbound

import "context"

// FileSearchPort lists the files a project search can suggest.
type FileSearchPort interface {
	// Files returns every file under root that is not git-ignored, as `/`-separated relative
	// paths. It is empty when root is not a git repository.
	Files(ctx context.Context, root string) ([]string, error)
}
