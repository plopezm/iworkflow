package state

import "strings"

// ExplanationPrompt is the prompt sent to the task agent to obtain the explanation of the changes.
const ExplanationPrompt = "Genera un informe explicativo de los cambios realizados en esta tarea. Para cada fichero modificado explica qué cambió y por qué. Incluye además las decisiones clave, los trade-offs y las alternativas descartadas. Responde en markdown con encabezados, y no hagas ningún cambio de código adicional."

// FilePatch is one file of the diff extracted from a unified patch.
type FilePatch struct {
	Name  string
	Lines []string
}

// SplitPatchByFile splits a unified patch into one chunk per `diff --git` header. Lines before the first header
// (or the whole patch when there is no header) are grouped into a nameless chunk so nothing is lost. An empty patch
// yields no chunks.
func SplitPatchByFile(patch string) []FilePatch {
	var files []FilePatch
	for _, line := range splitPatchLines(patch) {
		if strings.HasPrefix(line, "diff --git") {
			files = append(files, FilePatch{Name: FileNameFromHeader(line), Lines: []string{line}})
			continue
		}
		if len(files) == 0 {
			files = append(files, FilePatch{Name: "", Lines: []string{line}})
			continue
		}
		files[len(files)-1].Lines = append(files[len(files)-1].Lines, line)
	}
	return files
}

// FileNameFromHeader extracts the `b/` path of a `diff --git a/x b/x` header (the whole header when unparsable).
func FileNameFromHeader(header string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(header, "diff --git"))
	if index := strings.LastIndex(rest, " b/"); index >= 0 {
		return rest[index+3:]
	}
	return rest
}

// FindExplanation finds the agent's answer to the prompt, looking only at the transcript items after the given
// baseline. The answer is ready when the last matching user message is followed by at least one assistant message,
// nothing is streaming, no tool is still running and the last relevant item is an assistant message. The texts of
// the assistant answers are joined by blank lines.
func FindExplanation(transcript Transcript, prompt string, since int) (string, bool) {
	if transcript.IsStreaming() {
		return "", false
	}
	if since < 0 || since > len(transcript.Items) {
		return "", false
	}
	items := transcript.Items[since:]
	promptIndex := -1
	for index := len(items) - 1; index >= 0; index-- {
		if item, ok := items[index].(UserItem); ok && item.Text == prompt {
			promptIndex = index
			break
		}
	}
	if promptIndex < 0 {
		return "", false
	}
	after := items[promptIndex+1:]
	toolRunning := false
	for _, item := range after {
		if tool, ok := item.(ToolItem); ok && tool.IsError == nil {
			toolRunning = true
			break
		}
	}
	if toolRunning {
		return "", false
	}
	lastRelevant := -1
	for index := len(after) - 1; index >= 0; index-- {
		if _, ok := after[index].(StatusItem); ok {
			continue
		}
		if _, ok := after[index].(NoticeItem); ok {
			continue
		}
		lastRelevant = index
		break
	}
	if lastRelevant < 0 {
		return "", false
	}
	_, ok := after[lastRelevant].(AssistantItem)
	if !ok {
		return "", false
	}
	var texts []string
	for _, item := range after {
		if assistant, ok := item.(AssistantItem); ok && strings.TrimSpace(assistant.Text) != "" {
			texts = append(texts, assistant.Text)
		}
	}
	text := ""
	for index, value := range texts {
		if index > 0 {
			text += "\n\n"
		}
		text += value
	}
	if text == "" {
		return "", false
	}
	return text, true
}

// splitPatchLines splits a patch the way a unified diff reads it: lines end at a newline, an optional carriage
// return is stripped, and a final line ending does not open a new empty line.
func splitPatchLines(patch string) []string {
	lines := strings.Split(patch, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, strings.TrimSuffix(line, "\r"))
	}
	return result
}
