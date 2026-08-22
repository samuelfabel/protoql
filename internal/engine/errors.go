package engine

import "fmt"

const CodeEngineBindingUnsupported = "ENGINE_BINDING_UNSUPPORTED"

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

func unsupported(msg string) error {
	return &Error{Code: CodeEngineBindingUnsupported, Message: msg}
}
