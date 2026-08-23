package compile

import "fmt"

const (
	CodeQueryInvalid      = "QUERY_INVALID"
	CodeQueryFieldUnknown = "QUERY_FIELD_UNKNOWN"
)

// Error is a compile failure with a stable error code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
