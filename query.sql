-- name: InsertEvent :exec
INSERT INTO events (payload, source)
VALUES ($1, $2);