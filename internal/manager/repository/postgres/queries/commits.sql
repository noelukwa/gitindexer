-- name: SaveRepo :exec
INSERT INTO repositories (id, watchers, stargazers, full_name, created_at, updated_at, language, forks)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (full_name) DO UPDATE SET
    watchers = EXCLUDED.watchers,
    stargazers = EXCLUDED.stargazers,
    updated_at = EXCLUDED.updated_at,
    language = EXCLUDED.language,
    forks = EXCLUDED.forks;

-- name: GetRepo :one
SELECT * FROM repositories
WHERE full_name = $1;

-- name: GetAuthor :one
SELECT * FROM authors
WHERE id = $1;

-- name: SaveAuthor :one
INSERT INTO authors (id, name, email, username)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    username = EXCLUDED.username
RETURNING *;

-- name: SaveCommit :exec
INSERT INTO commits (hash, author_id, message, url, created_at, repository_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (hash) DO NOTHING;


-- name: FindCommits :many
SELECT 
    c.hash, c.message, c.url, c.created_at,
    a.id AS author_id, a.name AS author_name, a.email AS author_email, a.username AS author_username,
    r.id AS repo_id, r.watchers, r.stargazers, r.full_name AS repository, r.created_at AS repo_created_at, 
    r.updated_at AS repo_updated_at, r.language, r.forks
FROM commits c
JOIN repositories r ON c.repository_id = r.id
JOIN authors a ON c.author_id = a.id
WHERE r.full_name = $1
    AND ($2::timestamptz IS NULL OR c.created_at >= $2)
    AND ($3::timestamptz IS NULL OR c.created_at <= $3)
    AND ($4::text IS NULL OR a.username = $4)
ORDER BY c.created_at DESC
LIMIT $5 OFFSET $6;


-- name: CountCommits :one
SELECT COUNT(*)
FROM commits c
JOIN repositories r ON c.repository_id = r.id
JOIN authors a ON c.author_id = a.id
WHERE r.full_name = $1
    AND ($2::timestamptz IS NULL OR c.created_at >= $2)
    AND ($3::timestamptz IS NULL OR c.created_at <= $3)
    AND ($4::text IS NULL OR a.username = $4);

-- name: GetTopCommitters :many
SELECT a.id, a.name, a.email, a.username, COUNT(c.hash) as commit_count
FROM authors a
JOIN commits c ON a.id = c.author_id
JOIN repositories r ON c.repository_id = r.id
WHERE r.full_name = $1
    AND ($2::timestamptz IS NULL OR c.created_at >= $2)
    AND ($3::timestamptz IS NULL OR c.created_at <= $3)
GROUP BY a.id, a.name, a.email, a.username
ORDER BY commit_count DESC
LIMIT $4 OFFSET $5;

-- name: SaveManyCommits :many
INSERT INTO commits (hash, author_id, message, url, created_at, repository_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (hash) DO NOTHING
RETURNING *;

