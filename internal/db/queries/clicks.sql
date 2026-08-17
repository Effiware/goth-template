-- name: GetClickCount :one
SELECT count FROM clicks WHERE name = sqlc.arg(name);

-- name: IncrementClickCount :one
-- Atomic read-modify-write: the returned value is this caller's own increment.
UPDATE clicks SET count = count + 1
WHERE name = sqlc.arg(name)
RETURNING count;
