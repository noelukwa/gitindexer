-- +goose Up
-- +goose StatementBegin

CREATE TYPE intent_status AS ENUM ('pending_broadcast', 'success_broadcast');

CREATE TABLE intents (
    id UUID PRIMARY KEY,
    repository_name VARCHAR(255) NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    status intent_status NOT NULL,
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE intent_errors (
    id UUID PRIMARY KEY,
    intent_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    message TEXT NOT NULL,
    FOREIGN KEY (intent_id) REFERENCES intents(id) ON DELETE CASCADE
);

CREATE INDEX idx_intent_errors_intent_id ON intent_errors(intent_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS intent_errors;
DROP TABLE IF EXISTS intents;
DROP TYPE IF EXISTS intent_status;

-- +goose StatementEnd