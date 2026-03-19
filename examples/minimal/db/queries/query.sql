-- name: CreateUser :one
INSERT INTO users (id, email, name) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListUser :many
SELECT * FROM users ORDER BY id LIMIT $1 OFFSET $2;

