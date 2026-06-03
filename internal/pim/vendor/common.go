package pimvendor

import (
	"fmt"
	"strings"
)

// Error mirrors the upstream pkg/common.Error type. Absorbed locally so we
// don't import the upstream module.
type Error struct {
	Operation string
	Message   string
	Status    string
	Err       error
	Response  interface{}
	Request   interface{}
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() string {
	if e.Status == "" {
		return fmt.Sprintf("%s: %s", e.Operation, e.Message)
	}
	return fmt.Sprintf("%s failed with status %s: %s", e.Operation, e.Status, e.Message)
}

func (e *Error) Debug() string {
	var debugLines []string
	if e.Request != nil {
		debugLines = append(debugLines, fmt.Sprintf("Request:\n%v", e.Request))
	}
	if e.Response != nil {
		debugLines = append(debugLines, fmt.Sprintf("Response:\n%v", e.Response))
	}
	if e.Err != nil {
		debugLines = append(debugLines, fmt.Sprintf("Error:\n%v", e.Err.Error()))
	}
	return strings.Join(debugLines, "\n")
}
