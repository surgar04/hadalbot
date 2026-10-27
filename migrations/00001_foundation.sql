-- +goose Up
-- 新 Go 工程自己的版本标记；不创建、覆盖或迁移旧项目业务表。
CREATE TABLE hadalbot_schema_metadata (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO hadalbot_schema_metadata (key, value)
VALUES ('foundation', '{"version":1}'::jsonb);

-- +goose Down
DROP TABLE hadalbot_schema_metadata;
