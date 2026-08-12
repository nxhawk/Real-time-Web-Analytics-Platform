// Package httpx holds transport-level building blocks shared by every HTTP service:
// middleware, the error envelope, and a server with graceful shutdown.
//
// Nothing in here knows about analytics. Business rules live in internal/service.
package httpx

import "github.com/gin-gonic/gin"

// Keys used to carry request-scoped values through gin's context. They are strings rather
// than a custom type because gin's Set/Get API takes strings.
const (
	ContextKeyRequestID = "request_id"
	ContextKeySiteID    = "site_id"
)

// HeaderRequestID is the inbound and outbound correlation header.
const HeaderRequestID = "X-Request-ID"

// RequestIDFrom returns the request id attached by the RequestID middleware, or "".
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(ContextKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SiteIDFrom returns the site id resolved from the API key, or "".
// The API key middleware that sets it arrives in Level 1 (task L1-19).
func SiteIDFrom(c *gin.Context) string {
	if v, ok := c.Get(ContextKeySiteID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
