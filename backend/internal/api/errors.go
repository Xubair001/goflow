package api

import "github.com/gin-gonic/gin"

// errorResponse is the JSON envelope every error response uses, so API
// clients parse one shape regardless of endpoint or status code.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes are part of the API contract: clients branch on Code, not on
// the human-readable Message, which may change wording over time.
const (
	codeInvalidRequest = "invalid_request"
	codeNotFound       = "not_found"
	codeInternal       = "internal_error"
)

// writeError writes the standard error envelope. It doesn't call c.Abort():
// every handler returns immediately after calling this, and there's no
// downstream handler chain here for Abort to short-circuit -- see
// middleware.go's recovery, which does call Abort for the one case
// (a panic) where a later middleware could otherwise still run.
func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}
