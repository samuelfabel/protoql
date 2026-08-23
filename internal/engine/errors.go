package engine

import "fmt"

const (
	CodeEngineBindingUnsupported = "ENGINE_BINDING_UNSUPPORTED"
	CodeTypeUnsupported          = "TYPE_UNSUPPORTED"
	CodeEngineNullViolation      = "ENGINE_NULL_VIOLATION"
)

// Error is an engine failure with a stable error code.
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

func bindingUnsupported(msg string) error {
	return &Error{Code: CodeEngineBindingUnsupported, Message: msg}
}

func typeUnsupported(msg string) error {
	return &Error{Code: CodeTypeUnsupported, Message: msg}
}

func nullViolation(msg string) error {
	return &Error{Code: CodeEngineNullViolation, Message: msg}
}
