package components

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/a-h/templ"

	"iworkflow/internal/core/markdown"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// argsPreviewChars is the longest summary of the arguments of a tool.
const argsPreviewChars = 80

// argBlockChars is the length from which an argument value is shown as a block instead of inline code.
const argBlockChars = 80

// keyArgs are the arguments that best identify a tool call, in order of preference.
var keyArgs = [...]string{"path", "command", "pattern", "query", "url", "file", "operation", "task"}

// argRow is one argument of a tool call: its key, and its value as readable text.
type argRow struct {
	Key   string
	Value string
}

// imagesLabel is the label of the images attached to a message ("📎 2 images").
func imagesLabel(ctx context.Context, count uint32) string {
	if count == 1 {
		return msgTranscriptOneImage.T(ctx)
	}
	return msgTranscriptManyImages.Fmt(ctx, map[string]string{"n": strconv.FormatUint(uint64(count), 10)})
}

// isLaunchNotice reports whether a notice dumps the launch details (args= or cwd=). Those notices are collapsed by
// default under the launch details label.
func isLaunchNotice(message string) bool {
	return strings.Contains(message, "args=") || strings.Contains(message, "cwd=")
}

// transcriptItem is the view of one item of the transcript. The task id is what a dialog answers to.
func transcriptItem(taskID string, item state.TranscriptItem) templ.Component {
	switch typed := item.(type) {
	case state.UserItem:
		return transcriptUser(typed)
	case state.AssistantItem:
		return transcriptAssistant(typed)
	case state.ToolItem:
		return transcriptTool(typed)
	case state.NoticeItem:
		return transcriptNotice(typed)
	case state.StatusItem:
		return transcriptStatus(typed)
	case state.DialogItem:
		return TranscriptDialog(taskID, typed, "")
	case state.ExitItem:
		return transcriptExit(typed)
	default:
		return templ.NopComponent
	}
}

// toolState is the readable state of a tool call and the CSS class that shows it.
func toolState(ctx context.Context, item state.ToolItem) (string, string) {
	switch {
	case item.IsError == nil:
		return msgTranscriptToolRunning.T(ctx), "tool tool-running"
	case *item.IsError:
		return msgTranscriptToolError.T(ctx), "tool tool-error"
	default:
		return msgTranscriptToolOK.T(ctx), "tool tool-ok"
	}
}

// toolClass is the CSS class of a tool call.
func toolClass(ctx context.Context, item state.ToolItem) string {
	_, class := toolState(ctx, item)
	return class
}

// toolSummary is the line that names a tool call: its name, its state and the summary of its arguments.
func toolSummary(ctx context.Context, item state.ToolItem) string {
	label, _ := toolState(ctx, item)
	return "🔧 " + item.Name + " · " + label + " · " + argsSummary(item.Args)
}

// dialogText is the line of a dialog, with whether it is still pending.
func dialogText(ctx context.Context, item state.DialogItem) string {
	label := msgTranscriptDialogPending.T(ctx)
	switch {
	case item.ClosedByTurn:
		label = msgTranscriptDialogClosed.T(ctx)
	case item.Resolved:
		label = msgTranscriptDialogAnswered.T(ctx)
	}
	return "❓ " + item.Request.Title + " (" + label + ")"
}

// exitText says how the agent process ended.
func exitText(ctx context.Context, item state.ExitItem) string {
	if item.Code == nil {
		return msgTranscriptStoppedNoCode.T(ctx)
	}
	return msgTranscriptStoppedCode.Fmt(ctx, map[string]string{"code": strconv.FormatInt(int64(*item.Code), 10)})
}

// noticeClass is the CSS class of a notice of the given level.
func noticeClass(level string) string {
	switch level {
	case dto.NoticeLevelWarning:
		return "notice notice-warn"
	case "error":
		return "notice notice-error"
	default:
		return "notice notice-info"
	}
}

// previewText collapses the whitespace of a text into single spaces and cuts it to argsPreviewChars characters.
func previewText(text string) string {
	singleLine := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(singleLine) > argsPreviewChars {
		return string([]rune(singleLine)[:argsPreviewChars]) + "…"
	}
	return singleLine
}

// argsSummary is a readable summary of the arguments of a tool call (its path or its command) instead of the raw
// JSON. Without a known key nothing is summarized: any other value could expose sensitive data.
func argsSummary(args string) string {
	object, ok := parseArgsObject(args)
	if !ok {
		return previewText(args)
	}
	for _, key := range keyArgs {
		if value, isString := object[key].(string); isString {
			return previewText(value)
		}
	}
	return ""
}

// argRows lists the arguments of a tool call as key and value pairs, in key order. Arguments that are not a JSON
// object are one row without key, and none when they are blank.
func argRows(args string) []argRow {
	object, ok := parseArgsObject(args)
	if !ok {
		if strings.TrimSpace(args) == "" {
			return nil
		}
		return []argRow{{Value: args}}
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]argRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, argRow{Key: key, Value: argText(object[key])})
	}
	return rows
}

// isBlockArg reports whether a value is shown as a block: it spans lines or it is long.
func isBlockArg(value string) bool {
	return strings.Contains(value, "\n") || len(value) > argBlockChars
}

// parseArgsObject reads the arguments as a JSON object. Numbers stay as they were written.
func parseArgsObject(args string) (map[string]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(args))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}

// argText is the text of an argument value: a string as it is, anything else as its compact JSON. The value comes
// from the decoder, so it always encodes back.
func argText(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

// markdownHTML renders a message as sanitized HTML. A message that cannot be rendered is shown as escaped text.
func markdownHTML(text string) string {
	rendered, err := markdown.Render(text)
	if err != nil {
		return html.EscapeString(text)
	}
	return rendered
}
