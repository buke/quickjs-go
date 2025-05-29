package quickjs

import "fmt"

// Error represents a JavaScript error with detailed information.
type Error struct {
	Name       string // Error name (e.g., "TypeError", "ReferenceError")
	Message    string // Error message
	Cause      string // Error cause
	Stack      string // Stack trace
	JSONString string // Serialized JSON string
}

// Error implements the error interface.
func (err Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Name, err.Message)
}
