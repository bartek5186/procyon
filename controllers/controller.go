// Package controllers contains application-owned HTTP handlers. Controllers
// should translate HTTP input and output while delegating business logic to
// services; keep each feature in a descriptive file such as invoice_controller.go.
//
// A basic controller has one service dependency, a constructor and small Echo
// handlers. Business logic belongs in the service:
//
//	type InvoiceService interface {
//		List(context.Context) ([]models.InvoiceResponse, error)
//	}
//
//	type InvoiceController struct {
//		service InvoiceService
//	}
//
//	func NewInvoiceController(service InvoiceService) *InvoiceController {
//		return &InvoiceController{service: service}
//	}
//
//	func (c *InvoiceController) List(ec echo.Context) error {
//		invoices, err := c.service.List(ec.Request().Context())
//		if err != nil {
//			return apierr.Reply(ec, err)
//		}
//		return ec.JSON(http.StatusOK, invoices)
//	}
//
// Register the handler in routes.go. For a complete generated implementation,
// run: procyon-cli module create invoice.
package controllers
