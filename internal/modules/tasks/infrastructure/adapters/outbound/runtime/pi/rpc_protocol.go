//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

const (
	// maxToolTextChars limits the tool output and the tool arguments that are kept.
	maxToolTextChars = 20_000
	truncatedSuffix  = "\n… (truncado)"
	// jobsStatusKey is the status line under which pi publishes its background jobs ("⧗ 2 jobs").
	jobsStatusKey = "gentle-jobs"
	// subagentsWidgetKey is the widget under which the subagent extension publishes its activity.
	subagentsWidgetKey = "gentle-agents"
	// subagentsSchemaPrefix is the prefix of the schema of the subagent payloads this code can read.
	subagentsSchemaPrefix = "gentle-agents.activity/"
	interruptedMessage    = "ejecución interrumpida"
)

// encodeCommand returns the stdin record (without its line ending) of a command. requestID is the id of
// the record. The boolean is false when pi has nothing to receive for the command (a mode change).
func encodeCommand(command models.AgentCommand, requestID string) (string, bool, error) {
	var record map[string]any
	switch typed := command.(type) {
	case models.PromptCommand:
		// streamingBehavior avoids a pi error if the agent were still busy.
		record = withImages(map[string]any{
			"id":                requestID,
			"type":              "prompt",
			"message":           typed.Message,
			"streamingBehavior": "followUp",
		}, typed.Images)
	case models.SteerCommand:
		record = withImages(map[string]any{"id": requestID, "type": "steer", "message": typed.Message}, typed.Images)
	case models.FollowUpCommand:
		record = withImages(map[string]any{"id": requestID, "type": "follow_up", "message": typed.Message}, typed.Images)
	case models.AbortCommand:
		record = map[string]any{"id": requestID, "type": "abort"}
	case models.SetModeCommand:
		// Pi exposes no modes: the use case filters them by capability, and there is nothing to send.
		return "", false, nil
	case models.AnswerDialogCommand:
		answer, err := answerRecord(typed)
		if err != nil {
			return "", false, err
		}
		record = answer
	default:
		return "", false, werrors.Unsupported(fmt.Sprintf("comando de agente no soportado: %v", command))
	}
	line, err := compactJSON(record)
	if err != nil {
		return "", false, err
	}
	return line, true, nil
}

// answerRecord builds the extension_ui_response record that answers a dialog of the agent.
func answerRecord(command models.AnswerDialogCommand) (map[string]any, error) {
	record := map[string]any{"type": "extension_ui_response", "id": command.DialogID}
	switch answer := command.Answer.(type) {
	case models.ValueAnswer:
		record["value"] = answer.Value
	case models.ConfirmedAnswer:
		record["confirmed"] = answer.Confirmed
	case models.CancelledAnswer:
		record["cancelled"] = true
	default:
		return nil, werrors.Unsupported(fmt.Sprintf("respuesta de diálogo no soportada: %v", command.Answer))
	}
	return record, nil
}

// withImages adds the images field of pi only when there are images to send.
func withImages(record map[string]any, images []models.ImageAttachment) map[string]any {
	if len(images) == 0 {
		return record
	}
	encoded := make([]map[string]any, 0, len(images))
	for _, image := range images {
		encoded = append(encoded, map[string]any{"type": "image", "data": image.Data, "mimeType": image.MimeType})
	}
	record["images"] = encoded
	return record
}

// decode translates one stdout line into domain events. It returns nothing for lines that are not JSON
// or do not matter.
func decode(line string) []models.AgentEvent {
	record, ok := parseJSON(line)
	if !ok {
		return nil
	}
	var events []models.AgentEvent
	if event, found := decodeRecord(stringOf(record, "type"), record); found {
		events = append(events, event)
	}
	if limit, found := usageLimitOf(events); found {
		events = append(events, limit)
	}
	if activity, found := decodeSubagents(record); found {
		events = append(events, activity)
	}
	return events
}

// decodeRecord maps the record kind to its event. The boolean is false when the kind has no event.
func decodeRecord(kind string, record any) (models.AgentEvent, bool) {
	switch kind {
	case "agent_start":
		return models.RunStarted{}, true
	case "agent_settled":
		return models.RunSettled{}, true
	case "message_end":
		return decodeMessage(field(record, "message"))
	case "message_update":
		return decodeUpdate(record)
	case "tool_execution_start":
		return models.ToolStarted{
			CallID:   stringOf(record, "toolCallId"),
			ToolName: stringOf(record, "toolName"),
			Args:     truncate(argsText(record)),
		}, true
	case "tool_execution_update":
		return models.ToolUpdated{
			CallID: stringOf(record, "toolCallId"),
			Output: truncate(contentText(field(field(record, "partialResult"), "content"))),
		}, true
	case "tool_execution_end":
		isError, _ := boolField(record, "isError")
		return models.ToolFinished{
			CallID:   stringOf(record, "toolCallId"),
			ToolName: stringOf(record, "toolName"),
			Output:   truncate(contentText(field(field(record, "result"), "content"))),
			IsError:  isError,
		}, true
	case "queue_update":
		return models.QueueChanged{
			Steering: stringList(field(record, "steering")),
			FollowUp: stringList(field(record, "followUp")),
		}, true
	case "auto_retry_start":
		attempt, _ := uintField(record, "attempt")
		maxAttempts, _ := uintField(record, "maxAttempts")
		return notice(models.NoticeWarning, fmt.Sprintf("Reintento %d/%d: %s",
			attempt, maxAttempts, stringOf(record, "errorMessage"))), true
	case "auto_retry_end":
		if success, ok := boolField(record, "success"); ok && !success {
			return notice(models.NoticeError, "Reintentos agotados: "+stringOf(record, "finalError")), true
		}
		return nil, false
	case "compaction_start":
		return notice(models.NoticeInfo, "Compactando contexto ("+stringOf(record, "reason")+")"), true
	case "compaction_end":
		if errorText, ok := strField(record, "errorMessage"); ok {
			return notice(models.NoticeWarning, "Compactación fallida: "+errorText), true
		}
		return nil, false
	case "extension_error":
		return notice(models.NoticeError, fmt.Sprintf("Error de extensión (%s): %s",
			stringOf(record, "extensionPath"), stringOf(record, "error"))), true
	case "response":
		if success, ok := boolField(record, "success"); ok && !success {
			return notice(models.NoticeError, fmt.Sprintf("%s rechazado: %s",
				stringOf(record, "command"), stringOf(record, "error"))), true
		}
		return nil, false
	case "extension_ui_request":
		return decodeUIRequest(record)
	default:
		return nil, false
	}
}

// usageLimitOf reports the usage limit of an assistant message that failed because of one. pi only
// describes the error in text, without the instant of renewal.
func usageLimitOf(events []models.AgentEvent) (models.AgentEvent, bool) {
	for _, event := range events {
		assistant, ok := event.(models.AssistantMessage)
		if !ok || assistant.Error == nil || !models.LooksLikeUsageLimit(*assistant.Error) {
			continue
		}
		return models.UsageLimitReached{ResetsAt: nil, Message: *assistant.Error}, true
	}
	return nil, false
}

// decodeMessage translates a finished message. User messages count their images; assistant messages keep
// their text, reasoning and error. Messages with nothing to show produce no event.
func decodeMessage(message any) (models.AgentEvent, bool) {
	role, ok := strField(message, "role")
	if !ok {
		return nil, false
	}
	content := field(message, "content")
	switch role {
	case "user":
		text := contentText(content)
		images := countBlocks(content, "image")
		if text == "" && images == 0 {
			return nil, false
		}
		return models.UserMessage{Text: text, Images: images}, true
	case "assistant":
		text := blocksText(content, "text", "text")
		thinking := blocksText(content, "thinking", "thinking")
		var failure *string
		if stop := stringOf(message, "stopReason"); stop == "error" || stop == "aborted" {
			reason, hasReason := strField(message, "errorMessage")
			if !hasReason {
				reason = interruptedMessage
			}
			failure = &reason
		}
		if text == "" && thinking == "" && failure == nil {
			return nil, false
		}
		var thinkingText *string
		if thinking != "" {
			thinkingText = &thinking
		}
		return models.AssistantMessage{Text: text, Thinking: thinkingText, Error: failure}, true
	default:
		return nil, false
	}
}

// decodeUpdate translates the streamed pieces of an assistant message.
func decodeUpdate(record any) (models.AgentEvent, bool) {
	update := field(record, "assistantMessageEvent")
	delta := stringOf(update, "delta")
	kind, ok := strField(update, "type")
	if !ok {
		return nil, false
	}
	switch kind {
	case "text_delta":
		return models.TextDelta{Delta: delta}, true
	case "thinking_delta":
		return models.ThinkingDelta{Delta: delta}, true
	default:
		return nil, false
	}
}

// decodeUIRequest translates a request of the agent to the user: a dialog, a notice or a status line.
func decodeUIRequest(record any) (models.AgentEvent, bool) {
	method, ok := strField(record, "method")
	if !ok {
		return nil, false
	}
	id := stringOf(record, "id")
	title := stripANSI(stringOf(record, "title"))
	var timeoutMS *uint64
	if timeout, hasTimeout := uintField(record, "timeout"); hasTimeout {
		timeoutMS = &timeout
	}
	dialog := func(kind models.DialogKind) (models.AgentEvent, bool) {
		return models.DialogRequested{Request: models.DialogRequest{
			ID:        id,
			Title:     title,
			Kind:      kind,
			TimeoutMS: timeoutMS,
		}}, true
	}
	switch method {
	case "select":
		return dialog(models.SelectDialog{Options: stringList(field(record, "options"))})
	case "confirm":
		return dialog(models.ConfirmDialog{Message: optString(record, "message")})
	case "input":
		return dialog(models.InputDialog{Placeholder: optString(record, "placeholder")})
	case "editor":
		return dialog(models.EditorDialog{Prefill: optString(record, "prefill")})
	case "notify":
		return notice(notifyLevel(stringOf(record, "notifyType")), stripANSI(stringOf(record, "message"))), true
	case "setStatus":
		return models.StatusChanged{Key: stringOf(record, "statusKey"), Text: optStripped(record, "statusText")}, true
	case "setWidget":
		return models.WidgetChanged{Key: stringOf(record, "widgetKey"), Lines: widgetLines(record)}, true
	default:
		return nil, false
	}
}

// notifyLevel maps the notifyType of a notify request; anything but warning or error is informational.
func notifyLevel(raw string) models.NoticeLevel {
	switch raw {
	case "warning":
		return models.NoticeWarning
	case "error":
		return models.NoticeError
	default:
		return models.NoticeInfo
	}
}

// widgetLines returns the text lines of a widget, without escape sequences. It is nil when the request
// carries no lines, which clears the widget.
func widgetLines(record any) *[]string {
	items, ok := arrayField(record, "widgetLines")
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		if line, isString := item.(string); isString {
			lines = append(lines, stripANSI(line))
		}
	}
	return &lines
}

// activityTracker remembers the subagent counters and the background jobs of one agent. Jobs are work the
// agent does without the user, so they count as active, next to the subagents.
type activityTracker struct {
	subagents uint32
	waiting   uint32
	jobs      uint32
}

// fold remembers the counters carried by the events and, after each change, emits one SubagentsChanged
// that adds the background jobs to the active subagents.
func (t *activityTracker) fold(events []models.AgentEvent) []models.AgentEvent {
	out := make([]models.AgentEvent, 0, len(events))
	for _, event := range events {
		switch typed := event.(type) {
		case models.SubagentsChanged:
			t.subagents = typed.Active
			t.waiting = typed.Waiting
			out = append(out, t.combined())
		case models.StatusChanged:
			if typed.Key != jobsStatusKey {
				out = append(out, event)
				continue
			}
			t.jobs = 0
			if typed.Text != nil {
				t.jobs = jobsInStatus(*typed.Text)
			}
			out = append(out, event, t.combined())
		default:
			out = append(out, event)
		}
	}
	return out
}

func (t *activityTracker) combined() models.SubagentsChanged {
	return models.SubagentsChanged{Active: saturatingAdd(t.subagents, t.jobs), Waiting: t.waiting}
}

// jobsInStatus returns the first integer of a jobs status text ("⧗ 2 jobs" gives 2), or 0 when there is none.
func jobsInStatus(text string) uint32 {
	for _, word := range strings.Fields(text) {
		// Rust's integer parsing accepts a leading '+', so the same spellings are counted here.
		if count, err := strconv.ParseUint(strings.TrimPrefix(word, "+"), 10, 32); err == nil {
			return uint32(count)
		}
	}
	return 0
}

// decodeSubagents returns the active and waiting counts of the gentle-agents widget. The boolean is false
// when the record is not that widget, or when its payload cannot be read (the counts are then kept).
func decodeSubagents(record any) (models.AgentEvent, bool) {
	first, isWidget := subagentsWidget(record)
	if !isWidget {
		return nil, false
	}
	if first == nil {
		return models.SubagentsChanged{Active: 0, Waiting: 0}, true
	}
	activity, ok := parseJSON(*first)
	if !ok {
		return nil, false
	}
	summary, present := lookup(activity, "summary")
	if !present {
		return nil, false
	}
	return models.SubagentsChanged{
		Active:  saturatingAdd(counterField(summary, "running"), counterField(summary, "queued")),
		Waiting: counterField(summary, "waiting"),
	}, true
}

// decodeSubagentTasks returns the full snapshot of the subagents carried by the gentle-agents widget. The
// boolean is false when the line is not that widget or its payload is malformed; the caller then keeps the
// snapshot it had. Clearing the widget is an empty snapshot.
func decodeSubagentTasks(line string) ([]models.SubagentTask, bool) {
	record, ok := parseJSON(line)
	if !ok {
		return nil, false
	}
	first, isWidget := subagentsWidget(record)
	if !isWidget {
		return nil, false
	}
	if first == nil {
		return []models.SubagentTask{}, true
	}
	payload, ok := parseJSON(*first)
	if !ok {
		return nil, false
	}
	if schema, hasSchema := strField(payload, "schema"); hasSchema && !strings.HasPrefix(schema, subagentsSchemaPrefix) {
		return nil, false
	}
	items, ok := arrayField(payload, "tasks")
	if !ok {
		return nil, false
	}
	tasks := make([]models.SubagentTask, 0, len(items))
	for _, item := range items {
		if task, decoded := decodeSubagentTask(item); decoded {
			tasks = append(tasks, task)
		}
	}
	return tasks, true
}

// subagentsWidget returns the first line of the gentle-agents widget. isWidget is false when the record is
// another record; first is nil when the widget has no text line, which is what clearing it looks like.
func subagentsWidget(record any) (first *string, isWidget bool) {
	if stringOf(record, "type") != "extension_ui_request" ||
		stringOf(record, "method") != "setWidget" ||
		stringOf(record, "widgetKey") != subagentsWidgetKey {
		return nil, false
	}
	items, ok := arrayField(record, "widgetLines")
	if !ok || len(items) == 0 {
		return nil, true
	}
	line, isString := items[0].(string)
	if !isString {
		return nil, true
	}
	return &line, true
}

func decodeSubagentTask(task any) (models.SubagentTask, bool) {
	summary, present := lookup(task, "summary")
	if !present {
		return models.SubagentTask{}, false
	}
	id, ok := strField(summary, "id")
	if !ok {
		return models.SubagentTask{}, false
	}
	return models.SubagentTask{
		ID:             id,
		Agent:          stringOf(summary, "agent"),
		Label:          optString(summary, "label"),
		Prompt:         optString(summary, "prompt"),
		Status:         decodeSubagentStatus(stringOf(summary, "status")),
		CreatedAt:      intPtr(summary, "createdAt"),
		StartedAt:      intPtr(summary, "startedAt"),
		EndedAt:        intPtr(summary, "endedAt"),
		LastStep:       optString(summary, "lastStep"),
		LastActivityAt: intPtr(summary, "lastActivityAt"),
		Turns:          counterField(summary, "turns"),
		ToolCalls:      counterField(summary, "toolCalls"),
		Error:          optString(summary, "error"),
		Thread:         decodeThread(task),
	}, true
}

func decodeSubagentStatus(raw string) models.SubagentStatus {
	switch raw {
	case "queued":
		return models.SubagentQueued
	case "running":
		return models.SubagentRunning
	case "waiting":
		return models.SubagentWaiting
	case "completed":
		return models.SubagentCompleted
	case "failed":
		return models.SubagentFailed
	case "cancelled":
		return models.SubagentCancelled
	case "timed_out":
		return models.SubagentTimedOut
	default:
		return models.SubagentUnknown
	}
}

// decodeThread returns the transcript items of a subagent; items that cannot be read are skipped.
func decodeThread(task any) []models.SubagentThreadItem {
	items, ok := arrayField(field(task, "thread"), "items")
	if !ok {
		return nil
	}
	thread := make([]models.SubagentThreadItem, 0, len(items))
	for _, item := range items {
		if decoded, found := decodeThreadItem(item); found {
			thread = append(thread, decoded)
		}
	}
	return thread
}

// decodeThreadItem reads one transcript item. An unknown kind with text degrades to a note; without text
// it is dropped.
func decodeThreadItem(item any) (models.SubagentThreadItem, bool) {
	kind, ok := strField(item, "kind")
	if !ok {
		return nil, false
	}
	text, hasText := strField(item, "text")
	switch kind {
	case "text":
		if !hasText {
			return nil, false
		}
		return models.SubagentText{Text: text}, true
	case "thinking":
		if !hasText {
			return nil, false
		}
		return models.SubagentThinking{Text: text}, true
	case "tool":
		isError, _ := boolField(item, "isError")
		running, _ := boolField(item, "running")
		return models.SubagentTool{
			Name:    stringOf(item, "name"),
			Args:    optString(item, "args"),
			Running: running,
			IsError: isError,
			Output:  optString(item, "output"),
		}, true
	default:
		if !hasText {
			return nil, false
		}
		return models.SubagentNote{Text: text}, true
	}
}

// parseJSON decodes one line into a JSON value. Numbers keep their spelling (json.Number), so tool
// arguments are shown as the agent wrote them.
func parseJSON(line string) (any, bool) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return value, true
}

// compactJSON encodes value as compact JSON without HTML escaping, as serde_json does.
func compactJSON(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", fmt.Errorf("encode pi record: %w", err)
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

// argsText is the compact JSON of the tool arguments; empty when the record has none.
func argsText(record any) string {
	args, present := lookup(record, "args")
	if !present {
		return ""
	}
	// Values come from a decoded line, so they always encode; the fallback only keeps the stream going.
	text, err := compactJSON(args)
	if err != nil {
		return ""
	}
	return text
}

// lookup returns a member of a JSON object and whether it is present. Non-objects have no members.
func lookup(value any, key string) (any, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	member, present := object[key]
	return member, present
}

// field returns a member of a JSON object, or nil when value is not an object or lacks the key.
func field(value any, key string) any {
	member, _ := lookup(value, key)
	return member
}

func strField(value any, key string) (string, bool) {
	text, ok := field(value, key).(string)
	return text, ok
}

func stringOf(value any, key string) string {
	text, _ := strField(value, key)
	return text
}

func optString(value any, key string) *string {
	text, ok := strField(value, key)
	if !ok {
		return nil
	}
	return &text
}

// optStripped is optString with the escape sequences removed from the text.
func optStripped(value any, key string) *string {
	text, ok := strField(value, key)
	if !ok {
		return nil
	}
	stripped := stripANSI(text)
	return &stripped
}

func boolField(value any, key string) (bool, bool) {
	flag, ok := field(value, key).(bool)
	return flag, ok
}

func arrayField(value any, key string) ([]any, bool) {
	items, ok := field(value, key).([]any)
	return items, ok
}

// uintField reads a non-negative integer. Numbers written with a fraction or an exponent are not integers.
func uintField(value any, key string) (uint64, bool) {
	number, ok := field(value, key).(json.Number)
	if !ok {
		return 0, false
	}
	count, err := strconv.ParseUint(number.String(), 10, 64)
	return count, err == nil
}

// intField reads a signed integer, with the same rules as uintField.
func intField(value any, key string) (int64, bool) {
	number, ok := field(value, key).(json.Number)
	if !ok {
		return 0, false
	}
	count, err := strconv.ParseInt(number.String(), 10, 64)
	return count, err == nil
}

func intPtr(value any, key string) *int64 {
	count, ok := intField(value, key)
	if !ok {
		return nil
	}
	return &count
}

// counterField reads a counter as a uint32; missing, negative or fractional values count as zero, and
// values above the range saturate.
func counterField(value any, key string) uint32 {
	count, ok := uintField(value, key)
	if !ok {
		return 0
	}
	return clampUint32(count)
}

func clampUint32(count uint64) uint32 {
	if count > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(count)
}

func saturatingAdd(left uint32, right uint32) uint32 {
	if left > math.MaxUint32-right {
		return math.MaxUint32
	}
	return left + right
}

// stringList reads a list of texts: each item is a string, or an object with a text or message field.
func stringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	var texts []string
	for _, item := range items {
		if text, isString := item.(string); isString {
			texts = append(texts, text)
			continue
		}
		if text, hasText := strField(item, "text"); hasText {
			texts = append(texts, text)
			continue
		}
		if text, hasMessage := strField(item, "message"); hasMessage {
			texts = append(texts, text)
		}
	}
	return texts
}

// contentText returns the text of a content that is either a string or a list of blocks.
func contentText(content any) string {
	if text, ok := content.(string); ok {
		return text
	}
	return blocksText(content, "text", "text")
}

// countBlocks counts the blocks of one type in a list of content blocks.
func countBlocks(content any, blockType string) uint32 {
	items, ok := content.([]any)
	if !ok {
		return 0
	}
	var count uint32
	for _, item := range items {
		if stringOf(item, "type") == blockType {
			count++
		}
	}
	return count
}

// blocksText joins, with newlines, the field of every block of one type in a list of content blocks.
func blocksText(content any, blockType string, fieldName string) string {
	items, ok := content.([]any)
	if !ok {
		return ""
	}
	var texts []string
	for _, item := range items {
		if stringOf(item, "type") != blockType {
			continue
		}
		if text, hasText := strField(item, fieldName); hasText {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func notice(level models.NoticeLevel, message string) models.Notice {
	return models.Notice{Level: level, Message: message}
}

// truncate keeps the first maxToolTextChars characters of a text and marks the cut.
func truncate(text string) string {
	if utf8.RuneCountInString(text) <= maxToolTextChars {
		return text
	}
	return string([]rune(text)[:maxToolTextChars]) + truncatedSuffix
}

// stripANSI removes the escape sequences of terminal colors. An ESC followed by '[' drops everything up to
// and including the next ASCII letter; any other ESC is dropped alone.
func stripANSI(text string) string {
	runes := []rune(text)
	var output strings.Builder
	output.Grow(len(text))
	for index := 0; index < len(runes); index++ {
		if runes[index] != '\u001b' {
			output.WriteRune(runes[index])
			continue
		}
		if index+1 < len(runes) && runes[index+1] == '[' {
			index++
			for index+1 < len(runes) {
				index++
				if isASCIILetter(runes[index]) {
					break
				}
			}
		}
	}
	return output.String()
}

func isASCIILetter(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}
