package model

// Error codes returned in the "code" field of an error response.
const (
	CodeInvalidID           = "invalid_id"
	CodeInvalidPagination   = "invalid_pagination"
	CodeValidationError     = "validation_error"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeInvalidReference    = "invalid_reference"
	CodeInternalServerError = "internal_server_error"
	// Коди автентифікації та авторизації.
	CodeUnauthenticated = "unauthenticated" // 401: немає або недійсний токен
	CodeForbidden       = "forbidden"       // 403: роль не має доступу
)

// ErrorResponse is the JSON error envelope:
// {"error": {"code": "not_found", "message": "Book not found"}}.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries a machine-readable code and a human-readable message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorResponse builds an ErrorResponse from a code and a message.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Error: ErrorBody{Code: code, Message: message}}
}
