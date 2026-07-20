// Package models contains application-owned database entities and API types.
// Keep feature-specific persistence models in descriptive files such as
// invoice_models.go.
//
// A basic GORM model looks like this:
//
//	type Invoice struct {
//		gorm.Model
//		Name string `gorm:"size:120;not null"`
//	}
//
// Keep request DTOs in *_inputs.go and response DTOs in *_outputs.go. Do not
// expose a persistence model directly when the API needs a stable shape.
package models
