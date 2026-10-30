package models

import (
	"strings"
	"unicode"
)

// relatedLabelFallback names a project whose name has no usable characters.
const relatedLabelFallback = "project"

// relatedLabelDisambiguatorLength is how many leading characters of the workspace id tell apart two related
// projects that share a name.
const relatedLabelDisambiguatorLength = 8

// RelatedProjectLabels returns, in the order of refs, the label each related project has in an @ reference
// (@label:path/to/file). A label has no whitespace, so the reference stays one token. Two projects that share a
// name get their short id appended, so a label always names exactly one project of the set.
func RelatedProjectLabels(refs []WorkspaceRef) []string {
	bases := make([]string, len(refs))
	counts := make(map[string]int, len(refs))
	for index, ref := range refs {
		bases[index] = relatedLabelBase(ref.Name)
		counts[bases[index]]++
	}
	labels := make([]string, len(refs))
	for index, ref := range refs {
		labels[index] = bases[index]
		if counts[bases[index]] > 1 {
			labels[index] += "-" + shortWorkspaceID(ref.ID)
		}
	}
	return labels
}

// relatedLabelBase turns a project name into a token: every run of whitespace, and the ":" that separates label
// and path, becomes a dash.
func relatedLabelBase(name string) string {
	var builder strings.Builder
	pendingDash := false
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsSpace(r) || r == ':' {
			pendingDash = true
			continue
		}
		if pendingDash && builder.Len() > 0 {
			builder.WriteByte('-')
		}
		pendingDash = false
		builder.WriteRune(r)
	}
	if builder.Len() == 0 {
		return relatedLabelFallback
	}
	return builder.String()
}

func shortWorkspaceID(id string) string {
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) > relatedLabelDisambiguatorLength {
		return compact[:relatedLabelDisambiguatorLength]
	}
	return compact
}
