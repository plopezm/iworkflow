package models

import (
	"strconv"
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
// name get their short id appended. When that label still equals another project's label, the id part grows one
// character at a time; a workspace listed twice cannot be told apart by its id, so it gets a numeric suffix. Every
// label of the set is different, and each one depends only on its project, not on the order of refs.
func RelatedProjectLabels(refs []WorkspaceRef) []string {
	bases := make([]string, len(refs))
	counts := make(map[string]int, len(refs))
	for index, ref := range refs {
		bases[index] = relatedLabelBase(ref.Name)
		counts[bases[index]]++
	}
	// widths holds how many leading id characters each label carries: none for a name that is unique in the set.
	widths := make([]int, len(refs))
	for index := range refs {
		if counts[bases[index]] > 1 {
			widths[index] = relatedLabelDisambiguatorLength
		}
	}
	for {
		labels := buildRelatedLabels(bases, widths, refs)
		colliding := collidingIndexes(labels)
		if len(colliding) == 0 {
			return labels
		}
		grown := false
		for _, index := range colliding {
			if widths[index] > 0 && widths[index] < len(compactWorkspaceID(refs[index].ID)) {
				widths[index]++
				grown = true
			}
		}
		if !grown {
			return numberRepeatedLabels(labels)
		}
	}
}

// buildRelatedLabels joins each base with the id prefix of its width, when it has one.
func buildRelatedLabels(bases []string, widths []int, refs []WorkspaceRef) []string {
	labels := make([]string, len(refs))
	for index, ref := range refs {
		labels[index] = bases[index]
		if widths[index] > 0 {
			labels[index] += "-" + workspaceIDPrefix(ref.ID, widths[index])
		}
	}
	return labels
}

// collidingIndexes returns the positions whose label appears more than once.
func collidingIndexes(labels []string) []int {
	counts := make(map[string]int, len(labels))
	for _, label := range labels {
		counts[label]++
	}
	var colliding []int
	for index, label := range labels {
		if counts[label] > 1 {
			colliding = append(colliding, index)
		}
	}
	return colliding
}

// numberRepeatedLabels gives each repeat of a label after its first one a numeric suffix that no label of the set uses.
func numberRepeatedLabels(labels []string) []string {
	taken := make(map[string]bool, len(labels))
	for _, label := range labels {
		taken[label] = true
	}
	seen := make(map[string]bool, len(labels))
	for index, label := range labels {
		if !seen[label] {
			seen[label] = true
			continue
		}
		for ordinal := 2; ; ordinal++ {
			candidate := label + "-" + strconv.Itoa(ordinal)
			if !taken[candidate] {
				taken[candidate] = true
				labels[index] = candidate
				break
			}
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

// workspaceIDPrefix is the first width characters of the workspace id without its dashes, or all of them when it has fewer.
func workspaceIDPrefix(id string, width int) string {
	compact := compactWorkspaceID(id)
	if len(compact) > width {
		return compact[:width]
	}
	return compact
}

func compactWorkspaceID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}
