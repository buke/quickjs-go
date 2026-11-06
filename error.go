package quickjs

import "fmt"

// Error represents a JavaScript error with detailed information.
type Error struct {
	Name         string // Error name (e.g., "TypeError", "ReferenceError")
	Message      string // Error message
	Cause        string // Error cause
	Stack        string // Stack trace
	JSONString   string // Serialized JSON string
	Uncatchable  bool   // true when JS_IsUncatchableError (e.g. OOM / InternalError)
}

// Error implements the error interface.
func (err *Error) Error() string {
	msg := fmt.Sprintf("%s: %s", err.Name, err.Message)
	if err.Cause != "" {
		return fmt.Sprintf("%s (cause: %s)", msg, err.Cause)
	}
	return msg
}
