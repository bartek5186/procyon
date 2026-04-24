package apierr

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Error struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Err     error          `json:"-"`
}

type Response struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(status int, code, message string) *Error {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	code = strings.TrimSpace(code)
	if code == "" {
		code = "internal_error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = http.StatusText(status)
	}
	return &Error{Status: status, Code: code, Message: message}
}

func Wrap(err error, status int, code, message string) *Error {
	apiErr := New(status, code, message)
	apiErr.Err = err
	return apiErr
}

func WithDetails(err *Error, details map[string]any) *Error {
	if err == nil {
		return nil
	}
	err.Details = details
	return err
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "forbidden", message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "not_found", message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "conflict", message)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, "internal_error", message)
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}

	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFound("resource not found")
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		message := http.StatusText(echoErr.Code)
		if s, ok := echoErr.Message.(string); ok && strings.TrimSpace(s) != "" {
			message = s
		}
		return New(echoErr.Code, statusCodeName(echoErr.Code), message)
	}

	return Wrap(err, http.StatusInternalServerError, "internal_error", "internal server error")
}

func JSON(c echo.Context, err error) error {
	apiErr := FromError(err)
	if apiErr == nil {
		apiErr = Internal("internal server error")
	}

	requestID := requestID(c)
	return c.JSON(apiErr.Status, Response{
		Error: ErrorBody{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Details: apiErr.Details,
		},
		RequestID: requestID,
	})
}

func Reply(c echo.Context, err error) error {
	return JSON(c, err)
}

func ReplyBadRequest(c echo.Context, message string) error {
	return JSON(c, BadRequest(message))
}

func ReplyValidation(c echo.Context, err error) error {
	return JSON(c, Wrap(err, http.StatusBadRequest, "validation_failed", "validation failed"))
}

func ReplyUnauthorized(c echo.Context, message string) error {
	return JSON(c, Unauthorized(message))
}

func ReplyForbidden(c echo.Context, message string) error {
	return JSON(c, Forbidden(message))
}

func ReplyInternal(c echo.Context, message string, err error) error {
	if err == nil {
		return JSON(c, Internal(message))
	}
	return JSON(c, Wrap(err, http.StatusInternalServerError, "internal_error", message))
}

func ReplyInternalCode(c echo.Context, code, message string, err error) error {
	if err == nil {
		return JSON(c, New(http.StatusInternalServerError, code, message))
	}
	return JSON(c, Wrap(err, http.StatusInternalServerError, code, message))
}

func Handler(logger *zap.Logger) echo.HTTPErrorHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		apiErr := FromError(err)
		if apiErr.Status >= http.StatusInternalServerError {
			logger.Error("request failed", zap.Error(err), zap.String("request_id", requestID(c)))
		}

		if jsonErr := JSON(c, apiErr); jsonErr != nil {
			logger.Error("failed to write error response", zap.Error(jsonErr), zap.String("request_id", requestID(c)))
		}
	}
}

func requestID(c echo.Context) string {
	if c == nil {
		return ""
	}
	if rid := strings.TrimSpace(c.Response().Header().Get(echo.HeaderXRequestID)); rid != "" {
		return rid
	}
	if rid := strings.TrimSpace(c.Request().Header.Get(echo.HeaderXRequestID)); rid != "" {
		return rid
	}
	return ""
}

func statusCodeName(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "http_error"
	}
}
