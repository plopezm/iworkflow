package outbound

import "context"

// RelatedProjectProbe reports what the tools around a related project offer the agent: the codegraph index of the
// project and the engram memory of the machine. Consumer-side: the launch preparation reads it, the adapter answers
// from the file system.
type RelatedProjectProbe interface {
	// HasCodegraph reports whether the project at projectPath has a codegraph index.
	HasCodegraph(ctx context.Context, projectPath string) bool
	// EngramAvailable reports whether the engram memory is installed.
	EngramAvailable(ctx context.Context) bool
}
