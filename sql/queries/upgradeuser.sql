-- name: UpgradeUser :execresult
UPDATE users
SET updated_at = NOW(), is_chirpy_red = TRUE  
WHERE id = $1;