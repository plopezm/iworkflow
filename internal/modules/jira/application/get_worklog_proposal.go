//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"time"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// WorklogProposal is what the worklog form of a task starts from: the time not logged yet, the description built
// from the task, and whether the task has finished its work.
type WorklogProposal struct {
	Finished    bool
	Time        time.Duration
	Logged      time.Duration
	Description string
}

// GetWorklogProposal proposes a worklog for a task. The time is the agent working time not logged yet, so a task
// registered twice proposes only what is new.
type GetWorklogProposal struct {
	taskInfo outbound.TaskInfo
	logged   *ListTaskWorklogs
}

// NewGetWorklogProposal builds the use case.
func NewGetWorklogProposal(taskInfo outbound.TaskInfo, logged *ListTaskWorklogs) *GetWorklogProposal {
	return &GetWorklogProposal{taskInfo: taskInfo, logged: logged}
}

// Execute reads the task and what was already logged for it.
func (u *GetWorklogProposal) Execute(ctx context.Context, taskID string) (WorklogProposal, error) {
	info, err := u.taskInfo.Info(ctx, taskID)
	if err != nil {
		return WorklogProposal{}, err
	}
	worklogs, err := u.logged.Execute(ctx, taskID)
	if err != nil {
		return WorklogProposal{}, err
	}
	return WorklogProposal{
		Finished:    info.Finished,
		Time:        models.ProposeWorklogTime(info.AgentRunTime, worklogs.Total),
		Logged:      worklogs.Total,
		Description: models.WorklogDescription(info.Title, info.Commits),
	}, nil
}
