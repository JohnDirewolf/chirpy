-- name: GetChirpsByUserIDasc :many
SELECT * FROM chirps
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: GetChirpsByUserIDdesc :many
SELECT * FROM chirps
WHERE user_id = $1
ORDER BY created_at DESC;