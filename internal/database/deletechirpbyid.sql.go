// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: deletechirpbyid.sql

package database

import (
	"context"

	"github.com/google/uuid"
)

const deleteChirpByID = `-- name: DeleteChirpByID :exec
DELETE FROM chirps
WHERE id = $1
`

func (q *Queries) DeleteChirpByID(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, deleteChirpByID, id)
	return err
}
