//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// TaskAttachmentRepository stores the metadata of the images of a task.
//
// Positions are appended after the highest existing one. The read of that maximum and the insert
// happen in one BEGIN IMMEDIATE transaction, so two concurrent appends cannot pick the same position.
// The repository is not wrapped by the retry decorator, as in the Rust version: a busy error reaches the caller.
type TaskAttachmentRepository struct {
	db      *sql.DB
	queries *gen.Queries
	clock   Clock
}

var _ outbound.TaskAttachmentRepository = (*TaskAttachmentRepository)(nil)

// NewTaskAttachmentRepository builds the repository on an open database.
func NewTaskAttachmentRepository(db *sql.DB, clock Clock) *TaskAttachmentRepository {
	return &TaskAttachmentRepository{db: db, queries: gen.New(db), clock: clock}
}

// InsertMany stores the metadata in the received order, after the existing rows, in one transaction.
// Nothing is stored when any row fails (a missing task is one such failure).
func (r *TaskAttachmentRepository) InsertMany(ctx context.Context, taskID models.TaskID, attachments []models.StoredAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	createdAt := formatTimestamp(r.clock())
	return withImmediateTx(ctx, r.db, func(queries *gen.Queries) error {
		maxPosition, err := queries.MaxTaskAttachmentPosition(ctx, taskID.String())
		if err != nil {
			return dbError(err)
		}
		for index, attachment := range attachments {
			if attachment.SizeBytes > math.MaxInt32 {
				return werrors.Repository(fmt.Sprintf("tamaño de adjunto inválido: %d", attachment.SizeBytes))
			}
			err := queries.InsertTaskAttachment(ctx, gen.InsertTaskAttachmentParams{
				ID:        attachment.ID,
				TaskID:    taskID.String(),
				Position:  maxPosition + 1 + int64(index),
				Name:      attachment.Name,
				MimeType:  attachment.MimeType,
				Path:      attachment.Path,
				SizeBytes: int64(attachment.SizeBytes),
				CreatedAt: createdAt,
			})
			if err != nil {
				return dbError(err)
			}
		}
		return nil
	})
}

// ListByTask returns the metadata of the images of the task ordered by position.
func (r *TaskAttachmentRepository) ListByTask(ctx context.Context, taskID models.TaskID) ([]models.StoredAttachment, error) {
	rows, err := r.queries.ListTaskAttachments(ctx, taskID.String())
	if err != nil {
		return nil, dbError(err)
	}
	return rowsToAttachments(rows)
}

// Remove deletes only the rows of the task whose id is in ids, in one transaction, and returns the
// deleted rows in position order so the caller can delete their files. Unknown ids are ignored.
func (r *TaskAttachmentRepository) Remove(ctx context.Context, taskID models.TaskID, ids []string) ([]models.StoredAttachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var removed []models.StoredAttachment
	idList := jsonArray(ids)
	err := withImmediateTx(ctx, r.db, func(queries *gen.Queries) error {
		rows, err := queries.ListTaskAttachmentsByIDs(ctx, gen.ListTaskAttachmentsByIDsParams{TaskID: taskID.String(), Ids: idList})
		if err != nil {
			return dbError(err)
		}
		removed, err = rowsToAttachments(rows)
		if err != nil {
			return err
		}
		if err := queries.DeleteTaskAttachmentsByIDs(ctx, gen.DeleteTaskAttachmentsByIDsParams{TaskID: taskID.String(), Ids: idList}); err != nil {
			return dbError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}

// DeleteByTask deletes every image metadata row of the task.
func (r *TaskAttachmentRepository) DeleteByTask(ctx context.Context, taskID models.TaskID) error {
	if err := r.queries.DeleteTaskAttachmentsByTask(ctx, taskID.String()); err != nil {
		return dbError(err)
	}
	return nil
}

// rowsToAttachments maps stored rows to the domain. Malformed stored data is a repository error.
func rowsToAttachments(rows []gen.TaskAttachment) ([]models.StoredAttachment, error) {
	attachments := make([]models.StoredAttachment, 0, len(rows))
	for _, row := range rows {
		if row.SizeBytes < 0 {
			return nil, werrors.Repository(fmt.Sprintf("tamaño de adjunto inválido: %d", row.SizeBytes))
		}
		attachments = append(attachments, models.StoredAttachment{
			ID:        row.ID,
			Name:      row.Name,
			MimeType:  row.MimeType,
			Path:      row.Path,
			SizeBytes: uint64(row.SizeBytes),
		})
	}
	return attachments, nil
}
