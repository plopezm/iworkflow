//go:build windows

package process

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Survivors keeps one process and everything it starts in a Windows Job Object configured with
// KILL_ON_JOB_CLOSE. Children inherit the job, so a server that detaches from its parent is still inside;
// closing the job kills them all, even when the parent already exited. If this process dies, the OS closes
// the handle and kills them too.
type Survivors struct {
	mu  sync.Mutex
	job windows.Handle
	// settled is set by the first Track attempt, successful or not, and by Kill: a failure is final, so a
	// persistent one is neither retried nor logged every tick.
	settled bool
}

// NewSurvivors returns an empty set; the job is created by the first Track.
func NewSurvivors() *Survivors { return &Survivors{} }

// Track puts root in the job on the first call and never retries. Descendants join on their own.
// Processes started before the assignment are not inside; the window is the time between Start and Track.
func (s *Survivors) Track(_ context.Context, root int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled || root <= 0 {
		return
	}
	s.settled = true
	job, err := newKillOnCloseJob()
	if err != nil {
		slog.Warn("no se pudo crear el job del agente", "error", err)
		return
	}
	if err := assignToJob(job, uint32(root)); err != nil {
		_ = windows.CloseHandle(job)
		slog.Warn("no se pudo asignar el agente a su job", "pid", root, "error", err)
		return
	}
	s.job = job
}

// Kill closes the job, which terminates every process still inside. It returns 1 when a job was closed
// (the number of processes is not reported) and 0 when there was none.
func (s *Survivors) Kill() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settled = true
	if s.job == 0 {
		return 0
	}
	job := s.job
	s.job = 0
	if err := windows.CloseHandle(job); err != nil {
		slog.Warn("no se pudo cerrar el job del agente", "error", err)
		return 0
	}
	return 1
}

func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil { //nolint:gosec // size of a fixed struct
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("set job limits: %w", err)
	}
	return job, nil
}

func assignToJob(job windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(process) //nolint:errcheck // nothing to do if closing fails
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return fmt.Errorf("assign process %d: %w", pid, err)
	}
	return nil
}
