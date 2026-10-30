package models

import "strings"

// RelatedProject is one related project as the launch prompt describes it: its workspace, the label its @
// references use, and whether it has a codegraph index the agent can query.
type RelatedProject struct {
	Ref          WorkspaceRef
	Label        string
	HasCodegraph bool
}

// RelatedProjectsPrompt renders the launch section that tells the agent about the related projects of its task.
// It returns an empty string when there are no projects, so the caller appends nothing. engramAvailable reports
// whether the engram memory is installed; only then the section explains how to consult the memory of a project.
func RelatedProjectsPrompt(projects []RelatedProject, engramAvailable bool) string {
	if len(projects) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("## Related projects\n\n")
	builder.WriteString("This task relates to the projects listed below. You MAY read their files and use them as reference, ")
	builder.WriteString("but you MUST NOT edit them: every change belongs to the worktree of this task.\n\n")
	hasCodegraph := false
	for _, project := range projects {
		builder.WriteString("- `@" + project.Label + "` -> `" + project.Ref.Path + "` (" + project.Ref.Name + ")")
		if project.HasCodegraph {
			builder.WriteString(" - codegraph index available")
			hasCodegraph = true
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\nA reference written as `@<label>:<relative/path>` in the task prompt points to that file ")
	builder.WriteString("inside the root of the project with that label.\n")
	if hasCodegraph {
		builder.WriteString("\nA project with a codegraph index can be queried: pass its absolute path as `projectPath` to the codegraph tools.\n")
	}
	if engramAvailable {
		builder.WriteString("\nThe engram memory of a related project can be consulted: call `mem_current_project` with `cwd` set to ")
		builder.WriteString("its absolute path, then pass the returned name as `project` to `mem_search` or `mem_context`. ")
		builder.WriteString("This access is read-only: do not save into the memory of a related project unless the user asks you to.\n")
	}
	builder.WriteString("\nTo use the HTTP or code API of a related project, read its sources; no new endpoint is provided for it.\n")
	return builder.String()
}

// AppendPromptSection appends a section to a prompt after a blank line. An empty section leaves the prompt as it is,
// and a blank prompt yields the section alone.
func AppendPromptSection(prompt string, section string) string {
	if section == "" {
		return prompt
	}
	if strings.TrimSpace(prompt) == "" {
		return section
	}
	return prompt + "\n\n" + section
}
