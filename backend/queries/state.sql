-- name: GetState :one
SELECT payload FROM installation_state WHERE id = 1;

-- name: SetState :exec
INSERT INTO installation_state(id, payload) VALUES (1, ?)
ON DUPLICATE KEY UPDATE payload = VALUES(payload);
