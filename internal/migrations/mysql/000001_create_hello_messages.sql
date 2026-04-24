CREATE TABLE IF NOT EXISTS hello_messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  slug VARCHAR(64) NOT NULL,
  lang VARCHAR(16) NOT NULL,
  message VARCHAR(255) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY ux_hello_slug_lang (slug, lang),
  KEY idx_hello_messages_deleted_at (deleted_at),
  KEY idx_hello_messages_lang (lang)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
