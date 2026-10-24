package persistence

import (
	"context"
	"database/sql"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// withImmediateTx runs fn in a BEGIN IMMEDIATE transaction on one dedicated connection of db. IMMEDIATE takes
// the write lock up front, so a read-then-write sequence cannot fail with a lock upgrade error.
// Transaction control statements are the only raw SQL here, and they carry no parameters.
func withImmediateTx(ctx context.Context, db *sql.DB, fn func(queries *gen.Queries) error) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return dbError(err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil && err == nil {
			err = dbError(closeErr)
		}
	}()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return dbError(err)
	}
	committed := false
	defer func() {
		if !committed {
			// The rollback runs even when ctx is cancelled; its failure cannot improve the returned error.
			_, rollbackErr := conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			_ = rollbackErr
		}
	}()
	if err := fn(gen.New(conn)); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return dbError(err)
	}
	committed = true
	return nil
}
