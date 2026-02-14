-- name: CreatePost :one
INSERT INTO posts (id, authorid, title, slug, content, excerpt, status, publishedat, viewcount, createdat, updatedat) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING *;

-- name: GetPost :one
SELECT * FROM posts WHERE id = $1 LIMIT 1;

-- name: ListPost :many
SELECT * FROM posts ORDER BY id LIMIT $1 OFFSET $2;

-- name: CreateComment :one
INSERT INTO comments (id, postid, authorid, parentid, content, createdat, updatedat) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetComment :one
SELECT * FROM comments WHERE id = $1 LIMIT 1;

-- name: ListComment :many
SELECT * FROM comments ORDER BY id LIMIT $1 OFFSET $2;

-- name: CreateTag :one
INSERT INTO tags (id, name, slug, description) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetTag :one
SELECT * FROM tags WHERE id = $1 LIMIT 1;

-- name: ListTag :many
SELECT * FROM tags ORDER BY id LIMIT $1 OFFSET $2;

-- name: CreateUserVault :one
INSERT INTO uservaults (id, personalkey, bio) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUserVault :one
SELECT * FROM uservaults WHERE id = $1 LIMIT 1;

-- name: ListUserVault :many
SELECT * FROM uservaults ORDER BY id LIMIT $1 OFFSET $2;

