//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical across runtimes.
package agy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/runtime"
)

// Limits on tool output and on the summary of a tool input shown in a permission dialog.
const (
	maxToolTextChars = 20_000
	maxSummaryChars  = 500
)

// Protocol constants of ACP and of the messages the user reads.
const (
	protocolVersion        = 1
	interruptedMessage     = "ejecución interrumpida"
	methodNotFound         = -32601
	clientName             = "iworkflow"
	defaultPermissionTitle = "Permiso de herramienta"
	clientVersion          = "0.17.10"
	autoAllowedKind        = "edit"
	allowOnce              = "allow_once"
	rejectOnce             = "reject_once"
	defaultAllowOption     = "allow-once"
	defaultRejectOption    = "reject-once"
	modeStatusKey          = "mode"
)

// acpSessionConfig is what the session is opened with.
type acpSessionConfig struct {
	// cwd is the working folder of the session.
	cwd string
	// resumeSession is the session to reopen with session/load; nil opens a new one.
	resumeSession *string
	// model is the modelId to apply with session/set_model.
	model *string
}

// acpPhase is the stage of the handshake and of the session.
type acpPhase uint8

const (
	phaseStarting acpPhase = iota
	phaseReady
	phaseFailed
)

// acpRequestKind says which of our own requests is waiting for its reply.
type acpRequestKind uint8

const (
	rpcInitialize acpRequestKind = iota
	rpcNewSession
	rpcLoadSession
	rpcSetModel
	rpcSetMode
	rpcPrompt
)

// rpcRequest is one of our requests waiting for its reply. mode is set for rpcSetMode.
type rpcRequest struct {
	kind acpRequestKind
	mode models.AgentMode
}

// toolState is what is known of a tool call. args is its input, once the agent sends it.
type toolState struct {
	kind    string
	title   string
	args    any
	started bool
}

// pendingDialog is a request of the agent waiting for the user's answer.
type pendingDialog interface {
	requestID() any
}

// pendingPermission is a session/request_permission for a tool that asks for confirmation.
type pendingPermission struct {
	rpcID        any
	allowOption  string
	rejectOption string
}

func (p pendingPermission) requestID() any { return p.rpcID }

// dialogStore keeps track of open dialogs.
type dialogStore struct {
	entries map[string]pendingDialog
}

func newDialogStore() *dialogStore {
	return &dialogStore{entries: make(map[string]pendingDialog)}
}

func (s *dialogStore) put(id string, d pendingDialog) {
	s.entries[id] = d
}

func (s *dialogStore) take(id string) (pendingDialog, bool) {
	d, ok := s.entries[id]
	if ok {
		delete(s.entries, id)
	}
	return d, ok
}

type openDialog struct {
	key    string
	dialog pendingDialog
}

func (s *dialogStore) drain() []openDialog {
	out := make([]openDialog, 0, len(s.entries))
	for k, v := range s.entries {
		out = append(out, openDialog{key: k, dialog: v})
	}
	clear(s.entries)
	return out
}

type acpProtocol struct {
	config    acpSessionConfig
	phase     acpPhase
	sessionID *string
	nextID    uint64
	requests  map[uint64]rpcRequest

	outgoing       []string
	queued         [][]any
	promptInFlight bool
	running        bool
	text           string
	thinking       string
	tools          map[string]*toolState
	dialogs        *dialogStore
	abandoned      []string
	onSession      func(sessionID string)
}

var _ runtime.LineCodec = (*acpProtocol)(nil)

func newACPProtocol(config acpSessionConfig) *acpProtocol {
	return &acpProtocol{
		config:    config,
		nextID:    1,
		requests:  make(map[uint64]rpcRequest),
		tools:     make(map[string]*toolState),
		dialogs:   newDialogStore(),
		onSession: func(string) {},
	}
}

func (p *acpProtocol) withSessionListener(listener func(sessionID string)) *acpProtocol {
	p.onSession = listener
	return p
}

// Start begins the handshake: client sends initialize.
func (p *acpProtocol) Start() []string {
	p.sendRequest(rpcRequest{kind: rpcInitialize}, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": clientName, "version": clientVersion},
	})
	return p.TakeOutgoing()
}

func (p *acpProtocol) TakeOutgoing() []string {
	out := p.outgoing
	p.outgoing = nil
	return out
}

func (p *acpProtocol) sendRequest(req rpcRequest, method string, params map[string]any) {
	id := p.nextID
	p.nextID++
	p.requests[id] = req
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}))
}

func (p *acpProtocol) reply(id any, result any) {
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}))
}

func (p *acpProtocol) replyError(id any, code int, message string) {
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	}))
}

func (p *acpProtocol) openSession() {
	if p.config.resumeSession == nil {
		p.openNewSession()
		return
	}
	p.sendRequest(rpcRequest{kind: rpcLoadSession}, "session/load", map[string]any{
		"sessionId":  *p.config.resumeSession,
		"cwd":        p.config.cwd,
		"mcpServers": []any{},
	})
}

func (p *acpProtocol) openNewSession() {
	p.sendRequest(rpcRequest{kind: rpcNewSession}, "session/new", map[string]any{
		"cwd":        p.config.cwd,
		"mcpServers": []any{},
	})
}

func (p *acpProtocol) sessionOpened(sessionID string) {
	if p.onSession != nil {
		p.onSession(sessionID)
	}
	p.sessionID = &sessionID
	if p.config.model == nil {
		p.becomeReady()
		return
	}
	p.sendRequest(rpcRequest{kind: rpcSetModel}, "session/set_model", map[string]any{
		"sessionId": sessionID,
		"modelId":   *p.config.model,
	})
}

func (p *acpProtocol) becomeReady() {
	p.phase = phaseReady
	p.pump()
}

func (p *acpProtocol) fail(message string) []models.AgentEvent {
	p.phase = phaseFailed
	hadWork := p.promptInFlight || len(p.queued) > 0
	p.queued = nil
	p.promptInFlight = false
	events := []models.AgentEvent{errorEvent(message)}
	if hadWork || p.running {
		p.running = false
		events = append(events, models.RunSettled{})
	}
	return events
}

// Encode converts an AgentCommand into ACP line(s) or holds prompts until ready.
func (p *acpProtocol) Encode(command models.AgentCommand, _ string) (string, bool, error) {
	switch cmd := command.(type) {
	case models.PromptCommand:
		if p.phase == phaseFailed {
			return "", false, werrors.Runtime("el agente de agy no está disponible")
		}
		p.queued = append(p.queued, promptContent(cmd.Message, cmd.Images))
		p.pump()
		return "", false, nil

	case models.SetModeCommand:
		if p.phase != phaseReady || p.sessionID == nil {
			return "", false, werrors.Runtime("el agente de agy aún no tiene sesión abierta")
		}
		p.sendRequest(rpcRequest{kind: rpcSetMode, mode: cmd.Mode}, "session/set_mode", map[string]any{
			"sessionId": *p.sessionID,
			"modeId":    string(cmd.Mode),
		})
		return "", false, nil

	case models.AbortCommand:
		if p.sessionID != nil {
			p.sendRequest(rpcRequest{}, "session/cancel", map[string]any{
				"sessionId": *p.sessionID,
			})
		}
		return "", false, nil

	case models.AnswerDialogCommand:
		return p.encodeAnswer(cmd)

	default:
		return "", false, werrors.Unsupported(fmt.Sprintf("comando de agente no soportado: %v", command))
	}
}

func promptContent(message string, images []models.ImageAttachment) []any {
	blocks := make([]any, 0, 1+len(images))
	if message != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": message})
	}
	for _, img := range images {
		blocks = append(blocks, map[string]any{
			"type":     "image",
			"data":     img.Data,
			"mimeType": img.MimeType,
		})
	}
	return blocks
}

func (p *acpProtocol) encodeAnswer(cmd models.AnswerDialogCommand) (string, bool, error) {
	dialog, ok := p.dialogs.take(cmd.DialogID)
	if !ok {
		return "", false, nil
	}
	perm, isPerm := dialog.(pendingPermission)
	if !isPerm {
		return "", false, nil
	}
	var option string
	switch ans := cmd.Answer.(type) {
	case models.ConfirmedAnswer:
		if ans.Confirmed {
			option = perm.allowOption
		} else {
			option = perm.rejectOption
		}
	default:
		option = perm.rejectOption
	}
	p.reply(perm.rpcID, selected(option))
	return "", false, nil
}

func (p *acpProtocol) pump() {
	if p.phase != phaseReady || p.promptInFlight || p.sessionID == nil || len(p.queued) == 0 {
		return
	}
	next := p.queued[0]
	p.queued = p.queued[1:]
	p.promptInFlight = true
	p.sendRequest(rpcRequest{kind: rpcPrompt}, "session/prompt", map[string]any{
		"sessionId": *p.sessionID,
		"prompt":    next,
	})
}

// Decode processes incoming stdout lines.
func (p *acpProtocol) Decode(_ models.TaskID, line string) []models.AgentEvent {
	var record map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &record); err != nil {
		return nil
	}
	method, hasMethod := strField(record, "method")
	idVal, hasID := record["id"]

	switch {
	case hasMethod && hasID:
		return p.decodeRequest(method, idVal, record)
	case hasMethod:
		return p.decodeNotification(method, record)
	case hasID:
		return p.decodeReply(idVal, record)
	default:
		return nil
	}
}

func (p *acpProtocol) decodeReply(id any, record map[string]any) []models.AgentEvent {
	key, ok := parseID(id)
	if !ok {
		return nil
	}
	req, known := p.requests[key]
	if !known {
		return nil
	}
	delete(p.requests, key)

	var errText *string
	if rawErr, failed := lookup(record, "error"); failed {
		msg := errorMessage(rawErr)
		errText = &msg
	}
	result := field(record, "result")

	switch req.kind {
	case rpcInitialize:
		if errText != nil {
			return p.fail("no se pudo iniciar ACP con agy: " + *errText)
		}
		p.openSession()
		return nil

	case rpcNewSession:
		sessID, hasSess := strField(result, "sessionId")
		switch {
		case errText == nil && hasSess:
			p.sessionOpened(sessID)
			return modeStatus(result)
		case errText != nil:
			return p.fail("no se pudo abrir la sesión de agy: " + *errText)
		default:
			return p.fail("la sesión de agy no devolvió sessionId")
		}

	case rpcLoadSession:
		if errText == nil && p.config.resumeSession != nil {
			p.sessionOpened(*p.config.resumeSession)
			return modeStatus(result)
		}
		p.openNewSession()
		reason := "sin sesión guardada"
		if errText != nil {
			reason = *errText
		}
		return []models.AgentEvent{notice(models.NoticeWarning,
			fmt.Sprintf("no se pudo reanudar la sesión anterior (%s); se abre una nueva", reason))}

	case rpcSetModel:
		p.becomeReady()
		if errText != nil {
			return []models.AgentEvent{notice(models.NoticeWarning,
				fmt.Sprintf("no se pudo fijar el modelo %s (%s)", *p.config.model, *errText))}
		}
		return nil

	case rpcSetMode:
		if errText != nil {
			return []models.AgentEvent{notice(models.NoticeWarning,
				fmt.Sprintf("no se pudo fijar el modo %s (%s)", req.mode, *errText))}
		}
		return []models.AgentEvent{modeEvent(string(req.mode))}

	case rpcPrompt:
		return p.settle(errText, result)

	default:
		return nil
	}
}

func (p *acpProtocol) decodeNotification(method string, record map[string]any) []models.AgentEvent {
	if method != "session/update" || p.phase != phaseReady {
		return nil
	}
	params := field(record, "params")
	update, present := lookup(params, "update")
	if !present {
		return nil
	}
	switch stringOf(update, "sessionUpdate") {
	case "agent_message_chunk":
		delta := chunkText(update)
		events := p.startRun(nil)
		p.text += delta
		return append(events, models.TextDelta{Delta: delta})
	case "agent_thought_chunk":
		delta := chunkText(update)
		events := p.startRun(nil)
		p.thinking += delta
		return append(events, models.ThinkingDelta{Delta: delta})
	case "current_mode_update":
		if mode, ok := strField(update, "currentModeId"); ok {
			return []models.AgentEvent{modeEvent(mode)}
		}
		return nil
	case "tool_call":
		return p.toolCall(update)
	case "tool_call_update":
		return p.toolCallUpdate(update)
	default:
		return nil
	}
}

func (p *acpProtocol) toolCall(update any) []models.AgentEvent {
	callID := stringOf(update, "toolCallId")
	p.tools[callID] = &toolState{}
	return p.toolCallUpdateFor(callID, update)
}

func (p *acpProtocol) toolCallUpdate(update any) []models.AgentEvent {
	return p.toolCallUpdateFor(stringOf(update, "toolCallId"), update)
}

func (p *acpProtocol) toolCallUpdateFor(callID string, update any) []models.AgentEvent {
	events := p.startRun(nil)
	state, known := p.tools[callID]
	if !known {
		state = &toolState{}
		p.tools[callID] = state
	}
	if kind, ok := strField(update, "kind"); ok {
		state.kind = kind
	}
	if title, ok := strField(update, "title"); ok {
		state.title = title
	}
	if input := field(update, "rawInput"); !isEmptyValue(input) {
		state.args = input
	}
	status := stringOf(update, "status")
	finished := status == "completed" || status == "failed"
	shouldStart := !state.started && (!isEmptyValue(state.args) || finished || status == "in_progress")
	if shouldStart {
		state.started = true
		started := models.ToolStarted{CallID: callID, ToolName: toolName(state), Args: truncate(valueText(state.args))}
		events = p.flushMessage(events)
		events = append(events, started)
	}
	if finished {
		delete(p.tools, callID)
		events = append(events, models.ToolFinished{
			CallID:   callID,
			ToolName: toolName(state),
			Output:   truncate(toolOutput(update)),
			IsError:  status == "failed" || exitCodeFailed(update),
		})
	}
	return events
}

func (p *acpProtocol) decodeRequest(method string, id any, record map[string]any) []models.AgentEvent {
	params := field(record, "params")
	switch method {
	case "session/request_permission":
		return p.requestPermission(id, params)
	default:
		p.replyError(id, methodNotFound, "método no soportado por el cliente")
		return nil
	}
}

func (p *acpProtocol) requestPermission(id any, params any) []models.AgentEvent {
	tool := field(params, "toolCall")
	options, _ := field(params, "options").([]any)
	allowOption := optionIDOfKind(options, allowOnce, defaultAllowOption)
	rejectOption := optionIDOfKind(options, rejectOnce, defaultRejectOption)
	if stringOf(tool, "kind") == autoAllowedKind {
		p.reply(id, selected(allowOption))
		return nil
	}
	dialogID := "perm-" + idKey(id)
	p.dialogs.put(dialogID, pendingPermission{rpcID: id, allowOption: allowOption, rejectOption: rejectOption})
	title := stringOf(tool, "title")
	if title == "" {
		title = defaultPermissionTitle
	}
	return []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    dialogID,
		Title: title,
		Kind:  models.ConfirmDialog{Message: permissionSummary(tool)},
	}}}
}

func (p *acpProtocol) settle(errText *string, result any) []models.AgentEvent {
	var events []models.AgentEvent
	events = p.startRun(events)
	events = p.flushMessage(events)
	reason, hasReason := strField(result, "stopReason")
	switch {
	case errText != nil:
		events = append(events, errorEvent(*errText))
	case hasReason && reason == "cancelled":
		events = append(events, notice(models.NoticeWarning, interruptedMessage))
	case hasReason && reason != "end_turn":
		events = append(events, notice(models.NoticeWarning, fmt.Sprintf("el agente terminó con motivo «%s»", reason)))
	}
	events = append(events, models.RunSettled{})
	p.running = false
	p.promptInFlight = false
	clear(p.tools)
	stale := p.abandoned
	p.abandoned = nil
	for _, open := range p.dialogs.drain() {
		stale = append(stale, open.key)
	}
	for _, dialogID := range stale {
		events = append(events, models.DialogResolved{DialogID: dialogID})
	}
	p.pump()
	return events
}

func (p *acpProtocol) startRun(events []models.AgentEvent) []models.AgentEvent {
	if !p.running {
		p.running = true
		events = append(events, models.RunStarted{})
	}
	return events
}

func (p *acpProtocol) flushMessage(events []models.AgentEvent) []models.AgentEvent {
	text, thinking := p.text, p.thinking
	p.text, p.thinking = "", ""
	if text == "" && thinking == "" {
		return events
	}
	message := models.AssistantMessage{Text: text}
	if thinking != "" {
		message.Thinking = &thinking
	}
	return append(events, message)
}

// Helpers

func parseID(v any) (uint64, bool) {
	switch typed := v.(type) {
	case float64:
		return uint64(typed), true
	case int:
		return uint64(typed), true
	case int64:
		return uint64(typed), true
	case uint64:
		return typed, true
	case string:
		n, err := strconv.ParseUint(typed, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func idKey(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return fmt.Sprint(v)
	}
}

func lookup(record any, key string) (any, bool) {
	m, ok := record.(map[string]any)
	if !ok {
		return nil, false
	}
	v, present := m[key]
	return v, present
}

func field(record any, key string) any {
	v, _ := lookup(record, key)
	return v
}

func strField(record any, key string) (string, bool) {
	v, ok := lookup(record, key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func stringOf(record any, key string) string {
	s, _ := strField(record, key)
	return s
}

func chunkText(update any) string {
	content := field(update, "content")
	if txt, ok := strField(content, "text"); ok {
		return txt
	}
	return ""
}

func modeEvent(mode string) models.AgentEvent {
	return models.StatusChanged{Key: modeStatusKey, Text: &mode}
}

func modeStatus(result any) []models.AgentEvent {
	modes := field(result, "modes")
	if cur, ok := strField(modes, "currentModeId"); ok {
		return []models.AgentEvent{modeEvent(cur)}
	}
	return nil
}

func selected(optionID string) map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optionID}}
}

func optionIDOfKind(options []any, kind string, fallback string) string {
	index := slices.IndexFunc(options, func(opt any) bool { return stringOf(opt, "kind") == kind })
	if index < 0 {
		return fallback
	}
	if id, ok := strField(options[index], "optionId"); ok && id != "" {
		return id
	}
	if id, ok := strField(options[index], "id"); ok && id != "" {
		return id
	}
	return fallback
}

func permissionSummary(tool any) *string {
	input := field(tool, "rawInput")
	var detail *string
	if command, ok := strField(input, "command"); ok {
		detail = &command
	} else if path, ok := strField(input, "path"); ok {
		detail = &path
	} else if !isEmptyValue(input) {
		text := valueText(input)
		detail = &text
	}
	if detail == nil {
		return nil
	}
	trimmed := truncateSummary(*detail)
	return &trimmed
}

func toolName(state *toolState) string {
	if state.title != "" {
		return state.title
	}
	if state.kind != "" {
		return state.kind
	}
	return "herramienta"
}

func toolOutput(update any) string {
	out := field(update, "output")
	if str, ok := out.(string); ok {
		return str
	}
	return valueText(out)
}

func exitCodeFailed(update any) bool {
	code, ok := lookup(update, "exitCode")
	if !ok {
		return false
	}
	if num, ok := code.(float64); ok {
		return int(num) != 0
	}
	return false
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	if m, ok := v.(map[string]any); ok && len(m) == 0 {
		return true
	}
	return false
}

func valueText(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return encodeLine(v)
}

func encodeLine(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSpace(buf.String())
}

func truncate(text string) string {
	if len(text) <= maxToolTextChars {
		return text
	}
	return text[:maxToolTextChars] + "\n… (truncado)"
}

func truncateSummary(text string) string {
	if len(text) <= maxSummaryChars {
		return text
	}
	return text[:maxSummaryChars] + "…"
}

func errorMessage(rawError any) string {
	if str, ok := rawError.(string); ok {
		return str
	}
	if msg, ok := strField(rawError, "message"); ok {
		return msg
	}
	return fmt.Sprint(rawError)
}

func notice(level models.NoticeLevel, message string) models.AgentEvent {
	return models.Notice{Level: level, Message: message}
}

func errorEvent(message string) models.AgentEvent {
	if models.LooksLikeUsageLimit(message) {
		return models.UsageLimitReached{Message: message}
	}
	return notice(models.NoticeError, message)
}
