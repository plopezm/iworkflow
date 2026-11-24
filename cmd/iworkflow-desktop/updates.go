package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"iworkflow/internal/desktop"
)

// version is stamped at build time (-ldflags "-X main.version=..."); "dev" never offers an update.
var version = "dev"

// newUpdateFlow builds the update flow of the app, or nil when updates are off for this run. A run with
// IWF_SERVER_DIR set is a development run (make wails-dev), never a packaged app.
func (s *shell) newUpdateFlow() *desktop.Flow {
	strategy := desktop.UpdateStrategy(desktop.PolicyInput{
		GOOS:     runtime.GOOS,
		Packaged: os.Getenv("IWF_SERVER_DIR") == "",
		Getenv:   os.Getenv,
	})
	if strategy == desktop.StrategyOff {
		return nil
	}
	flow, err := desktop.NewFlow(desktop.Config{
		Strategy:   strategy,
		AppVersion: version,
		GHToken:    os.Getenv("GH_TOKEN"),
		Ask:        s.ask,
		OpenExternal: func(_ context.Context, url string) error {
			if err := s.app.Browser.OpenURL(url); err != nil {
				return fmt.Errorf("opening %s: %w", url, err)
			}
			return nil
		},
		// The installer cannot replace files while the server runs.
		BeforeInstall: func(context.Context) error {
			s.super.Stop()
			return nil
		},
		Recover: s.app.Quit,
		Log:     func(message string) { s.logger.Info("update", "message", message) },
	})
	if err != nil {
		s.logger.Warn("updates disabled", "error", err)
		return nil
	}
	return flow
}

// ask shows a dialog and returns the index of the button the user pressed. A question without buttons
// is acknowledged with a single OK.
func (s *shell) ask(ctx context.Context, question desktop.Question) (int, error) {
	buttons := question.Buttons
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}
	chosen := make(chan int, 1)
	application.InvokeSync(func() {
		var dialog *application.MessageDialog
		if question.Type == desktop.QuestionError {
			dialog = s.app.Dialog.Error()
		} else {
			dialog = s.app.Dialog.Info()
		}
		dialog.SetTitle(appTitle).SetMessage(joinMessage(question))
		for index, label := range buttons {
			button := dialog.AddButton(label).OnClick(func() {
				select {
				case chosen <- index:
				default:
				}
			})
			if index == question.DefaultID {
				dialog.SetDefaultButton(button)
			}
			if index == question.CancelID {
				dialog.SetCancelButton(button)
			}
		}
		dialog.Show()
	})
	select {
	case index := <-chosen:
		return index, nil
	case <-ctx.Done():
		return question.CancelID, fmt.Errorf("waiting for the dialog: %w", ctx.Err())
	}
}

func joinMessage(question desktop.Question) string {
	if question.Detail == "" {
		return question.Message
	}
	return question.Message + "\n\n" + question.Detail
}
