package inbound

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// pngHead is the signature of a PNG file: enough for the image type to be sniffed from the bytes.
var pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

type sendCall struct {
	id      models.TaskID
	message string
	images  []models.ImageAttachment
	mode    models.MessageMode
}

// fakeSender records what the composer hands to the send use case.
type fakeSender struct {
	calls []sendCall
	err   error
}

func (f *fakeSender) Execute(_ context.Context, id models.TaskID, message string, images []models.ImageAttachment, mode models.MessageMode) error {
	f.calls = append(f.calls, sendCall{id: id, message: message, images: images, mode: mode})
	return f.err
}

// fakeModes records the modes the composer asks the agent to switch to.
type fakeModes struct {
	calls []models.AgentMode
	err   error
}

func (f *fakeModes) Execute(_ context.Context, _ models.TaskID, mode models.AgentMode) error {
	f.calls = append(f.calls, mode)
	return f.err
}

// fakeHistory is the history the transcript of the composer is folded from.
type fakeHistory struct {
	events []models.TaskEvent
	err    error
}

func (f *fakeHistory) Execute(context.Context, models.TaskID, *int64) ([]models.TaskEvent, error) {
	return f.events, f.err
}

// modeReported is the history of a runtime that reports its mode under the mode status key.
func modeReported(id models.TaskID, mode string) []models.TaskEvent {
	text := mode
	return []models.TaskEvent{{
		TaskID: id,
		Kind: models.AgentEventKind{Event: models.StatusChanged{
			Key:  "mode",
			Text: &text,
		}},
		CreatedAt: time.Unix(0, 0),
	}}
}

// composerRouter mounts the composer handlers the way the module does, so the path parameters are read as in production.
func composerRouter(senders *fakeSender, modes *fakeModes, history *fakeHistory, files FileSearch) chi.Router {
	handlers := NewComposerHandlers(ComposerUseCases{Send: senders, Mode: modes, History: history}, files)
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}

// inEnglish gives the request the language of the locale middleware, so refusals are read in that language.
func inEnglish(r *http.Request) *http.Request {
	return r.WithContext(i18n.WithState(r.Context(), i18n.State{Locale: i18n.En, Chosen: true}))
}

// composerMultipart builds the multipart body the composer form sends: its fields and its image parts.
func composerMultipart(t *testing.T, target string, fields map[string]string, images map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	for name, data := range images {
		part, err := writer.CreateFormFile(name, "shot.png")
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("HX-Request", "true")
	return inEnglish(request)
}

// taskID is the identifier of the task the handler tests act on.
func taskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.ParseTaskID("0190f0a2-0000-7000-8000-000000000001")
	require.NoError(t, err)
	return id
}

func serve(router http.Handler, request *http.Request) (*httptest.ResponseRecorder, string) {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, recorder.Body.String()
}

func TestSendingAMessageHandsTheTrimmedTextImagesAndSteerModeToTheUseCase(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{}
	modes := &fakeModes{}
	router := composerRouter(senders, modes, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": "  hola  ", "mode": "steer"},
		map[string][]byte{"images": pngHead},
	)

	recorder, body := serve(router, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, senders.calls, 1, "the message reaches the send use case once")
	call := senders.calls[0]
	assert.Equal(t, taskID(t), call.id)
	assert.Equal(t, "hola", call.message)
	assert.Equal(t, models.MessageModeSteer, call.mode)
	require.Len(t, call.images, 1)
	assert.Equal(t, "image/png", call.images[0].MimeType, "the type is sniffed, not trusted")
	assert.Equal(t, base64.StdEncoding.EncodeToString(pngHead), call.images[0].Data)
	assert.Empty(t, modes.calls, "a plain message is not a mode change")
	assert.Contains(t, body, `name="message"`, "the send answers the composer body, so the field clears")
	assert.NotContains(t, body, "text-error")
}

func TestAnUnknownMessageModeIsRefusedBeforeTheUseCase(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{}
	router := composerRouter(senders, &fakeModes{}, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": "hola", "mode": "plan"},
		nil,
	)

	_, body := serve(router, request)

	assert.Empty(t, senders.calls, "a mode the adapter does not know never reaches the domain")
	assert.Contains(t, body, "The message mode is not valid.", "the refusal is shown in the request language")
	assert.Contains(t, body, "text-error")
}

func TestPlanWithAReportedModeSwitchesTheModeAndSendsNothing(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{}
	modes := &fakeModes{}
	history := &fakeHistory{events: modeReported(taskID(t), "agent")}
	router := composerRouter(senders, modes, history, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": " /plan ", "mode": "steer"},
		nil,
	)

	recorder, body := serve(router, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []models.AgentMode{models.AgentModePlan}, modes.calls, "the command switches the mode")
	assert.Empty(t, senders.calls, "a switch is not sent to the agent as a prompt")
	assert.Contains(t, body, `name="message"`)
	assert.Contains(t, body, `class="composer-mode-option is-active" aria-pressed="true"`, "the new mode shows as active")
}

func TestPlanWithoutAReportedModeIsAnOrdinaryMessage(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{}
	modes := &fakeModes{}
	router := composerRouter(senders, modes, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": "/plan", "mode": "steer"},
		nil,
	)

	_, _ = serve(router, request)

	assert.Empty(t, modes.calls, "a runtime without modes gets no switch")
	require.Len(t, senders.calls, 1)
	assert.Equal(t, "/plan", senders.calls[0].message)
}

func TestAUseCaseRefusalAnswersTheComposerWithItsText(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{err: errors.New("el agente no está en ejecución")}
	router := composerRouter(senders, &fakeModes{}, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": "hola", "mode": "steer"},
		nil,
	)

	recorder, body := serve(router, request)

	assert.Equal(t, http.StatusOK, recorder.Code, "the refusal is an answer of the composer, not a server error")
	assert.Contains(t, body, "el agente no está en ejecución")
	assert.Contains(t, body, "text-error")
	assert.Contains(t, body, `name="message"`, "the composer keeps its field after a refusal")
}

func TestAModeCommandWithImagesIsSentAsAMessage(t *testing.T) {
	t.Parallel()
	senders := &fakeSender{}
	modes := &fakeModes{}
	history := &fakeHistory{events: modeReported(taskID(t), "agent")}
	router := composerRouter(senders, modes, history, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
		map[string]string{"message": "/plan", "mode": "steer"},
		map[string][]byte{"images": pngHead},
	)

	_, _ = serve(router, request)

	assert.Empty(t, modes.calls, "a message with images is never swallowed as a switch")
	require.Len(t, senders.calls, 1, "the images travel with the message")
	assert.Len(t, senders.calls[0].images, 1)
}

func TestAModeChangeRefusalStaysInTheModeSwitch(t *testing.T) {
	t.Parallel()
	modes := &fakeModes{err: errors.New("el agente no permite cambiar de modo")}
	history := &fakeHistory{events: modeReported(taskID(t), "agent")}
	router := composerRouter(&fakeSender{}, modes, history, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/mode", map[string]string{"mode": "plan"}, nil)

	recorder, body := serve(router, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []models.AgentMode{models.AgentModePlan}, modes.calls)
	assert.Contains(t, body, "el agente no permite cambiar de modo")
	assert.NotContains(t, body, `name="message"`, "the typed text is not part of a mode answer")
}

func TestSetModeRefusesAnUnknownModeBeforeTheUseCase(t *testing.T) {
	t.Parallel()
	modes := &fakeModes{}
	router := composerRouter(&fakeSender{}, modes, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/mode", map[string]string{"mode": "ask"}, nil)

	_, body := serve(router, request)

	assert.Empty(t, modes.calls)
	assert.Contains(t, body, "The agent mode is not valid.")
}

func TestFileSuggestionsAnswerTheSearchOfTheTask(t *testing.T) {
	t.Parallel()
	var gotID models.TaskID
	var gotQuery string
	search := func(_ context.Context, id models.TaskID, query string) ([]string, error) {
		gotID, gotQuery = id, query
		return []string{"README.md"}, nil
	}
	router := composerRouter(&fakeSender{}, &fakeModes{}, &fakeHistory{}, search)
	target := "/tasks/" + taskID(t).String() + "/composer/files?" + url.Values{"q": {"READ"}}.Encode()

	recorder, body := serve(router, inEnglish(httptest.NewRequest(http.MethodGet, target, nil)))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, taskID(t), gotID)
	assert.Equal(t, "READ", gotQuery)
	assert.Contains(t, body, `data-insert="@README.md"`, "a suggestion inserts its reference")
}

func TestFileSuggestionsWithoutASearchSayNothingMatches(t *testing.T) {
	t.Parallel()
	router := composerRouter(&fakeSender{}, &fakeModes{}, &fakeHistory{}, nil)
	target := "/tasks/" + taskID(t).String() + "/composer/files?q=x"

	_, body := serve(router, inEnglish(httptest.NewRequest(http.MethodGet, target, nil)))

	assert.Contains(t, body, "No matching files")
	assert.False(t, strings.Contains(body, "composer-menu-item"))
}

func TestAFailedFileSearchIsShownInTheMenu(t *testing.T) {
	t.Parallel()
	search := func(context.Context, models.TaskID, string) ([]string, error) {
		return nil, errors.New("el worktree no existe")
	}
	router := composerRouter(&fakeSender{}, &fakeModes{}, &fakeHistory{}, search)
	target := "/tasks/" + taskID(t).String() + "/composer/files?q=x"

	_, body := serve(router, inEnglish(httptest.NewRequest(http.MethodGet, target, nil)))

	assert.Contains(t, body, "el worktree no existe")
	assert.Contains(t, body, "text-error")
}

func TestAnInvalidTaskIDIsAnsweredInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	texts := map[i18n.Locale]string{
		i18n.En: "The task identifier is not valid.",
		i18n.Es: "El identificador de la tarea no es válido.",
		i18n.Zh: "任务标识无效。",
		i18n.De: "Die Kennung der Aufgabe ist ungültig.",
		i18n.Fr: "L’identifiant de la tâche n’est pas valide.",
	}
	for locale, want := range texts {
		t.Run(string(locale), func(t *testing.T) {
			t.Parallel()
			senders := &fakeSender{}
			router := composerRouter(senders, &fakeModes{}, &fakeHistory{}, nil)
			request := composerMultipart(t, "/tasks/not-a-uuid/messages", map[string]string{"message": "hola", "mode": "steer"}, nil)
			request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: locale, Chosen: true}))

			_, body := serve(router, request)

			assert.Contains(t, body, want)
			assert.Empty(t, senders.calls)
		})
	}
}

func TestTheModeSwitchRefusesABodyOverTheMessageLimit(t *testing.T) {
	t.Parallel()
	modes := &fakeModes{}
	router := composerRouter(&fakeSender{}, modes, &fakeHistory{}, nil)
	// A multipart body is read into memory and temporary files with no limit of its own, so the switch caps it like
	// the message route does: the padding is above that cap.
	padding := strings.Repeat("x", models.MaxImagesPerMessage*models.MaxImageBytes+2<<20)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/mode", map[string]string{"mode": "plan", "pad": padding}, nil)

	_, body := serve(router, request)

	assert.Empty(t, modes.calls, "an oversized body never reaches the mode switch")
	assert.Contains(t, body, "text-error")
}

func TestARefusedMessageKeepsTheTypedTextInTheComposer(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		senders *fakeSender
		modes   *fakeModes
		history *fakeHistory
		mode    string
		message string
	}{
		"the use case refuses the send": {
			senders: &fakeSender{err: errors.New("el agente no está en ejecución")},
			modes:   &fakeModes{}, history: &fakeHistory{}, mode: "steer", message: "hola",
		},
		"the adapter refuses the mode": {
			senders: &fakeSender{}, modes: &fakeModes{}, history: &fakeHistory{}, mode: "plan", message: "hola",
		},
		"the agent refuses the mode switch": {
			senders: &fakeSender{}, modes: &fakeModes{err: errors.New("el agente no permite cambiar de modo")},
			history: &fakeHistory{events: modeReported(taskID(t), "agent")}, mode: "steer", message: "/plan",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := composerRouter(test.senders, test.modes, test.history, nil)
			request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages",
				map[string]string{"message": test.message, "mode": test.mode}, nil)

			_, body := serve(router, request)

			assert.Contains(t, body, ">"+test.message+"</textarea>", "the answer keeps the text the user typed")
			assert.Contains(t, body, "text-error")
		})
	}
}

func TestASentMessageClearsTheComposer(t *testing.T) {
	t.Parallel()
	router := composerRouter(&fakeSender{}, &fakeModes{}, &fakeHistory{}, nil)
	request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages", map[string]string{"message": "hola", "mode": "steer"}, nil)

	_, body := serve(router, request)

	assert.Contains(t, body, "></textarea>", "a delivered message leaves the field empty")
}

func TestAHistoryReadErrorIsShownInTheRequestLanguageWithoutTheInternalText(t *testing.T) {
	t.Parallel()
	readErr := errors.New("sqlite: database is locked")
	texts := map[i18n.Locale]string{
		i18n.En: "The history of the task could not be read.",
		i18n.Es: "No se pudo leer el historial de la tarea.",
		i18n.Zh: "无法读取任务的历史记录。",
		i18n.De: "Der Verlauf der Aufgabe konnte nicht gelesen werden.",
		i18n.Fr: "L’historique de la tâche n’a pas pu être lu.",
	}
	for locale, want := range texts {
		t.Run(string(locale), func(t *testing.T) {
			t.Parallel()
			inLocale := func(r *http.Request) *http.Request {
				return r.WithContext(i18n.WithState(r.Context(), i18n.State{Locale: locale, Chosen: true}))
			}

			t.Run("a send still goes out as an ordinary message", func(t *testing.T) {
				t.Parallel()
				senders := &fakeSender{}
				router := composerRouter(senders, &fakeModes{}, &fakeHistory{err: readErr}, nil)
				request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/messages", map[string]string{"message": "hola", "mode": "steer"}, nil)

				_, body := serve(router, inLocale(request))

				require.Len(t, senders.calls, 1, "without a readable history the message is an ordinary message")
				assert.Contains(t, body, want)
				assert.NotContains(t, body, "database is locked", "the internal error text is never shown")
				assert.Contains(t, body, "text-error")
			})

			t.Run("a mode change says why the switch is not offered", func(t *testing.T) {
				t.Parallel()
				router := composerRouter(&fakeSender{}, &fakeModes{}, &fakeHistory{err: readErr}, nil)
				request := composerMultipart(t, "/tasks/"+taskID(t).String()+"/mode", map[string]string{"mode": "plan"}, nil)

				_, body := serve(router, inLocale(request))

				assert.Contains(t, body, want)
				assert.NotContains(t, body, "database is locked", "the internal error text is never shown")
				assert.Contains(t, body, "text-error")
			})
		})
	}
}
