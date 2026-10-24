//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

// Limits on tool output and on the summary of a tool input shown in a permission dialog.
const (
	maxToolTextChars = 20_000
	maxSummaryChars  = 500
)

// Messages sent back to claude when the user does not allow a request. They are user-visible Spanish
// text, kept verbatim from the Rust version.
const (
	denyPlanMessage      = "El usuario rechazó el plan; ajústalo y vuelve a proponerlo."
	denyMessage          = "El usuario rechazó el uso de la herramienta."
	denyQuestionsMessage = "El usuario canceló las preguntas."
)

const (
	planTool      = "ExitPlanMode"
	questionsTool = "AskUserQuestion"
	// questionIDSeparator joins the request id and the question index in AskUserQuestion dialog ids.
	questionIDSeparator = "#"
	interruptedMessage  = "ejecución interrumpida"
	// modeStatusKey is the status line that reports the working mode of the agent.
	modeStatusKey = "mode"
)

// pendingRequest is a permission request waiting for the user's answer.
type pendingRequest interface {
	isPendingRequest()
}

// pendingTool is a tool that asks for confirmation. input is returned as is when the tool is allowed.
type pendingTool struct {
	input       any
	denyMessage string
}

// pendingQuestions is an AskUserQuestion request: one dialog per question, answered all together.
type pendingQuestions struct {
	input     map[string]any
	questions []question
	// answers holds the label chosen for each question; nil while the question is unanswered.
	answers []*string
}

func (*pendingTool) isPendingRequest()      {}
func (*pendingQuestions) isPendingRequest() {}

// question is one question of AskUserQuestion. Each option keeps the text shown in the dialog and the
// label claude expects back.
type question struct {
	title   string
	text    string
	options []questionOption
}

type questionOption struct {
	shown string
	label string
}

// streamProtocol translates the stream-json protocol of one claude process. It runs in a single
// direction per ProcessRuntime, so it needs no locking. It remembers the name of each tool (the result
// carries only the id), the input of each pending permission (allow returns it in updatedInput), and
// whether the current turn was rejected for a usage limit.
type streamProtocol struct {
	runtime.NoHandshake
	running bool
	// thinking is the reasoning received as deltas: the final thinking block arrives empty, with only
	// a signature, so the text is attached to the message or tool that follows it.
	thinking  string
	toolNames map[string]string
	pending   map[string]pendingRequest
	// settled holds requests already cancelled whose sibling dialogs may still arrive.
	settled map[string]bool
	// limitRejected is set when the last rate_limit_event rejected the request. The error result that
	// follows is then a usage limit, whatever its text (it varies by plan and language).
	limitRejected bool
	// limitResetsAt is the renewal instant (epoch seconds) reported by that rate_limit_event.
	limitResetsAt *int64
}

var _ runtime.LineCodec = (*streamProtocol)(nil)

func newStreamProtocol() *streamProtocol {
	return &streamProtocol{
		toolNames: make(map[string]string),
		pending:   make(map[string]pendingRequest),
		settled:   make(map[string]bool),
	}
}

// Decode returns the domain events of one stdout line. Lines that are not JSON or that do not matter
// (hooks, usage-limit notes, subagent output) produce no events.
func (p *streamProtocol) Decode(_ models.TaskID, line string) []models.AgentEvent {
	record, ok := parseRecord(line)
	if !ok {
		return nil
	}
	if member(record, "parent_tool_use_id") != nil {
		return nil
	}
	switch stringOf(record, "type") {
	case "stream_event":
		return p.decodeStreamEvent(record)
	case "assistant":
		return p.decodeAssistant(record)
	case "user":
		return p.decodeUser(record)
	case "control_request":
		return p.decodeControlRequest(record)
	case "rate_limit_event":
		p.noteRateLimit(record)
		return nil
	case "result":
		return p.decodeResult(record)
	case "system":
		return decodeSystem(record)
	default:
		return nil
	}
}

// Encode returns the stdin line (without its line ending) for a command. requestID identifies the
// control request when the command needs one. ok is false when the command produces no line yet.
func (p *streamProtocol) Encode(command models.AgentCommand, requestID string) (string, bool, error) {
	switch typed := command.(type) {
	case models.PromptCommand:
		return encodeLine(userMessage(typed.Message, typed.Images))
	case models.SteerCommand:
		return encodeLine(userMessage(typed.Message, typed.Images))
	case models.FollowUpCommand:
		return encodeLine(userMessage(typed.Message, typed.Images))
	case models.AbortCommand:
		return encodeLine(controlRequestRecord(requestID, map[string]any{"subtype": "interrupt"}))
	case models.SetModeCommand:
		mode, err := permissionMode(typed.Mode)
		if err != nil {
			return "", false, err
		}
		return encodeLine(controlRequestRecord(requestID, map[string]any{"subtype": "set_permission_mode", "mode": mode}))
	case models.AnswerDialogCommand:
		return p.answerDialog(typed.DialogID, typed.Answer)
	default:
		return "", false, werrors.Unsupported(fmt.Sprintf("comando de agente no soportado: %v", command))
	}
}

// startRun opens the run with RunStarted, emitted only by the first event of a turn.
func (p *streamProtocol) startRun(events []models.AgentEvent) []models.AgentEvent {
	if p.running {
		return events
	}
	p.running = true
	return append(events, models.RunStarted{})
}

func (p *streamProtocol) decodeStreamEvent(record map[string]any) []models.AgentEvent {
	event := member(record, "event")
	if stringOf(event, "type") != "content_block_delta" {
		return nil
	}
	delta := member(event, "delta")
	var decoded models.AgentEvent
	switch stringOf(delta, "type") {
	case "text_delta":
		decoded = models.TextDelta{Delta: stringOf(delta, "text")}
	case "thinking_delta":
		text := stringOf(delta, "thinking")
		p.thinking += text
		decoded = models.ThinkingDelta{Delta: text}
	default:
		return nil
	}
	return append(p.startRun(nil), decoded)
}

func (p *streamProtocol) decodeAssistant(record map[string]any) []models.AgentEvent {
	blocks := contentBlocks(record)
	text := joinBlocks(blocks, "text", "text")
	hasContent := text != "" || hasBlockOfType(blocks, "tool_use")
	blockThinking := joinBlocks(blocks, "thinking", "thinking")
	// A block with only thinking (empty text) must not consume the streamed reasoning: it belongs to
	// the text or tool that follows.
	var thinking string
	switch {
	case blockThinking != "":
		p.thinking = ""
		thinking = blockThinking
	case hasContent:
		thinking = p.thinking
		p.thinking = ""
	}
	events := p.startRun(nil)
	if text != "" || thinking != "" {
		events = append(events, models.AssistantMessage{Text: text, Thinking: nonEmpty(thinking)})
	}
	for _, block := range blocks {
		if stringOf(block, "type") != "tool_use" {
			continue
		}
		callID := stringOf(block, "id")
		toolName := stringOf(block, "name")
		p.toolNames[callID] = toolName
		events = append(events, models.ToolStarted{CallID: callID, ToolName: toolName, Args: truncate(inputText(block))})
	}
	return events
}

func (p *streamProtocol) decodeUser(record map[string]any) []models.AgentEvent {
	blocks := contentBlocks(record)
	var events []models.AgentEvent
	for _, block := range blocks {
		if stringOf(block, "type") != "tool_result" {
			continue
		}
		callID := stringOf(block, "tool_use_id")
		toolName := p.toolNames[callID]
		delete(p.toolNames, callID)
		isError, _ := member(block, "is_error").(bool)
		events = append(events, models.ToolFinished{
			CallID:   callID,
			ToolName: toolName,
			Output:   truncate(toolResultText(member(block, "content"))),
			IsError:  isError,
		})
	}
	if len(events) > 0 {
		return events
	}
	// Without --replay-user-messages claude does not echo stdin. Only the messages it replays are shown.
	if !boolMember(record, "isReplay") {
		return nil
	}
	text, ok := stringMember(member(record, "message"), "content")
	if !ok {
		text = joinBlocks(blocks, "text", "text")
	}
	images := countBlocks(blocks, "image")
	if text == "" && images == 0 {
		return nil
	}
	return []models.AgentEvent{models.UserMessage{Text: text, Images: images}}
}

func (p *streamProtocol) decodeControlRequest(record map[string]any) []models.AgentEvent {
	request := member(record, "request")
	if stringOf(request, "subtype") != "can_use_tool" {
		return nil
	}
	id := stringOf(record, "request_id")
	toolName := stringOf(request, "tool_name")
	input := inputOf(request)
	if object, isObject := input.(map[string]any); isObject && toolName == questionsTool {
		if questions, ok := parseQuestions(object); ok {
			return p.requestQuestions(id, object, questions)
		}
	}
	displayName := stringOf(request, "display_name")
	if displayName == "" {
		displayName = toolName
	}
	var title string
	var message *string
	var deny string
	if toolName == planTool {
		title = "Aprobar plan"
		if plan, ok := stringMember(input, "plan"); ok {
			message = &plan
		}
		deny = denyPlanMessage
	} else {
		if displayName == "" {
			displayName = "herramienta"
		}
		title = "Permitir " + displayName
		message = permissionMessage(toolName, stringOf(request, "description"), input)
		deny = denyMessage
	}
	p.pending[id] = &pendingTool{input: input, denyMessage: deny}
	return []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    id,
		Title: title,
		Kind:  models.ConfirmDialog{Message: message},
	}}}
}

// requestQuestions opens one select dialog per question, with id `<request>#<index>`.
func (p *streamProtocol) requestQuestions(requestID string, input map[string]any, questions []question) []models.AgentEvent {
	events := make([]models.AgentEvent, 0, len(questions))
	for index, item := range questions {
		shown := make([]string, 0, len(item.options))
		for _, option := range item.options {
			shown = append(shown, option.shown)
		}
		events = append(events, models.DialogRequested{Request: models.DialogRequest{
			ID:    questionDialogID(requestID, index),
			Title: item.title,
			Kind:  models.SelectDialog{Options: shown},
		}})
	}
	p.pending[requestID] = &pendingQuestions{
		input:     input,
		questions: questions,
		answers:   make([]*string, len(questions)),
	}
	return events
}

func (p *streamProtocol) answerDialog(dialogID string, answer models.DialogAnswer) (string, bool, error) {
	requestID, index, hasIndex := splitDialogID(dialogID)
	pending := p.takePending(dialogID, requestID)
	if pending == nil {
		if p.settled[requestID] {
			return "", false, nil
		}
		return "", false, unknownDialog(dialogID)
	}
	switch request := pending.(type) {
	case *pendingQuestions:
		return p.answerQuestion(dialogID, requestID, index, hasIndex, request, answer)
	case *pendingTool:
		return encodeLine(permissionResponse(dialogID, answer, request.input, request.denyMessage))
	default:
		return "", false, unknownDialog(dialogID)
	}
}

// takePending removes and returns the request that answers dialogID. An exact id is a tool or plan
// request. `<request>#<n>` can only refer to an AskUserQuestion request, never to a tool of another one.
func (p *streamProtocol) takePending(dialogID, requestID string) pendingRequest {
	if pending, ok := p.pending[dialogID]; ok {
		delete(p.pending, dialogID)
		return pending
	}
	if pending, ok := p.pending[requestID].(*pendingQuestions); ok {
		delete(p.pending, requestID)
		return pending
	}
	return nil
}

func (p *streamProtocol) answerQuestion(dialogID, requestID string, index int, hasIndex bool, request *pendingQuestions, answer models.DialogAnswer) (string, bool, error) {
	if !hasIndex || index >= len(request.answers) {
		p.pending[requestID] = request
		return "", false, unknownDialog(dialogID)
	}
	value, isValue := answer.(models.ValueAnswer)
	if !isValue {
		// Cancelling one question denies the whole request; its sibling dialogs are ignored from now on.
		p.settled[requestID] = true
		return encodeLine(denyResponse(requestID, denyQuestionsMessage))
	}
	label := labelFor(request.questions[index], value.Value)
	request.answers[index] = &label
	if slices.ContainsFunc(request.answers, func(answered *string) bool { return answered == nil }) {
		p.pending[requestID] = request
		return "", false, nil
	}
	return encodeLine(allowResponse(requestID, withAnswers(request.input, request.questions, request.answers)))
}

// noteRateLimit records whether claude rejected the request for a usage limit, and when it renews.
func (p *streamProtocol) noteRateLimit(record map[string]any) {
	info := member(record, "rate_limit_info")
	status, ok := stringMember(info, "status")
	if !ok {
		return
	}
	p.limitRejected = status != "allowed" && status != "allowed_warning"
	p.limitResetsAt = nil
	if p.limitRejected {
		p.limitResetsAt = integerMember(info, "resetsAt")
	}
}

func (p *streamProtocol) decodeResult(record map[string]any) []models.AgentEvent {
	p.running = false
	p.thinking = ""
	clear(p.toolNames)
	clear(p.settled)
	// Dialogs that outlive the turn have no request left to answer. They are closed in the UI, or they
	// would stay pending forever.
	events := p.resolvePending()
	subtype, hasSubtype := stringMember(record, "subtype")
	failed := boolMember(record, "is_error") || (hasSubtype && subtype != "success")
	limitRejected := p.limitRejected
	resetsAt := p.limitResetsAt
	p.limitRejected = false
	p.limitResetsAt = nil
	if failed {
		events = append(events, failureEvent(record, limitRejected, resetsAt))
	}
	return append(events, models.RunSettled{})
}

// resolvePending empties the pending requests and returns a DialogResolved for each of their dialogs.
// Requests are visited in id order so the event log is deterministic.
func (p *streamProtocol) resolvePending() []models.AgentEvent {
	keys := make([]string, 0, len(p.pending))
	for key := range p.pending {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var events []models.AgentEvent
	for _, key := range keys {
		for _, dialogID := range dialogIDs(key, p.pending[key]) {
			events = append(events, models.DialogResolved{DialogID: dialogID})
		}
	}
	clear(p.pending)
	return events
}

// failureEvent turns a failed result into a notice, a usage-limit event, or the interrupted warning.
func failureEvent(record map[string]any, limitRejected bool, resetsAt *int64) models.AgentEvent {
	message := "error del agente"
	if text, ok := stringMember(record, "result"); ok {
		message = text
	} else if text, ok := stringMember(record, "subtype"); ok {
		message = text
	}
	switch {
	case stringOf(record, "terminal_reason") == "aborted_streaming":
		return models.Notice{Level: models.NoticeWarning, Message: interruptedMessage}
	case limitRejected || models.LooksLikeUsageLimit(message):
		return models.UsageLimitReached{ResetsAt: resetsAt, Message: message}
	default:
		return models.Notice{Level: models.NoticeError, Message: message}
	}
}

// dialogIDs returns the dialog ids the UI knows for a pending request: one per question, or the request id.
func dialogIDs(key string, pending pendingRequest) []string {
	questions, isQuestions := pending.(*pendingQuestions)
	if !isQuestions {
		return []string{key}
	}
	ids := make([]string, 0, len(questions.questions))
	for index := range questions.questions {
		ids = append(ids, questionDialogID(key, index))
	}
	return ids
}

func questionDialogID(requestID string, index int) string {
	return requestID + questionIDSeparator + strconv.Itoa(index)
}

// splitDialogID splits `<request>#<index>` into the request id and the question index. hasIndex is false
// when there is no separator or the suffix is not a non-negative number.
func splitDialogID(dialogID string) (requestID string, index int, hasIndex bool) {
	cut := strings.LastIndex(dialogID, questionIDSeparator)
	if cut < 0 {
		return dialogID, 0, false
	}
	parsed, err := strconv.Atoi(dialogID[cut+1:])
	if err != nil || parsed < 0 {
		return dialogID[:cut], 0, false
	}
	return dialogID[:cut], parsed, true
}

func unknownDialog(dialogID string) error {
	return werrors.Runtime("no hay un permiso pendiente con id " + dialogID)
}

// decodeSystem reports the working mode that system/init and system/status carry.
func decodeSystem(record map[string]any) []models.AgentEvent {
	subtype := stringOf(record, "subtype")
	if subtype != "init" && subtype != "status" {
		return nil
	}
	permission, ok := stringMember(record, "permissionMode")
	if !ok {
		return nil
	}
	label := modeLabel(permission)
	return []models.AgentEvent{models.StatusChanged{Key: modeStatusKey, Text: &label}}
}

// modeLabel maps a claude permission mode to the domain mode name. Other values are shown as they are.
func modeLabel(permission string) string {
	switch permission {
	case "acceptEdits":
		return string(models.AgentModeAgent)
	case "plan":
		return string(models.AgentModePlan)
	default:
		return permission
	}
}

// permissionMode is the --permission-mode value of each domain mode.
func permissionMode(mode models.AgentMode) (string, error) {
	switch mode {
	case models.AgentModeAgent:
		return "acceptEdits", nil
	case models.AgentModePlan:
		return "plan", nil
	default:
		return "", werrors.Unsupported(fmt.Sprintf("modo de agente no soportado: %s", mode))
	}
}

// userMessage is a plain text message, or content blocks when it carries images.
func userMessage(message string, images []models.ImageAttachment) map[string]any {
	var content any = message
	if len(images) > 0 {
		blocks := []any{map[string]any{"type": "text", "text": message}}
		for _, image := range images {
			blocks = append(blocks, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": image.MimeType,
					"data":       image.Data,
				},
			})
		}
		content = blocks
	}
	return map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}}
}

func controlRequestRecord(requestID string, request map[string]any) map[string]any {
	return map[string]any{"type": "control_request", "request_id": requestID, "request": request}
}

// permissionResponse answers a can_use_tool request. Only a confirmation allows the tool.
func permissionResponse(requestID string, answer models.DialogAnswer, input any, denyText string) map[string]any {
	if confirmed, ok := answer.(models.ConfirmedAnswer); ok && confirmed.Confirmed {
		return allowResponse(requestID, input)
	}
	return denyResponse(requestID, denyText)
}

func allowResponse(requestID string, updatedInput any) map[string]any {
	return controlSuccess(requestID, map[string]any{"behavior": "allow", "updatedInput": updatedInput})
}

func denyResponse(requestID, message string) map[string]any {
	return controlSuccess(requestID, map[string]any{"behavior": "deny", "message": message})
}

func controlSuccess(requestID string, decision map[string]any) map[string]any {
	return map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   decision,
		},
	}
}

// parseQuestions reads the questions of AskUserQuestion. It returns false when there are no questions or
// when any question or option is malformed, so the request falls back to a plain confirmation.
func parseQuestions(input map[string]any) ([]question, bool) {
	raws, ok := input["questions"].([]any)
	if !ok || len(raws) == 0 {
		return nil, false
	}
	questions := make([]question, 0, len(raws))
	for _, raw := range raws {
		parsed, ok := parseQuestion(raw)
		if !ok {
			return nil, false
		}
		questions = append(questions, parsed)
	}
	return questions, true
}

func parseQuestion(raw any) (question, bool) {
	text := stringOf(raw, "question")
	if text == "" {
		return question{}, false
	}
	rawOptions, ok := member(raw, "options").([]any)
	if !ok || len(rawOptions) == 0 {
		return question{}, false
	}
	options := make([]questionOption, 0, len(rawOptions))
	for _, rawOption := range rawOptions {
		label := stringOf(rawOption, "label")
		if label == "" {
			return question{}, false
		}
		shown := label
		if description := stringOf(rawOption, "description"); description != "" {
			shown = label + " — " + description
		}
		options = append(options, questionOption{shown: shown, label: label})
	}
	title := text
	if header := stringOf(raw, "header"); header != "" {
		title = header + ": " + text
	}
	return question{title: title, text: text, options: options}, true
}

// labelFor returns the label of the chosen option. Free text that matches no option is passed as is.
func labelFor(item question, value string) string {
	for _, option := range item.options {
		if option.shown == value || option.label == value {
			return option.label
		}
	}
	return value
}

// withAnswers sets the answers field (question text to label) that AskUserQuestion expects.
func withAnswers(input map[string]any, questions []question, answers []*string) map[string]any {
	byQuestion := make(map[string]any, len(questions))
	for index, item := range questions {
		answer := ""
		if answers[index] != nil {
			answer = *answers[index]
		}
		byQuestion[item.text] = answer
	}
	input["answers"] = byQuestion
	return input
}

// permissionMessage is the text of a permission dialog: the part of the input that the tool will act on
// (command, file, URL...) followed by the model's description, so the user decides on the real data.
func permissionMessage(tool, description string, input any) *string {
	summary, hasSummary := "", false
	if key := summaryKey(tool); key != "" {
		summary, hasSummary = stringMember(input, key)
	}
	if !hasSummary {
		if object, isObject := input.(map[string]any); isObject && len(object) > 0 {
			summary, hasSummary = truncateChars(compactText(input), maxSummaryChars), true
		}
	}
	candidates := []string{description}
	if hasSummary {
		candidates = []string{summary, description}
	}
	var unique []string
	for _, part := range candidates {
		if part != "" && !slices.Contains(unique, part) {
			unique = append(unique, part)
		}
	}
	if len(unique) == 0 {
		return nil
	}
	return ptrTo(strings.Join(unique, "\n"))
}

// summaryKey is the input field that names what a tool acts on. Empty for tools without one.
func summaryKey(tool string) string {
	switch tool {
	case "Bash":
		return "command"
	case "Write", "Edit", "MultiEdit", "Read":
		return "file_path"
	case "NotebookEdit":
		return "notebook_path"
	case "WebFetch":
		return "url"
	case "WebSearch":
		return "query"
	default:
		return ""
	}
}

// encodeLine serializes one stdin record. Lines are compact JSON, one per command.
func encodeLine(record map[string]any) (string, bool, error) {
	line, err := marshalCompact(record)
	if err != nil {
		return "", false, err
	}
	return line, true, nil
}

// marshalCompact encodes value as compact JSON without HTML escaping, as serde_json does, and keeps the
// spelling of numbers (json.Number).
func marshalCompact(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", fmt.Errorf("encode claude record: %w", err)
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

// inputText is the compact JSON of a tool_use block's input as the agent wrote it; empty when the block
// has no input.
func inputText(block any) string {
	object, isObject := block.(map[string]any)
	if !isObject {
		return ""
	}
	input, present := object["input"]
	if !present {
		return ""
	}
	return compactText(input)
}

// compactText is the compact JSON of a value, or empty when it cannot be encoded. Values come from a
// decoded line, so they always encode; the fallback only keeps the stream going.
func compactText(value any) string {
	text, err := marshalCompact(value)
	if err != nil {
		return ""
	}
	return text
}

// parseRecord decodes one stdout line into a JSON object. Numbers keep their spelling (json.Number), so
// tool inputs are shown as the agent wrote them.
func parseRecord(line string) (map[string]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return record, true
}

// member returns the member key of a JSON object, or nil when value is not an object or lacks the key.
func member(value any, key string) any {
	object, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	return object[key]
}

func stringMember(value any, key string) (string, bool) {
	text, ok := member(value, key).(string)
	return text, ok
}

func stringOf(value any, key string) string {
	text, _ := stringMember(value, key)
	return text
}

func boolMember(value any, key string) bool {
	flag, _ := member(value, key).(bool)
	return flag
}

// integerMember returns the member only when it is an integer literal, as serde_json's as_i64 does.
func integerMember(value any, key string) *int64 {
	number, ok := member(value, key).(json.Number)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

// inputOf is the tool input of a control request. It defaults to an empty object when absent.
func inputOf(request any) any {
	object, isObject := request.(map[string]any)
	if !isObject {
		return map[string]any{}
	}
	input, present := object["input"]
	if !present {
		return map[string]any{}
	}
	return input
}

// contentBlocks returns message.content when it is a list of blocks, and nil for a string or no content.
func contentBlocks(record map[string]any) []any {
	blocks, _ := member(member(record, "message"), "content").([]any)
	return blocks
}

func hasBlockOfType(blocks []any, kind string) bool {
	for _, block := range blocks {
		if stringOf(block, "type") == kind {
			return true
		}
	}
	return false
}

func countBlocks(blocks []any, kind string) uint32 {
	var count uint32
	for _, block := range blocks {
		if stringOf(block, "type") == kind {
			count++
		}
	}
	return count
}

// joinBlocks concatenates the field of the blocks of the given type, one per line. Empty texts are skipped.
func joinBlocks(blocks []any, kind, key string) string {
	var texts []string
	for _, block := range blocks {
		if stringOf(block, "type") != kind {
			continue
		}
		if text := stringOf(block, key); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

// toolResultText is the text of a tool_result: a string, or a list of text blocks.
func toolResultText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		return joinBlocks(value, "text", "text")
	default:
		return ""
	}
}

func truncate(text string) string {
	return truncateChars(text, maxToolTextChars)
}

// truncateChars keeps at most limit characters (runes), not bytes.
func truncateChars(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}

// nonEmpty returns a pointer to text, or nil when it is empty.
func nonEmpty(text string) *string {
	if text == "" {
		return nil
	}
	return ptrTo(text)
}

func ptrTo[T any](value T) *T {
	return &value
}
