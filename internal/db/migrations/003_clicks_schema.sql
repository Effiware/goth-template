-- +goose Up

-- Demo table behind the click counter — the state that keeps the app itself
-- stateless. Delete it (and internal/db/queries/clicks.sql) when forking.
CREATE TABLE clicks (
    name  TEXT PRIMARY KEY,
    count BIGINT NOT NULL DEFAULT 0
);
INSERT INTO clicks (name, count) VALUES ('global', 0);

-- +goose Down
DROP TABLE clicks;
