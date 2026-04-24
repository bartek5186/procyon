-- +goose Up
CREATE TABLE IF NOT EXISTS hello_messages (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,
  slug VARCHAR(64) NOT NULL,
  lang VARCHAR(16) NOT NULL,
  message VARCHAR(255) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_hello_slug_lang ON hello_messages (slug, lang);
CREATE INDEX IF NOT EXISTS idx_hello_messages_deleted_at ON hello_messages (deleted_at);
CREATE INDEX IF NOT EXISTS idx_hello_messages_lang ON hello_messages (lang);

-- +goose Down
DROP TABLE IF EXISTS hello_messages;
