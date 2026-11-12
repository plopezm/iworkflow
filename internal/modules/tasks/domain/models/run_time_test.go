package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
)

var runTimeBase = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

func at(minutes int) time.Time { return runTimeBase.Add(time.Duration(minutes) * time.Minute) }

func agent(minutes int, event models.AgentEvent) models.TaskEvent {
	return models.TaskEvent{Kind: models.AgentEventKind{Event: event}, CreatedAt: at(minutes)}
}

func status(minutes int, value models.TaskStatus) models.TaskEvent {
	return models.TaskEvent{Kind: models.StatusKind(value, nil), CreatedAt: at(minutes)}
}

func TestAgentRunTime(t *testing.T) {
	exitCode := int32(1)
	tests := []struct {
		name   string
		events []models.TaskEvent
		now    time.Time
		want   time.Duration
	}{
		{name: "no events", want: 0, now: at(100)},
		{
			name: "one settled turn",
			events: []models.TaskEvent{
				agent(0, models.RunStarted{}), status(0, models.StatusRunning),
				agent(10, models.RunSettled{}), status(10, models.StatusIdle),
			},
			now:  at(100),
			want: 10 * time.Minute,
		},
		{
			name: "waiting for the user between turns is not counted",
			events: []models.TaskEvent{
				agent(0, models.RunStarted{}), agent(5, models.RunSettled{}), status(5, models.StatusIdle),
				agent(60, models.RunStarted{}), status(60, models.StatusRunning), agent(70, models.RunSettled{}),
			},
			now:  at(100),
			want: 15 * time.Minute,
		},
		{
			name: "a pending dialog is waiting for the user",
			events: []models.TaskEvent{
				agent(0, models.RunStarted{}),
				agent(2, models.DialogRequested{}),
				agent(12, models.DialogResolved{}),
				agent(14, models.RunSettled{}),
			},
			now:  at(100),
			want: 4 * time.Minute,
		},
		{
			name: "an unresolved dialog stops counting when the run closes",
			events: []models.TaskEvent{
				agent(0, models.RunStarted{}), agent(3, models.DialogRequested{}), agent(20, models.ProcessExited{Code: &exitCode}),
			},
			now:  at(100),
			want: 3 * time.Minute,
		},
		{
			name:   "a run still open counts until now",
			events: []models.TaskEvent{agent(0, models.RunStarted{}), status(0, models.StatusRunning)},
			now:    at(7),
			want:   7 * time.Minute,
		},
		{
			name:   "a usage limit closes the run",
			events: []models.TaskEvent{agent(0, models.RunStarted{}), agent(9, models.UsageLimitReached{}), status(9, models.StatusRateLimited)},
			now:    at(100),
			want:   9 * time.Minute,
		},
		{
			name:   "a status event alone opens and closes a run",
			events: []models.TaskEvent{status(0, models.StatusStarting), status(6, models.StatusInterrupted)},
			now:    at(100),
			want:   6 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, models.AgentRunTime(tt.events, tt.now))
		})
	}
}
