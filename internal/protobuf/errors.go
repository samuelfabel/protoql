package protobuf

import "fmt"

const CodeDescriptorBuildFailed = "DESCRIPTOR_BUILD_FAILED"

// Error is a protobuf package failure with a stable error code.
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

func descriptorBuildFailed(msg string) error {
	return &Error{Code: CodeDescriptorBuildFailed, Message: msg}
}
