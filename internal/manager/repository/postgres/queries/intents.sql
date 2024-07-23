-- SaveIntent.sql
-- name: SaveIntent :one
INSERT INTO intents (
    id, repository_name, start_date, status, is_active
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING id, repository_name, start_date, status, is_active, created_at, updated_at;

-- UpdateIntent.sql
-- name: UpdateIntent :one
UPDATE intents
SET 
    status = COALESCE($2, status),
    is_active = COALESCE($3, is_active),
    start_date = COALESCE($4, start_date),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, repository_name, start_date, status, is_active, created_at, updated_at;

-- SaveIntentError.sql
-- name: SaveIntentError :exec
INSERT INTO intent_errors (
    id, intent_id, created_at, message
) VALUES (
    $1, $2, $3, $4
);


-- name: FindIntents :many
SELECT 
    id, repository_name, start_date, status, is_active, created_at, updated_at
FROM 
    intents
WHERE 
    (NOT @status_provided OR status = @status::intent_status)
    AND (NOT @is_active_provided OR is_active = @is_active)
    AND (NOT @repository_name_provided OR repository_name = @repository_name)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountIntents :one
SELECT COUNT(*)
FROM intents
WHERE 
    (NOT @status_provided OR status = @status::intent_status)
    AND (NOT @is_active_provided OR is_active = @is_active)
    AND (NOT @repository_name_provided OR repository_name = @repository_name);
    
-- FindIntent.sql
-- name: FindIntent :one
SELECT 
    id, repository_name, start_date, status, is_active, created_at, updated_at
FROM 
    intents
WHERE 
    id = $1;
