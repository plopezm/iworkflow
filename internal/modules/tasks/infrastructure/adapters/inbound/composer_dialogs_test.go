package inbound

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

type answerCall struct {
	id       models.TaskID
	dialogID string
	answer   models.DialogAnswer
}

// fakeAnswerer records the answers the composer hands to the answer use case.
type fakeAnswerer struct {
	calls []answerCall
	err   error
}

func (f *fakeAnswerer) Execute(_ context.Context, id models.TaskID, dialogID string, answer models.DialogAnswer) error {
	f.calls = append(f.calls, answerCall{id: id, dialogID: dialogID, answer: answer})
	return f.err
}

// dialogRequested is the history of a dialog the agent opened with the kind given.
func dialogRequested(id models.TaskID, dialogID string, kind models.DialogKind) models.TaskEvent {
	return models.TaskEvent{
		TaskID: id,
		Kind: models.AgentEventKind{Event: models.DialogRequested{Request: models.DialogRequest{
			ID:    dialogID,
			Title: "¿Seguir?",
			Kind:  kind,
		}}},
		CreatedAt: time.Unix(0, 0),
	}
}

// dialogResolved is the history of a dialog that was answered or closed.
func dialogResolved(id models.TaskID, dialogID string) models.TaskEvent {
	return models.TaskEvent{
		TaskID:    id,
		Kind:      models.AgentEventKind{Event: models.DialogResolved{DialogID: dialogID}},
		CreatedAt: time.Unix(0, 0),
	}
}

// answerRouter mounts the composer with the answer use case, the way the module does.
func answerRouter(answerer *fakeAnswerer, history *fakeHistory) chi.Router {
	handlers := NewComposerHandlers(ComposerUseCases{Send: &fakeSender{}, Mode: &fakeModes{}, History: history, Answer: answerer}, nil)
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}

// answerRequest is the form a dialog control of the task posts, form-encoded.
func answerRequest(t *testing.T, id models.TaskID, dialogID string, form url.Values, locale i18n.Locale) *http.Request {
	t.Helper()
	return answerRequestTo(t, "/tasks/"+id.String()+"/dialogs/"+url.PathEscape(dialogID)+"/answer", form, locale)
}

// answerRequestTo is the form-encoded answer posted to target.
func answerRequestTo(t *testing.T, target string, form url.Values, locale i18n.Locale) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.WithContext(i18n.WithState(r.Context(), i18n.State{Locale: locale, Chosen: true}))
}

// dialogMismatchTexts are the texts of an answer that does not fit the dialog, in the five languages.
var dialogMismatchTexts = map[i18n.Locale]string{
	i18n.En: "The answer does not match the dialog.",
	i18n.Es: "La respuesta no corresponde al diálogo.",
	i18n.Zh: "回答与对话框不匹配。",
	i18n.De: "Die Antwort passt nicht zu dem Dialog.",
	i18n.Fr: "La réponse ne correspond pas à la boîte de dialogue.",
}

// dialogDeliveryTexts are the texts of an answer the agent did not take, in the five languages.
var dialogDeliveryTexts = map[i18n.Locale]string{
	i18n.En: "The answer could not be delivered to the agent.",
	i18n.Es: "No se pudo enviar la respuesta al agente.",
	i18n.Zh: "无法把回答发送给智能体。",
	i18n.De: "Die Antwort konnte nicht an den Agenten gesendet werden.",
	i18n.Fr: "La réponse n’a pas pu être envoyée à l’agent.",
}

func TestAnsweredDialogDeliversTheDomainAnswerForEachKind(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	cases := map[string]struct {
		kind models.DialogKind
		form url.Values
		want models.DialogAnswer
	}{
		"value of a select":   {models.SelectDialog{Options: []string{"Sí"}}, url.Values{"kind": {"value"}, "value": {"Sí"}}, models.ValueAnswer{Value: "Sí"}},
		"value of an input":   {models.InputDialog{}, url.Values{"kind": {"value"}, "value": {"x"}}, models.ValueAnswer{Value: "x"}},
		"value of an editor":  {models.EditorDialog{}, url.Values{"kind": {"value"}, "value": {"uno\ndos"}}, models.ValueAnswer{Value: "uno\ndos"}},
		"confirmed":           {models.ConfirmDialog{}, url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, models.ConfirmedAnswer{Confirmed: true}},
		"declined":            {models.ConfirmDialog{}, url.Values{"kind": {"confirmed"}, "confirmed": {"false"}}, models.ConfirmedAnswer{Confirmed: false}},
		"cancelled of select": {models.SelectDialog{Options: []string{"Sí"}}, url.Values{"kind": {"cancelled"}}, models.CancelledAnswer{}},
	}
	for name, tc := range cases {
		answerer := &fakeAnswerer{}
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", tc.kind)}}
		recorder, _ := serve(answerRouter(answerer, history), answerRequest(t, id, "d1", tc.form, i18n.En))
		assert.Equal(t, http.StatusOK, recorder.Code, name)
		require.Len(t, answerer.calls, 1, name)
		assert.Equal(t, id, answerer.calls[0].id, name)
		assert.Equal(t, "d1", answerer.calls[0].dialogID, name)
		assert.Equal(t, tc.want, answerer.calls[0].answer, name)
	}
}

// The answer must be one the dialog takes: a value for a select, input or editor, a confirmation for a confirm. A
// mismatched answer is refused in the request language and never reaches the agent.
func TestAnswerOfAnotherKindThanTheDialogIsRefused(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	cases := map[string]struct {
		kind models.DialogKind
		form url.Values
	}{
		"value to a confirm":        {models.ConfirmDialog{}, url.Values{"kind": {"value"}, "value": {"Sí"}}},
		"confirmation to a select":  {models.SelectDialog{Options: []string{"Sí"}}, url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}},
		"confirmation to an input":  {models.InputDialog{}, url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}},
		"confirmation to an editor": {models.EditorDialog{}, url.Values{"kind": {"confirmed"}, "confirmed": {"false"}}},
	}
	for name, tc := range cases {
		answerer := &fakeAnswerer{}
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", tc.kind)}}
		recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1", tc.form, i18n.En))
		assert.Equal(t, http.StatusOK, recorder.Code, name)
		assert.Empty(t, answerer.calls, name)
		assert.Contains(t, body, dialogMismatchTexts[i18n.En], name)
		assert.Contains(t, body, "hx-post", name)
	}
}

// The refusal of a mismatched answer is shown in the language of the request, in each of the five languages.
func TestAnswerMismatchIsRefusedInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	for locale, want := range dialogMismatchTexts {
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})}}
		answerer := &fakeAnswerer{}
		_, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
			url.Values{"kind": {"value"}, "value": {"S\u00ed"}}, locale))
		assert.Contains(t, body, want, locale)
		assert.Empty(t, answerer.calls, locale)
	}
}

// A cancel closes any dialog, whatever its kind: it is the one answer every dialog takes.
func TestCancelIsDeliveredToEveryKindOfDialog(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	kinds := []models.DialogKind{models.SelectDialog{}, models.ConfirmDialog{}, models.InputDialog{}, models.EditorDialog{}}
	for _, kind := range kinds {
		answerer := &fakeAnswerer{}
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", kind)}}
		serve(answerRouter(answerer, history), answerRequest(t, id, "d1", url.Values{"kind": {"cancelled"}}, i18n.En))
		require.Len(t, answerer.calls, 1, "%T", kind)
		assert.Equal(t, models.CancelledAnswer{}, answerer.calls[0].answer, "%T", kind)
	}
}

// The answer replaces the dialog: once the agent has taken it, the dialog is a line of the transcript.
func TestAnsweredDialogIsShownAsAnsweredInItsPlace(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})}}
	answerer := &fakeAnswerer{}
	recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
		url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "answered")
	assert.Contains(t, body, `data-dialog`)
	assert.NotContains(t, body, "hx-post")
}

// A dialog whose turn is closed, or whose agent is gone, is closed: no error is shown for it, and it is not shown as
// answered, since the user did not answer it.
func TestAnswerToADialogOfAClosedTurnClosesItWithoutAnError(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	closed := map[string]error{
		"agent not running": werrors.AgentNotRunning(),
		"invalid state":     werrors.InvalidState("responder diálogo", "idle"),
	}
	for name, cause := range closed {
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})}}
		answerer := &fakeAnswerer{err: cause}
		recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
			url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, i18n.En))
		assert.Equal(t, http.StatusOK, recorder.Code, name)
		assert.Contains(t, body, "(closed)", name)
		assert.NotContains(t, body, "answered", name)
		assert.NotContains(t, body, "hx-post", name)
		assert.NotContains(t, body, dialogDeliveryTexts[i18n.En], name)
		assert.NotContains(t, strings.ToLower(body), "diálogo pendiente", name)
	}
}

// An answer the agent took is shown as answered even when its resolution could not be recorded: it was delivered, so
// no undelivered notice is shown and the dialog has no controls left.
func TestAnswerTheAgentTookButNotRecordedIsShownAsAnswered(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})}}
	answerer := &fakeAnswerer{err: werrors.AnswerNotRecorded()}
	recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
		url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Len(t, answerer.calls, 1)
	assert.Contains(t, body, "(answered)")
	assert.NotContains(t, body, dialogDeliveryTexts[i18n.En])
	assert.NotContains(t, body, "hx-post")
}

// A failure of the agent keeps the dialog open, with the controls, and says in the language of the request that the
// answer did not reach the agent. The runtime text is not shown.
func TestDeliveryFailureKeepsTheDialogOpenWithALocalizedNotice(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	for locale, want := range dialogDeliveryTexts {
		history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})}}
		answerer := &fakeAnswerer{err: werrors.Runtime("no hay un diálogo pendiente con id d1")}
		recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
			url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, locale))
		assert.Equal(t, http.StatusOK, recorder.Code, locale)
		assert.Contains(t, body, want, locale)
		assert.Contains(t, body, "hx-post", locale)
		assert.NotContains(t, body, "no hay un diálogo pendiente", locale)
	}
}

// A refused answer does not reach the use case; the dialog stays open with the refusal in the request language.
func TestRefusedAnswerKeepsTheDialogOpen(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	history := &fakeHistory{events: []models.TaskEvent{dialogRequested(id, "d1", models.InputDialog{})}}
	answerer := &fakeAnswerer{}
	recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
		url.Values{"kind": {"maybe"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, answerer.calls)
	assert.Contains(t, body, "The answer to the dialog is not valid.")
	assert.Contains(t, body, "hx-post")
}

func TestAnswerForAnInvalidTaskIsAnsweredInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	answerer := &fakeAnswerer{}
	request := answerRequestTo(t, "/tasks/not-a-task/dialogs/d1/answer", url.Values{"kind": {"cancelled"}}, i18n.En)
	recorder, body := serve(answerRouter(answerer, &fakeHistory{}), request)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, msgInvalidTask.En)
	assert.Empty(t, answerer.calls)
}

// The history is read before the answer: without it the dialog cannot be drawn, so nothing is answered.
func TestAnswerWithAnUnreadableHistoryIsNotDelivered(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	answerer := &fakeAnswerer{}
	history := &fakeHistory{err: context.DeadlineExceeded}
	recorder, body := serve(answerRouter(answerer, history), answerRequest(t, id, "d1",
		url.Values{"kind": {"cancelled"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, msgHistoryUnavailable.En)
	assert.Empty(t, answerer.calls)
}

// A dialog the transcript does not hold, or one already answered, is never delivered to the agent. A dialog the
// transcript no longer holds is answered as gone: the response is empty, so its element is removed.
func TestAnswerToAnUnknownOrResolvedDialogDeliversNothing(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	answerer := &fakeAnswerer{}
	recorder, body := serve(answerRouter(answerer, &fakeHistory{}), answerRequest(t, id, "gone",
		url.Values{"kind": {"cancelled"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, strings.TrimSpace(body))
	assert.Empty(t, answerer.calls)

	answered := &fakeHistory{events: []models.TaskEvent{
		dialogRequested(id, "d1", models.ConfirmDialog{}),
		dialogResolved(id, "d1"),
	}}
	answerer = &fakeAnswerer{}
	recorder, body = serve(answerRouter(answerer, answered), answerRequest(t, id, "d1",
		url.Values{"kind": {"cancelled"}}, i18n.En))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "answered")
	assert.Empty(t, answerer.calls)
}

// Without the answer use case the route is not mounted, so the answer is not found instead of failing on a nil use case.
func TestAnswerRouteIsMountedOnlyWithTheAnswerUseCase(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	handlers := NewComposerHandlers(ComposerUseCases{Send: &fakeSender{}, Mode: &fakeModes{}, History: &fakeHistory{}}, nil)
	router := chi.NewRouter()
	handlers.Mount(router)
	recorder, _ := serve(router, answerRequest(t, id, "d1", url.Values{"kind": {"cancelled"}}, i18n.En))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// sharedDialogs is the history of one dialog that two answers race for. A read waits, for a moment, until a second
// reader arrives: two answers that both read before either records its resolution are then seen together.
type sharedDialogs struct {
	mu       sync.Mutex
	events   []models.TaskEvent
	reads    atomic.Int32
	bothRead chan struct{}
	once     sync.Once
}

func (s *sharedDialogs) Execute(context.Context, models.TaskID, *int64) ([]models.TaskEvent, error) {
	if s.reads.Add(1) == 2 {
		s.once.Do(func() { close(s.bothRead) })
	}
	select {
	case <-s.bothRead:
	case <-time.After(300 * time.Millisecond):
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]models.TaskEvent(nil), s.events...), nil
}

// sharedAnswerer delivers answers to sharedDialogs: it counts the deliveries and records the resolution, as the use
// case does.
type sharedAnswerer struct {
	dialogs   *sharedDialogs
	delivered atomic.Int32
}

func (a *sharedAnswerer) Execute(_ context.Context, id models.TaskID, dialogID string, _ models.DialogAnswer) error {
	a.delivered.Add(1)
	a.dialogs.mu.Lock()
	defer a.dialogs.mu.Unlock()
	a.dialogs.events = append(a.dialogs.events, dialogResolved(id, dialogID))
	return nil
}

// Two answers to the same dialog that arrive together are delivered once: the second finds the dialog answered.
func TestTwoAnswersToOneDialogDeliverItOnce(t *testing.T) {
	t.Parallel()
	id := taskID(t)
	dialogs := &sharedDialogs{
		bothRead: make(chan struct{}),
		events:   []models.TaskEvent{dialogRequested(id, "d1", models.ConfirmDialog{})},
	}
	answerer := &sharedAnswerer{dialogs: dialogs}
	router := chi.NewRouter()
	NewComposerHandlers(ComposerUseCases{Send: &fakeSender{}, Mode: &fakeModes{}, History: dialogs, Answer: answerer}, nil).Mount(router)
	form := url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}
	first := answerRequest(t, id, "d1", form, i18n.En)
	second := answerRequest(t, id, "d1", form, i18n.En)

	var wg sync.WaitGroup
	for _, request := range []*http.Request{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			serve(router, request)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), answerer.delivered.Load())
}
