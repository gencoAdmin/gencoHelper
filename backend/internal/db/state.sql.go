// Queries corresponding to queries/state.sql. Regenerate with sqlc generate.

package db

import (
	"context"
	"encoding/json"
)

const getState = `-- name: GetState :one
SELECT payload FROM installation_state WHERE id = 1
`

func (q *Queries) GetState(ctx context.Context) (json.RawMessage, error) {
	var payload json.RawMessage
	err := q.db.QueryRowContext(ctx, getState).Scan(&payload)
	return payload, err
}

const setState = `-- name: SetState :exec
INSERT INTO installation_state(id,payload) VALUES(1,?)
ON DUPLICATE KEY UPDATE payload=VALUES(payload)
`

func (q *Queries) SetState(ctx context.Context, payload json.RawMessage) error {
	_, err := q.db.ExecContext(ctx, setState, payload)
	return err
}
