#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: scripts/generate-feature.sh <feature_name> [--force] [--with-wiring]" >&2
  exit 1
fi

FEATURE="$1"
FORCE=false
WITH_WIRING=false
for arg in "${@:2}"; do
  case "$arg" in
    --force) FORCE=true ;;
    --with-wiring) WITH_WIRING=true ;;
    *)
      echo "unknown option: $arg" >&2
      exit 1
      ;;
  esac
done

if [[ ! "$FEATURE" =~ ^[a-z][a-z0-9_]*$ ]]; then
  echo "feature_name must be snake_case and start with a letter" >&2
  exit 1
fi

MODULE="$(awk '/^module / {print $2}' go.mod)"
if [[ -z "$MODULE" ]]; then
  echo "unable to read module path from go.mod" >&2
  exit 1
fi

to_pascal() {
  local input="$1"
  local out=""
  local part
  IFS='_' read -ra parts <<< "$input"
  for part in "${parts[@]}"; do
    out+="${part^}"
  done
  printf '%s' "$out"
}

pluralize() {
  local input="$1"
  if [[ "$input" == *s ]]; then
    printf '%s' "$input"
  else
    printf '%ss' "$input"
  fi
}

to_lower_camel() {
  local input="$1"
  local pascal
  pascal="$(to_pascal "$input")"
  printf '%s%s' "$(tr '[:upper:]' '[:lower:]' <<< "${pascal:0:1}")" "${pascal:1}"
}

PASCAL="$(to_pascal "$FEATURE")"
FIELD="$(to_lower_camel "$FEATURE")"
TABLE="$(pluralize "$FEATURE")"
MIGRATION_VERSION="$(date -u +%Y%m%d%H%M%S)"
MIGRATION_NAME="${MIGRATION_VERSION}_create_${TABLE}.sql"

write_file() {
  local path="$1"
  if [[ -e "$path" && "$FORCE" != true ]]; then
    echo "skip existing $path"
    return
  fi
  mkdir -p "$(dirname "$path")"
  cat > "$path"
  echo "write $path"
}

insert_in_block() {
  local file="$1"
  local start="$2"
  local line="$3"

  if grep -Fq "$line" "$file"; then
    return
  fi
  if ! grep -Fq "$start" "$file"; then
    echo "unable to find block '$start' in $file" >&2
    exit 1
  fi

  awk -v start="$start" -v line="$line" '
    $0 == start {
      in_block = 1
    }
    in_block && $0 == "}" && !done {
      print line
      done = 1
      in_block = 0
    }
    { print }
  ' "$file" > "$file.tmp"
  mv "$file.tmp" "$file"
}

insert_after_line() {
  local file="$1"
  local marker="$2"
  local line="$3"

  if grep -Fq "$line" "$file"; then
    return
  fi
  if ! grep -Fq "$marker" "$file"; then
    echo "unable to find marker '$marker' in $file" >&2
    exit 1
  fi

  awk -v marker="$marker" -v line="$line" '
    { print }
    $0 == marker && !done {
      print line
      done = 1
    }
  ' "$file" > "$file.tmp"
  mv "$file.tmp" "$file"
}

write_file "models/${FEATURE}_inputs.go" <<EOF
package models

type ${PASCAL}CreateInput struct {
	Name string \`json:"name" validate:"required,max=120"\`
}
EOF

write_file "models/${FEATURE}_outputs.go" <<EOF
package models

type ${PASCAL}Response struct {
	ID   uint   \`json:"id"\`
	Name string \`json:"name"\`
}
EOF

write_file "models/${FEATURE}_models.go" <<EOF
package models

import "gorm.io/gorm"

type ${PASCAL} struct {
	gorm.Model
	Name string \`gorm:"size:120;not null"\`
}

func (${PASCAL}) TableName() string {
	return "${TABLE}"
}
EOF

write_file "models/${FEATURE}_mappers.go" <<EOF
package models

func Map${PASCAL}Response(row *${PASCAL}) *${PASCAL}Response {
	if row == nil {
		return nil
	}
	return &${PASCAL}Response{
		ID:   row.ID,
		Name: row.Name,
	}
}
EOF

write_file "store/${FEATURE}Store.go" <<EOF
package store

import (
	"context"

	"${MODULE}/models"
	"gorm.io/gorm"
)

type ${PASCAL}Store struct {
	db *gorm.DB
}

func New${PASCAL}Store(db *gorm.DB) *${PASCAL}Store {
	return &${PASCAL}Store{db: db}
}

func (s *${PASCAL}Store) Create(ctx context.Context, row *models.${PASCAL}) error {
	return s.db.WithContext(ctx).Create(row).Error
}

func (s *${PASCAL}Store) GetByID(ctx context.Context, id uint) (*models.${PASCAL}, error) {
	var row models.${PASCAL}
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
EOF

write_file "services/${FEATURE}Service.go" <<EOF
package services

import (
	"context"

	"${MODULE}/models"
	"${MODULE}/store"
	"go.uber.org/zap"
)

type ${PASCAL}Service struct {
	Store  store.Datastore
	logger *zap.Logger
}

func New${PASCAL}Service(store store.Datastore, logger *zap.Logger) *${PASCAL}Service {
	return &${PASCAL}Service{
		Store:  store,
		logger: logger,
	}
}

func (s *${PASCAL}Service) Create(ctx context.Context, in models.${PASCAL}CreateInput) (*models.${PASCAL}Response, error) {
	row := &models.${PASCAL}{Name: in.Name}
	if err := s.Store.${PASCAL}().Create(ctx, row); err != nil {
		return nil, err
	}
	return models.Map${PASCAL}Response(row), nil
}
EOF

write_file "controllers/${FEATURE}Controller.go" <<EOF
package controllers

import (
	"net/http"

	"${MODULE}/internal/apierr"
	"${MODULE}/models"
	"${MODULE}/services"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type ${PASCAL}Controller struct {
	appService *services.AppService
	logger     *zap.Logger
}

func New${PASCAL}Controller(appService *services.AppService, logger *zap.Logger) *${PASCAL}Controller {
	return &${PASCAL}Controller{
		appService: appService,
		logger:     logger,
	}
}

func (c *${PASCAL}Controller) Create(ec echo.Context) error {
	var in models.${PASCAL}CreateInput
	if err := ec.Bind(&in); err != nil {
		return apierr.ReplyBadRequest(ec, "invalid payload")
	}
	if err := ec.Validate(&in); err != nil {
		return apierr.ReplyValidation(ec, err)
	}

	out, err := c.appService.${PASCAL}.Create(ec.Request().Context(), in)
	if err != nil {
		c.logger.Error("${FEATURE} create failed", zap.Error(err))
		return apierr.Reply(ec, err)
	}

	return ec.JSON(http.StatusCreated, out)
}
EOF

write_file "internal/migrations/mysql/${MIGRATION_NAME}" <<EOF
-- +goose Up
CREATE TABLE IF NOT EXISTS ${TABLE} (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(120) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_${TABLE}_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS ${TABLE};
EOF

write_file "internal/migrations/postgres/${MIGRATION_NAME}" <<EOF
-- +goose Up
CREATE TABLE IF NOT EXISTS ${TABLE} (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NULL,
  updated_at TIMESTAMPTZ NULL,
  deleted_at TIMESTAMPTZ NULL,
  name VARCHAR(120) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_${TABLE}_deleted_at ON ${TABLE} (deleted_at);

-- +goose Down
DROP TABLE IF EXISTS ${TABLE};
EOF

if [[ "$WITH_WIRING" == true ]]; then
  insert_in_block "store/appStore.go" "type Datastore interface {" "	${PASCAL}() *${PASCAL}Store"
  insert_in_block "store/appStore.go" "type AppStore struct {" "	${FIELD} *${PASCAL}Store"
  insert_after_line "store/appStore.go" "		hello:  NewHelloStore(db)," "		${FIELD}: New${PASCAL}Store(db),"
  if ! grep -Fq "func (s *AppStore) ${PASCAL}() *${PASCAL}Store" "store/appStore.go"; then
    cat >> "store/appStore.go" <<EOF

func (s *AppStore) ${PASCAL}() *${PASCAL}Store {
	return s.${FIELD}
}
EOF
  fi

  insert_in_block "services/appService.go" "type AppService struct {" "	${PASCAL} *${PASCAL}Service"
  insert_after_line "services/appService.go" "		Hello:  NewHelloService(store, logger)," "		${PASCAL}: New${PASCAL}Service(store, logger),"
fi

gofmt -w \
  "models/${FEATURE}_inputs.go" \
  "models/${FEATURE}_outputs.go" \
  "models/${FEATURE}_models.go" \
  "models/${FEATURE}_mappers.go" \
  "store/${FEATURE}Store.go" \
  "services/${FEATURE}Service.go" \
  "controllers/${FEATURE}Controller.go"

if [[ "$WITH_WIRING" == true ]]; then
  gofmt -w store/appStore.go services/appService.go
fi

echo
if [[ "$WITH_WIRING" == true ]]; then
  echo "Wiring completed for store.AppStore and services.AppService."
  echo "Next manual steps:"
  echo "1. Register New${PASCAL}Controller and routes in main.go."
  echo "2. Review generated goose migrations."
else
  echo "Next manual wiring:"
  echo "1. Add ${PASCAL}() *${PASCAL}Store to store.Datastore and AppStore or rerun with --with-wiring."
  echo "2. Add ${PASCAL} *${PASCAL}Service to services.AppService or rerun with --with-wiring."
  echo "3. Register New${PASCAL}Controller and routes in main.go."
  echo "4. Review generated goose migrations."
fi
