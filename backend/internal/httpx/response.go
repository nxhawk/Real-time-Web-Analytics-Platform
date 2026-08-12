package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error codes returned to clients. The set is closed on purpose: a client can switch on it.
// See PLAN.md 12.4.
const (
	CodeInvalidJSON         = "invalid_json"
	CodeValidationFailed    = "validation_failed"
	CodeUnauthorized        = "unauthorized"
	CodeRateLimited         = "rate_limited"
	CodeInvalidRange        = "invalid_range"
	CodeRangeTooLarge       = "range_too_large"
	CodeUpstreamUnavailable = "upstream_unavailable"
	CodeNotFound            = "not_found"
	CodePayloadTooLarge     = "payload_too_large"
	CodeInternal            = "internal"
)

// ErrorBody is the machine-readable part of an error response.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ErrorResponse is the single error envelope every endpoint returns.
//
//	{"error":{"code":"invalid_range","message":"from must be before to","details":{}},
//	 "request_id":"01J..."}
type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

// AbortWithError writes the error envelope and stops the handler chain.
func AbortWithError(c *gin.Context, status int, code, message string) {
	AbortWithErrorDetails(c, status, code, message, nil)
}

// AbortWithErrorDetails is AbortWithError with a structured details object.
func AbortWithErrorDetails(c *gin.Context, status int, code, message string, details map[string]any) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error:     ErrorBody{Code: code, Message: message, Details: details},
		RequestID: RequestIDFrom(c),
	})
}

// NotFoundHandler answers unknown routes with the standard envelope instead of gin's
// plain-text 404.
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		AbortWithError(c, http.StatusNotFound, CodeNotFound, "route not found")
	}
}

// MethodNotAllowedHandler answers a known path with an unsupported method.
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		AbortWithError(c, http.StatusMethodNotAllowed, CodeNotFound, "method not allowed")
	}
}
