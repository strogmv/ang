-- name: CreateUserVault :one
INSERT INTO uservaults (id, personalkey, bio) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUserVault :one
SELECT * FROM uservaults WHERE id = $1 LIMIT 1;

-- name: ListUserVault :many
SELECT * FROM uservaults ORDER BY id LIMIT $1 OFFSET $2;

