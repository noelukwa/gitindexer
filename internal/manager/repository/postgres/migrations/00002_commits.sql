-- +goose Up
-- +goose StatementBegin
CREATE TABLE repositories (
    id BIGSERIAL PRIMARY KEY,
    watchers INT NOT NULL,
    stargazers INT NOT NULL,
    full_name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    language TEXT,
    forks INT NOT NULL
);

CREATE TABLE authors (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    username TEXT NOT NULL
);

CREATE TABLE commits (
    hash TEXT PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES authors(id),
    message TEXT NOT NULL,
    url TEXT DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    repository_id BIGINT NOT NULL REFERENCES repositories(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE commits;
DROP TABLE authors;
DROP TABLE repositories;
-- +goose StatementEnd
