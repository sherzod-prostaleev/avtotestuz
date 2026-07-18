-- name: InsertEvent :exec
INSERT INTO event (profile_id, name, props, ts)
VALUES ($1, $2, $3, $4);
