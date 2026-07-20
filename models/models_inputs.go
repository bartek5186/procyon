package models

// Request DTOs describe data accepted by the API. Validation tags are checked
// by Echo before the input reaches the service. A basic create input looks like:
//
//	type InvoiceCreateInput struct {
//		Name string `json:"name" validate:"required,max=120"`
//	}
//
// Keep transport validation here and business validation in the service or
// domain logic. Feature-specific DTOs should use names such as invoice_inputs.go.
