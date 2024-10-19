-- name: GetAllChirpsAsc :many
SELECT * FROM chirps
ORDER BY created_at ASC;

-- name: GetAllChirpsDesc :many
SELECT * FROM chirps
ORDER BY created_at DESC;