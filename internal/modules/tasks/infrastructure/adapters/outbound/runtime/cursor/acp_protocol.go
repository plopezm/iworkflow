//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package cursor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

// Protocol constants of ACP and of the messages the user reads (kept verbatim from the Rust version).
const (
	protocolVersion        = 1
	authMethod             = "cursor_login"
	interruptedMessage     = "ejecución interrumpida"
	methodNotFound         = -32601
	clientName             = "iworkflow"
	defaultPermissionTitle = "Permiso de herramienta"
	defaultPlanTitle       = "Plan"
	todosWidgetKey         = "todos"
	// modeStatusKey is the status line that reports the working mode of the agent.
	modeStatusKey = "mode"
	// clientVersion is reported to the agent in initialize. Rust reads CARGO_PKG_VERSION at build time; the Go
	// port has no build version yet, so it mirrors Cargo.toml.
	clientVersion = "0.17.10"
	// autoAllowedKind is the kind of tool whose permissions are accepted without asking (acceptEdits policy).
	autoAllowedKind = "edit"
	allowOnce       = "allow_once"
	rejectOnce      = "reject_once"
	// defaultAllowOption and defaultRejectOption are used when the agent offers no option of that kind.
	defaultAllowOption  = "allow-once"
	defaultRejectOption = "reject-once"
	// questionIDSeparator joins the request id and the question index in cursor/ask_question dialog ids.
	questionIDSeparator = "#"
)

// acpSessionConfig is what the session is opened with.
type acpSessionConfig struct {
	// cwd is the working folder of the session.
	cwd string
	// resumeSession is the session to reopen with session/load; nil opens a new one.
	resumeSession *string
	// model is the modelId of Cursor to apply with session/set_model.
	model *string
}

// acpPhase is the stage of the handshake and of the session.
type acpPhase uint8

const (
	// phaseStarting is the handshake in progress: prompts are held.
	phaseStarting acpPhase = iota
	phaseReady
	phaseFailed
)

// acpRequestKind says which of our own requests is waiting for its reply.
type acpRequestKind uint8

const (
	rpcInitialize acpRequestKind = iota
	rpcAuthenticate
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

// todo is one item of the list the agent keeps.
type todo struct {
	id      string
	content string
	status  string
}

// acpOption is one choice of a cursor/ask_question question: the label shown and the id Cursor expects back.
type acpOption struct {
	label string
	id    string
}

// acpQuestion is one question of cursor/ask_question, answered together with the others.
type acpQuestion struct {
	prompt  string
	id      string
	options []acpOption
}

// pendingDialog is a request of the agent waiting for the user's answer.
type pendingDialog interface {
	// requestID is the id of the JSON-RPC request the answer goes back to.
	requestID() any
}

// pendingPermission is a session/request_permission for a tool that asks for confirmation.
type pendingPermission struct {
	rpcID        any
	allowOption  string
	rejectOption string
}

// pendingPlan is a cursor/create_plan waiting for approval.
type pendingPlan struct {
	rpcID any
}

// pendingQuestions is a cursor/ask_question: one dialog per question, answered all together.
type pendingQuestions struct {
	rpcID     any
	questions []acpQuestion
	// answers holds the option id chosen for each question; nil while the question is unanswered.
	answers []*string
}

func (p pendingPermission) requestID() any { return p.rpcID }
func (p pendingPlan) requestID() any       { return p.rpcID }
func (p pendingQuestions) requestID() any  { return p.rpcID }

// keyedDialog is a pending dialog together with its key.
type keyedDialog struct {
	key    string
	dialog pendingDialog
}

// dialogBook holds the pending dialogs by key and remembers the order they were opened in. Events and
// replies that close them are emitted in that order, so the output does not depend on map iteration.
type dialogBook struct {
	byKey map[string]pendingDialog
	order []string
}

func newDialogBook() dialogBook {
	return dialogBook{byKey: make(map[string]pendingDialog)}
}

// put stores a dialog. A key that is already present keeps its position.
func (b *dialogBook) put(key string, dialog pendingDialog) {
	if _, exists := b.byKey[key]; !exists {
		b.order = append(b.order, key)
	}
	b.byKey[key] = dialog
}

func (b *dialogBook) get(key string) (pendingDialog, bool) {
	dialog, ok := b.byKey[key]
	return dialog, ok
}

func (b *dialogBook) remove(key string) {
	delete(b.byKey, key)
	b.order = slices.DeleteFunc(b.order, func(candidate string) bool { return candidate == key })
}

// drain empties the book and returns its dialogs in the order they were opened.
func (b *dialogBook) drain() []keyedDialog {
	drained := make([]keyedDialog, 0, len(b.order))
	for _, key := range b.order {
		drained = append(drained, keyedDialog{key: key, dialog: b.byKey[key]})
	}
	b.byKey = make(map[string]pendingDialog)
	b.order = nil
	return drained
}

// acpProtocol translates ACP (JSON-RPC 2.0 over lines) of `agent acp` into domain events. It is a state
// machine: on start it sends only initialize, then authenticate and session/new (or session/load when
// resuming). It holds the prompts until the session is open (and the model applied), then sends them one
// at a time. Everything it writes to stdin goes to the outgoing queue (TakeOutgoing). It runs in one
// direction per ProcessRuntime, so it needs no locking of its own.
type acpProtocol struct {
	config    acpSessionConfig
	onSession func(sessionID string)
	nextID    uint64
	requests  map[uint64]rpcRequest
	phase     acpPhase
	// sessionID is nil until the session is open.
	sessionID      *string
	queued         [][]any
	promptInFlight bool
	outgoing       []string
	running        bool
	text           string
	thinking       string
	tools          map[string]*toolState
	dialogs        dialogBook
	// abandoned holds dialogs cancelled while a turn was in flight; they are resolved when the turn settles.
	abandoned []string
	todos     []todo
}

var _ runtime.LineCodec = (*acpProtocol)(nil)

// newACPProtocol returns the protocol of one agent process. Start must be called before anything else.
func newACPProtocol(config acpSessionConfig) *acpProtocol {
	return &acpProtocol{
		config:   config,
		nextID:   1,
		requests: make(map[uint64]rpcRequest),
		tools:    make(map[string]*toolState),
		dialogs:  newDialogBook(),
	}
}

// withSessionListener sets the function called with the session id every time a session is open, new or resumed.
func (p *acpProtocol) withSessionListener(listener func(sessionID string)) *acpProtocol {
	p.onSession = listener
	return p
}

// Start returns the first request of the handshake, initialize.
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

// TakeOutgoing returns the lines queued for stdin and clears the queue.
func (p *acpProtocol) TakeOutgoing() []string {
	lines := p.outgoing
	p.outgoing = nil
	return lines
}

// Encode turns a command into protocol state. Nothing is written on its own: the requests it needs are queued
// and come out through TakeOutgoing, so the returned line is always absent.
func (p *acpProtocol) Encode(command models.AgentCommand, _ string) (string, bool, error) {
	switch typed := command.(type) {
	case models.PromptCommand:
		if p.phase == phaseFailed {
			return "", false, werrors.Runtime("el agente de Cursor no está disponible")
		}
		p.queued = append(p.queued, promptContent(typed.Message, typed.Images))
		p.pump()
	case models.SteerCommand, models.FollowUpCommand:
		return "", false, werrors.Unsupported("Cursor no admite mensajes durante la ejecución")
	case models.AbortCommand:
		p.abort()
	case models.SetModeCommand:
		if err := p.setMode(typed.Mode); err != nil {
			return "", false, err
		}
	case models.AnswerDialogCommand:
		if err := p.answerDialog(typed.DialogID, typed.Answer); err != nil {
			return "", false, err
		}
	default:
		return "", false, werrors.Unsupported(fmt.Sprintf("comando de agente no soportado: %v", command))
	}
	return "", false, nil
}

// Decode returns the events of one stdout line. Lines that are not a JSON object, and records that carry
// neither a method nor an id, produce no events.
func (p *acpProtocol) Decode(_ models.TaskID, line string) []models.AgentEvent {
	record, ok := parseRecord(line)
	if !ok {
		return nil
	}
	return p.decodeRecord(record)
}

// sendRequest queues one of our requests and remembers its kind until the reply arrives.
func (p *acpProtocol) sendRequest(request rpcRequest, method string, params map[string]any) {
	id := p.nextID
	p.nextID++
	p.requests[id] = request
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}))
}

func (p *acpProtocol) sendNotification(method string, params map[string]any) {
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{"jsonrpc": "2.0", "method": method, "params": params}))
}

func (p *acpProtocol) reply(id any, result any) {
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}))
}

func (p *acpProtocol) replyError(id any, code int, message string) {
	p.outgoing = append(p.outgoing, encodeLine(map[string]any{
		"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message},
	}))
}

// openSession reopens the saved session when there is one, and opens a new one otherwise.
func (p *acpProtocol) openSession() {
	if p.config.resumeSession == nil {
		p.openNewSession()
		return
	}
	p.sendRequest(rpcRequest{kind: rpcLoadSession}, "session/load", map[string]any{
		"sessionId": *p.config.resumeSession, "cwd": p.config.cwd, "mcpServers": []any{},
	})
}

func (p *acpProtocol) openNewSession() {
	p.sendRequest(rpcRequest{kind: rpcNewSession}, "session/new", map[string]any{
		"cwd": p.config.cwd, "mcpServers": []any{},
	})
}

// sessionOpened records the session and applies the model, if any, before the held prompts are released.
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
		"sessionId": sessionID, "modelId": *p.config.model,
	})
}

// setMode asks Cursor to change mode. The new mode is reported once Cursor confirms it.
func (p *acpProtocol) setMode(mode models.AgentMode) error {
	if p.sessionID == nil || p.phase != phaseReady {
		return werrors.Runtime("el agente de Cursor aún no tiene sesión abierta")
	}
	p.sendRequest(rpcRequest{kind: rpcSetMode, mode: mode}, "session/set_mode", map[string]any{
		"sessionId": *p.sessionID, "modeId": string(mode),
	})
	return nil
}

func (p *acpProtocol) becomeReady() {
	p.phase = phaseReady
	p.pump()
}

// pump sends the next held prompt when the session is ready and no other turn is in flight.
func (p *acpProtocol) pump() {
	if p.phase != phaseReady || p.promptInFlight || p.sessionID == nil || len(p.queued) == 0 {
		return
	}
	next := p.queued[0]
	p.queued = p.queued[1:]
	p.promptInFlight = true
	p.sendRequest(rpcRequest{kind: rpcPrompt}, "session/prompt", map[string]any{
		"sessionId": *p.sessionID, "prompt": next,
	})
}

// fail ends the protocol with an error. A turn that was pending is settled too.
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

// startRun opens the run with RunStarted. Only the first event of a turn emits it.
func (p *acpProtocol) startRun(events []models.AgentEvent) []models.AgentEvent {
	if p.running {
		return events
	}
	p.running = true
	return append(events, models.RunStarted{})
}

// flushMessage turns the text and the reasoning accumulated so far into one assistant message.
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

// decodeRecord routes one record of the agent: a request, a notification, or the reply to one of ours.
func (p *acpProtocol) decodeRecord(record map[string]any) []models.AgentEvent {
	method, hasMethod := strField(record, "method")
	id, hasID := lookup(record, "id")
	hasID = hasID && id != nil
	switch {
	case hasMethod && hasID:
		return p.decodeRequest(method, id, record)
	case hasMethod:
		return p.decodeNotification(method, record)
	case hasID:
		return p.decodeResponse(id, record)
	default:
		return nil
	}
}

// decodeResponse settles the reply to one of our requests. Replies to unknown ids are ignored.
func (p *acpProtocol) decodeResponse(id any, record map[string]any) []models.AgentEvent {
	key, isNumber := asUint64(id)
	if !isNumber {
		return nil
	}
	request, known := p.requests[key]
	if !known {
		return nil
	}
	delete(p.requests, key)
	var errText *string
	if rawError, failed := lookup(record, "error"); failed {
		message := errorMessage(rawError)
		errText = &message
	}
	result := field(record, "result")
	switch request.kind {
	case rpcInitialize:
		if errText != nil {
			return p.fail("no se pudo iniciar ACP: " + *errText)
		}
		p.sendRequest(rpcRequest{kind: rpcAuthenticate}, "authenticate", map[string]any{"methodId": authMethod})
		return nil
	case rpcAuthenticate:
		if errText != nil {
			return p.fail("autenticación rechazada (ejecuta `agent login`): " + *errText)
		}
		p.openSession()
		return nil
	case rpcNewSession:
		sessionID, hasSessionID := strField(result, "sessionId")
		switch {
		case errText == nil && hasSessionID:
			p.sessionOpened(sessionID)
			return modeStatus(result)
		case errText != nil:
			return p.fail("no se pudo abrir la sesión: " + *errText)
		default:
			return p.fail("la sesión de ACP no devolvió sessionId")
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
		var events []models.AgentEvent
		if errText != nil {
			events = append(events, notice(models.NoticeWarning, "no se pudo aplicar el modelo: "+*errText))
		}
		p.becomeReady()
		return events
	case rpcSetMode:
		if errText != nil {
			return []models.AgentEvent{notice(models.NoticeWarning, "no se pudo cambiar el modo: "+*errText)}
		}
		return []models.AgentEvent{modeEvent(string(request.mode))}
	case rpcPrompt:
		return p.settle(errText, result)
	default:
		return nil
	}
}

// settle closes the turn: the message is flushed, the run is settled and the next held prompt is sent.
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
	// A dialog that outlives the turn has no request left to answer: it must be closed in the UI, or it
	// would stay pending for ever.
	stale := p.abandoned
	p.abandoned = nil
	for _, open := range p.dialogs.drain() {
		stale = append(stale, dialogIDs(open.key, open.dialog)...)
	}
	for _, dialogID := range stale {
		events = append(events, models.DialogResolved{DialogID: dialogID})
	}
	p.pump()
	return events
}

// decodeNotification handles the notifications of the agent. Session updates count only once the session is open.
func (p *acpProtocol) decodeNotification(method string, record map[string]any) []models.AgentEvent {
	params := field(record, "params")
	switch method {
	case "session/update":
		if p.phase == phaseReady {
			return p.decodeUpdate(params)
		}
		return nil
	case "cursor/update_todos":
		return p.updateTodos(params)
	default:
		return nil
	}
}

func (p *acpProtocol) decodeUpdate(params any) []models.AgentEvent {
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

// toolCall registers a new tool call and merges its first update into it.
func (p *acpProtocol) toolCall(update any) []models.AgentEvent {
	callID := stringOf(update, "toolCallId")
	p.tools[callID] = &toolState{}
	return p.toolCallUpdateFor(callID, update)
}

func (p *acpProtocol) toolCallUpdate(update any) []models.AgentEvent {
	return p.toolCallUpdateFor(stringOf(update, "toolCallId"), update)
}

// toolCallUpdateFor merges the data of an update into its tool. ToolStarted is emitted once the arguments are
// known (the first notification often carries rawInput: {}) or once the tool moves on.
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

// updateTodos replaces the todo list, or merges the items into it, and publishes the widget.
func (p *acpProtocol) updateTodos(params any) []models.AgentEvent {
	incoming := parseTodos(params)
	merge, _ := field(params, "merge").(bool)
	if !merge {
		p.todos = nil
	}
	for _, item := range incoming {
		if index := slices.IndexFunc(p.todos, func(existing todo) bool { return existing.id == item.id }); index >= 0 {
			p.todos[index] = item
		} else {
			p.todos = append(p.todos, item)
		}
	}
	var lines *[]string
	if len(p.todos) > 0 {
		rendered := make([]string, 0, len(p.todos))
		for _, item := range p.todos {
			rendered = append(rendered, todoMarker(item.status)+" "+item.content)
		}
		lines = &rendered
	}
	return []models.AgentEvent{models.WidgetChanged{Key: todosWidgetKey, Lines: lines}}
}

// decodeRequest answers the requests the agent makes to the client: permissions, questions, plans and todos.
func (p *acpProtocol) decodeRequest(method string, id any, record map[string]any) []models.AgentEvent {
	params := field(record, "params")
	switch method {
	case "session/request_permission":
		return p.requestPermission(id, params)
	case "cursor/ask_question":
		return p.askQuestion(id, params)
	case "cursor/create_plan":
		return p.createPlan(id, params)
	case "cursor/update_todos":
		events := p.updateTodos(params)
		todos := make([]any, 0, len(p.todos))
		for _, item := range p.todos {
			todos = append(todos, map[string]any{"id": item.id, "content": item.content, "status": item.status})
		}
		p.reply(id, map[string]any{"outcome": map[string]any{"outcome": "accepted", "todos": todos}})
		return events
	case "cursor/task", "cursor/generate_image":
		p.reply(id, cancelled())
		return nil
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

func (p *acpProtocol) createPlan(id any, params any) []models.AgentEvent {
	dialogID := "plan-" + idKey(id)
	p.dialogs.put(dialogID, pendingPlan{rpcID: id})
	title := stringOf(params, "name")
	if title == "" {
		title = defaultPlanTitle
	}
	var message *string
	if plan, ok := strField(params, "plan"); ok {
		message = &plan
	}
	return []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    dialogID,
		Title: title,
		Kind:  models.ConfirmDialog{Message: message},
	}}}
}

func (p *acpProtocol) askQuestion(id any, params any) []models.AgentEvent {
	questions := parseQuestions(params)
	if len(questions) == 0 {
		p.reply(id, map[string]any{"outcome": map[string]any{"outcome": "skipped"}})
		return nil
	}
	base := "ask-" + idKey(id)
	events := make([]models.AgentEvent, 0, len(questions))
	for index, question := range questions {
		labels := make([]string, 0, len(question.options))
		for _, option := range question.options {
			labels = append(labels, option.label)
		}
		events = append(events, models.DialogRequested{Request: models.DialogRequest{
			ID:    base + questionIDSeparator + strconv.Itoa(index),
			Title: question.prompt,
			Kind:  models.SelectDialog{Options: labels},
		}})
	}
	p.dialogs.put(base, pendingQuestions{rpcID: id, questions: questions, answers: make([]*string, len(questions))})
	return events
}

// answerDialog delivers the user's answer to the agent. A rejected answer leaves the request pending, so the
// agent never waits for a reply nobody can send.
func (p *acpProtocol) answerDialog(dialogID string, answer models.DialogAnswer) error {
	unknown := werrors.Runtime(fmt.Sprintf("no hay un diálogo pendiente con id %s", dialogID))
	key := dialogID
	index := 0
	indexed := false
	if cut := strings.LastIndex(dialogID, questionIDSeparator); cut >= 0 {
		parsed, err := strconv.Atoi(dialogID[cut+len(questionIDSeparator):])
		if err != nil || parsed < 0 {
			return unknown
		}
		key, index, indexed = dialogID[:cut], parsed, true
	}
	// Validate before removing: a rejected answer must leave the request pending.
	pending, found := p.dialogs.get(key)
	if !found {
		return unknown
	}
	switch dialog := pending.(type) {
	case pendingPermission, pendingPlan:
		if indexed {
			return unknown
		}
	case pendingQuestions:
		if !indexed || index >= len(dialog.questions) {
			return unknown
		}
		if value, isValue := answer.(models.ValueAnswer); isValue {
			if _, valid := optionIDFor(dialog.questions[index].options, value.Value); !valid {
				return unknown
			}
		}
	default:
		return unknown
	}
	p.dialogs.remove(key)
	switch dialog := pending.(type) {
	case pendingPermission:
		result := cancelled()
		if confirmed, isConfirm := answer.(models.ConfirmedAnswer); isConfirm {
			option := dialog.rejectOption
			if confirmed.Confirmed {
				option = dialog.allowOption
			}
			result = selected(option)
		}
		p.reply(dialog.rpcID, result)
	case pendingPlan:
		outcome := cancelled()
		if confirmed, isConfirm := answer.(models.ConfirmedAnswer); isConfirm {
			decision := "rejected"
			if confirmed.Confirmed {
				decision = "accepted"
			}
			outcome = map[string]any{"outcome": map[string]any{"outcome": decision}}
		}
		p.reply(dialog.rpcID, outcome)
	case pendingQuestions:
		value, isValue := answer.(models.ValueAnswer)
		if !isValue {
			p.reply(dialog.rpcID, cancelled())
			return nil
		}
		chosen, valid := optionIDFor(dialog.questions[index].options, value.Value)
		if !valid {
			return unknown
		}
		dialog.answers[index] = &chosen
		if !allAnswered(dialog.answers) {
			p.dialogs.put(key, dialog)
			return nil
		}
		answered := make([]any, 0, len(dialog.questions))
		for position, question := range dialog.questions {
			answered = append(answered, map[string]any{
				"questionId":        question.id,
				"selectedOptionIds": selectedIDs(dialog.answers[position]),
			})
		}
		p.reply(dialog.rpcID, map[string]any{"outcome": map[string]any{"outcome": "answered", "answers": answered}})
	default:
		return unknown
	}
	return nil
}

// abort cancels the turn: queued prompts are dropped, every pending request is answered as cancelled, and the
// agent is told to cancel the turn in flight.
func (p *acpProtocol) abort() {
	p.queued = nil
	for _, open := range p.dialogs.drain() {
		// Only a turn in flight guarantees the settle that resolves them; without one the list would stay alive
		// into an unrelated turn.
		if p.promptInFlight {
			p.abandoned = append(p.abandoned, dialogIDs(open.key, open.dialog)...)
		}
		p.reply(open.dialog.requestID(), cancelled())
	}
	if p.sessionID != nil && p.promptInFlight {
		p.sendNotification("session/cancel", map[string]any{"sessionId": *p.sessionID})
	}
}

// dialogIDs lists the ids the UI knows for one pending request: one per question, or the request itself.
func dialogIDs(key string, dialog pendingDialog) []string {
	questions, isQuestions := dialog.(pendingQuestions)
	if !isQuestions {
		return []string{key}
	}
	ids := make([]string, 0, len(questions.questions))
	for index := range questions.questions {
		ids = append(ids, key+questionIDSeparator+strconv.Itoa(index))
	}
	return ids
}

// modeEvent is the status line that reports the working mode.
func modeEvent(mode string) models.AgentEvent {
	return models.StatusChanged{Key: modeStatusKey, Text: &mode}
}

// modeStatus reports the mode Cursor declares when the session opens (modes.currentModeId).
func modeStatus(result any) []models.AgentEvent {
	if mode, ok := strField(field(result, "modes"), "currentModeId"); ok {
		return []models.AgentEvent{modeEvent(mode)}
	}
	return nil
}

// promptContent is the content of a prompt: the text, then one ACP image block per image.
func promptContent(message string, images []models.ImageAttachment) []any {
	blocks := make([]any, 0, 1+len(images))
	blocks = append(blocks, map[string]any{"type": "text", "text": message})
	for _, image := range images {
		blocks = append(blocks, map[string]any{"type": "image", "mimeType": image.MimeType, "data": image.Data})
	}
	return blocks
}

// selected is the reply that picks an option of a permission.
func selected(optionID string) map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optionID}}
}

// cancelled is the reply that closes a request without an answer.
func cancelled() map[string]any {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

// optionIDOfKind returns the id of the first option of the given kind, or fallback when the agent does not
// offer one (or the first one has no id).
func optionIDOfKind(options []any, kind string, fallback string) string {
	index := slices.IndexFunc(options, func(option any) bool { return stringOf(option, "kind") == kind })
	if index < 0 {
		return fallback
	}
	if id, ok := strField(options[index], "optionId"); ok {
		return id
	}
	return fallback
}

// optionIDFor returns the id of the option whose label or id equals value.
func optionIDFor(options []acpOption, value string) (string, bool) {
	index := slices.IndexFunc(options, func(option acpOption) bool { return option.label == value || option.id == value })
	if index < 0 {
		return "", false
	}
	return options[index].id, true
}

// selectedIDs is the list of option ids chosen for one question: one id, or none while unanswered.
func selectedIDs(answer *string) []any {
	if answer == nil {
		return []any{}
	}
	return []any{*answer}
}

func allAnswered(answers []*string) bool {
	return !slices.ContainsFunc(answers, func(answer *string) bool { return answer == nil })
}

// parseQuestions reads the questions of cursor/ask_question. Questions without options cannot be asked and are dropped.
func parseQuestions(params any) []acpQuestion {
	items, _ := field(params, "questions").([]any)
	var questions []acpQuestion
	for _, item := range items {
		rawOptions, _ := field(item, "options").([]any)
		var options []acpOption
		for _, raw := range rawOptions {
			options = append(options, acpOption{label: stringOf(raw, "label"), id: stringOf(raw, "id")})
		}
		if len(options) == 0 {
			continue
		}
		questions = append(questions, acpQuestion{prompt: stringOf(item, "prompt"), id: stringOf(item, "id"), options: options})
	}
	return questions
}

// parseTodos reads the items of cursor/update_todos.
func parseTodos(params any) []todo {
	items, _ := field(params, "todos").([]any)
	todos := make([]todo, 0, len(items))
	for _, item := range items {
		todos = append(todos, todo{id: stringOf(item, "id"), content: stringOf(item, "content"), status: stringOf(item, "status")})
	}
	return todos
}

// permissionSummary is what the permission dialog shows: the real command or file before the title.
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
		if title, ok := strField(tool, "title"); ok {
			detail = &title
		}
	}
	if detail == nil {
		return nil
	}
	summary := truncateChars(*detail, maxSummaryChars)
	return &summary
}

// toolName is the kind of the tool, or its title when the kind is missing or generic.
func toolName(state *toolState) string {
	if state.kind == "" || state.kind == "other" {
		return state.title
	}
	return state.kind
}

// toolOutput is the output of a finished tool: its rawOutput (stdout and stderr), or the content it returned.
func toolOutput(update any) string {
	if raw := field(update, "rawOutput"); raw != nil {
		if stdout, ok := strField(raw, "stdout"); ok {
			stderr := stringOf(raw, "stderr")
			if stderr == "" {
				return stdout
			}
			return stdout + "\n" + stderr
		}
		return valueText(raw)
	}
	items, _ := field(update, "content").([]any)
	var parts []string
	for _, item := range items {
		switch stringOf(item, "type") {
		case "diff":
			parts = append(parts, stringOf(item, "path")+"\n"+stringOf(item, "newText"))
		case "content":
			if text, ok := strField(field(item, "content"), "text"); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// exitCodeFailed reports whether the rawOutput of a tool carries a non-zero integer exit code.
func exitCodeFailed(update any) bool {
	code, isInteger := asInt64(field(field(update, "rawOutput"), "exitCode"))
	return isInteger && code != 0
}

// todoMarker is the checkbox that shows the status of a todo.
func todoMarker(status string) string {
	switch status {
	case "completed":
		return "[x]"
	case "in_progress":
		return "[~]"
	case "cancelled":
		return "[-]"
	default:
		return "[ ]"
	}
}

// errorMessage is the text of an error of the agent: its message, or the whole error when it has none.
func errorMessage(value any) string {
	if message, ok := strField(value, "message"); ok {
		return message
	}
	return encodeLine(value)
}

// idKey is the text of a JSON-RPC id, to compose dialog ids from it.
func idKey(id any) string {
	if text, ok := id.(string); ok {
		return text
	}
	return encodeLine(id)
}

// isEmptyValue reports whether a JSON value carries nothing: null, an empty object, string or array.
func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		return len(typed) == 0
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

// valueText is a JSON value as the user reads it: strings as they are, anything else as JSON.
func valueText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return encodeLine(typed)
	}
}

// chunkText is the text of a message or thought chunk.
func chunkText(update any) string {
	return stringOf(field(update, "content"), "text")
}

func notice(level models.NoticeLevel, message string) models.AgentEvent {
	return models.Notice{Level: level, Message: message}
}

// errorEvent is an error of the agent: a usage limit (ACP gives no renewal instant, only text), or a plain error.
func errorEvent(message string) models.AgentEvent {
	if models.LooksLikeUsageLimit(message) {
		return models.UsageLimitReached{Message: message}
	}
	return notice(models.NoticeError, message)
}

func truncate(text string) string {
	return truncateChars(text, maxToolTextChars)
}

// truncateChars keeps at most limit characters (runes), as the Rust version counts them.
func truncateChars(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// lookup returns a member of a JSON object and whether it is present.
func lookup(value any, key string) (any, bool) {
	object, isObject := value.(map[string]any)
	if !isObject {
		return nil, false
	}
	member, present := object[key]
	return member, present
}

// field returns a member of a JSON object, or nil when it is absent.
func field(value any, key string) any {
	member, _ := lookup(value, key)
	return member
}

// strField returns a member of a JSON object when it is a string.
func strField(value any, key string) (string, bool) {
	text, isString := field(value, key).(string)
	return text, isString
}

// stringOf returns a member of a JSON object when it is a string, and "" otherwise.
func stringOf(value any, key string) string {
	text, _ := strField(value, key)
	return text
}

// asUint64 returns the value of a JSON integer that is not negative.
func asUint64(value any) (uint64, bool) {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return 0, false
	}
	parsed, err := strconv.ParseUint(number.String(), 10, 64)
	return parsed, err == nil
}

// asInt64 returns the value of a JSON integer. Numbers written with a fraction or exponent are not integers.
func asInt64(value any) (int64, bool) {
	number, isNumber := value.(json.Number)
	if !isNumber {
		return 0, false
	}
	parsed, err := number.Int64()
	return parsed, err == nil
}

// parseRecord decodes one stdout line into a JSON object. Numbers stay as json.Number, so the integer checks
// behave as they do in serde_json. Anything but a single JSON object is rejected.
func parseRecord(line string) (map[string]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	record, isObject := value.(map[string]any)
	return record, isObject
}

// encodeLine renders a JSON value compactly, with sorted keys and without HTML escaping, as serde_json does in the
// Rust version. The values come from literals and from decoded JSON, so they always encode; a failure would be a
// bug and is logged instead of panicking.
func encodeLine(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		slog.Error("no se pudo codificar una línea de ACP", "error", err)
		return ""
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}
