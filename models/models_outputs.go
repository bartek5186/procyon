package models

// Response DTOs define the stable JSON shape returned by the API. A basic
// response looks like:
//
//	type InvoiceResponse struct {
//		ID   uint   `json:"id"`
//		Name string `json:"name"`
//	}
//
// Map persistence models to response DTOs explicitly so database-only fields
// are not exposed accidentally. Feature-specific DTOs should use names such as
// invoice_outputs.go.
