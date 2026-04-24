#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: scripts/generate-feature.sh <feature_name> [--force]" >&2
  exit 1
fi

FEATURE="$1"
FORCE=false
if [[ "${2:-}" == "--force" ]]; then
  FORCE=true
fi

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

PASCAL="$(to_pascal "$FEATURE")"
LOWER_CAMEL="$(printf '%s%s' "$(tr '[:upper:]' '[:lower:]' <<< "${PASCAL:0:1}")" "${PASCAL:1}")"

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
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	if err := ec.Validate(&in); err != nil {
		return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	out, err := c.appService.${PASCAL}.Create(ec.Request().Context(), in)
	if err != nil {
		c.logger.Error("${FEATURE} create failed", zap.Error(err))
		return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return ec.JSON(http.StatusCreated, out)
}
EOF

echo
echo "Next manual wiring:"
echo "1. Add ${PASCAL}() *${PASCAL}Store to store.Datastore and AppStore."
echo "2. Add ${PASCAL} *${PASCAL}Service to services.AppService."
echo "3. Register New${PASCAL}Controller and routes in main.go."
echo "4. Add models.${PASCAL} to migrations."
