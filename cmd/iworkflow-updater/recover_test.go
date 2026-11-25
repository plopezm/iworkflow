package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type recoveryFixture struct {
	r        recovery
	messages []string
	started  []string
	waited   []int
}

func newRecoveryFixture() *recoveryFixture {
	fx := &recoveryFixture{}
	fx.r = recovery{
		notify:   func(message string) { fx.messages = append(fx.messages, message) },
		waitExit: func(_ context.Context, pid int) error { fx.waited = append(fx.waited, pid); return nil },
		exists:   func(string) error { return nil },
		start:    func(dir string, exe string) error { fx.started = append(fx.started, dir+"|"+exe); return nil },
		logPath:  "/tmp/updater.log",
		timeout:  time.Second,
	}
	return fx
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRecoveryReopensTheAppAfterTheOldProcessExitedAndTellsTheUser(t *testing.T) {
	fx := newRecoveryFixture()

	fx.r.run(quietLogger(), "/opt/iworkflow/iworkflow-desktop", 42, errors.New("swap failed"))

	assert.Equal(t, []int{42}, fx.waited)
	assert.Equal(t, []string{"/opt/iworkflow|/opt/iworkflow/iworkflow-desktop"}, fx.started)
	assert.Len(t, fx.messages, 1)
	assert.Contains(t, fx.messages[0], "swap failed")
	assert.Contains(t, fx.messages[0], "opened again")
	assert.Contains(t, fx.messages[0], "/tmp/updater.log")
}

func TestRecoveryDoesNotStartASecondCopyWhileTheOldAppStillRuns(t *testing.T) {
	fx := newRecoveryFixture()
	fx.r.waitExit = func(context.Context, int) error { return context.DeadlineExceeded }

	fx.r.run(quietLogger(), "/opt/iworkflow/iworkflow-desktop", 42, errors.New("wait timed out"))

	assert.Empty(t, fx.started)
	assert.Len(t, fx.messages, 1, "the user is still told")
	assert.Contains(t, fx.messages[0], "open it again")
}

func TestRecoveryDoesNotReopenAMissingExecutable(t *testing.T) {
	fx := newRecoveryFixture()
	fx.r.exists = func(string) error { return errors.New("gone") }

	fx.r.run(quietLogger(), "/opt/iworkflow/iworkflow-desktop", 42, errors.New("boom"))

	assert.Empty(t, fx.started)
	assert.Len(t, fx.messages, 1)
}

func TestRecoveryWithoutAnExecutableOnlyNotifies(t *testing.T) {
	fx := newRecoveryFixture()

	fx.r.run(quietLogger(), "", 42, errors.New("boom"))

	assert.Empty(t, fx.waited)
	assert.Empty(t, fx.started)
	assert.Len(t, fx.messages, 1)
}

func TestRecoveryReportsAStartFailureWithoutClaimingTheAppReopened(t *testing.T) {
	fx := newRecoveryFixture()
	fx.r.start = func(string, string) error { return errors.New("cannot start") }

	fx.r.run(quietLogger(), "/opt/iworkflow/iworkflow-desktop", 42, errors.New("boom"))

	assert.Contains(t, fx.messages[0], "open it again")
}
