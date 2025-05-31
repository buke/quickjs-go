package quickjs_test

import (
	"strings"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestErrorBasics tests basic Error functionality and Error() method
func TestErrorBasics(t *testing.T) {
	// Test Error with all fields
	err := quickjs.Error{
		Name:       "TestError",
		Message:    "test message",
		Cause:      "test cause",
		Stack:      "test stack trace",
		JSONString: `{"name":"TestError","message":"test message"}`,
	}

	// Test Error() method format
	require.EqualValues(t, "TestError: test message", err.Error())

	// Test all fields are accessible
	require.EqualValues(t, "TestError", err.Name)
	require.EqualValues(t, "test message", err.Message)
	require.EqualValues(t, "test cause", err.Cause)
	require.EqualValues(t, "test stack trace", err.Stack)
	require.EqualValues(t, `{"name":"TestError","message":"test message"}`, err.JSONString)

	// Test empty Error
	emptyErr := quickjs.Error{}
	require.EqualValues(t, ": ", emptyErr.Error())

	// Test partial fields
	nameOnlyErr := quickjs.Error{Name: "OnlyName"}
	require.EqualValues(t, "OnlyName: ", nameOnlyErr.Error())

	messageOnlyErr := quickjs.Error{Message: "only message"}
	require.EqualValues(t, ": only message", messageOnlyErr.Error())
}

// TestErrorStandardTypes tests Error with standard JavaScript error types
func TestErrorStandardTypes(t *testing.T) {
	testCases := []struct {
		name      string
		errorName string
		message   string
		expected  string
	}{
		{"Error", "Error", "generic error", "Error: generic error"},
		{"TypeError", "TypeError", "type mismatch", "TypeError: type mismatch"},
		{"ReferenceError", "ReferenceError", "variable not defined", "ReferenceError: variable not defined"},
		{"SyntaxError", "SyntaxError", "unexpected token", "SyntaxError: unexpected token"},
		{"RangeError", "RangeError", "value out of range", "RangeError: value out of range"},
		{"InternalError", "InternalError", "internal system error", "InternalError: internal system error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := quickjs.Error{
				Name:    tc.errorName,
				Message: tc.message,
			}
			require.EqualValues(t, tc.expected, err.Error())
		})
	}
}

// TestErrorSpecialCharacters tests Error with special characters and edge cases
func TestErrorSpecialCharacters(t *testing.T) {
	testCases := []struct {
		name        string
		errorName   string
		message     string
		expectedErr string
	}{
		{
			"unicode_characters",
			"UnicodeError",
			"处理中文字符时出错",
			"UnicodeError: 处理中文字符时出错",
		},
		{
			"special_symbols",
			"SymbolError",
			"Error with symbols: @#$%^&*()",
			"SymbolError: Error with symbols: @#$%^&*()",
		},
		{
			"newlines_and_tabs",
			"FormatError",
			"Error with\nnewlines\tand\ttabs",
			"FormatError: Error with\nnewlines\tand\ttabs",
		},
		{
			"quotes",
			"QuoteError",
			`Error with "double" and 'single' quotes`,
			`QuoteError: Error with "double" and 'single' quotes`,
		},
		{
			"colons_only",
			":",
			":",
			":: :",
		},
		{
			"whitespace_only",
			"   ",
			"\t\n  ",
			"   : \t\n  ",
		},
		{
			"control_characters",
			"ControlError",
			"Message with \x00 null byte and \x07 bell",
			"ControlError: Message with \x00 null byte and \x07 bell",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := quickjs.Error{
				Name:    tc.errorName,
				Message: tc.message,
			}
			require.EqualValues(t, tc.expectedErr, err.Error())
		})
	}

	// Test long strings
	longName := strings.Repeat("A", 100)
	longMessage := strings.Repeat("a", 500)

	longErr := quickjs.Error{
		Name:    longName,
		Message: longMessage,
	}

	expectedLongErr := longName + ": " + longMessage
	require.EqualValues(t, expectedLongErr, longErr.Error())
}

// TestErrorAsGoInterface tests Error implementing Go's error interface
func TestErrorAsGoInterface(t *testing.T) {
	quickjsErr := quickjs.Error{
		Name:    "TestError",
		Message: "test error message",
	}

	// Test that it can be used as Go error
	var goErr error = quickjsErr
	require.EqualValues(t, "TestError: test error message", goErr.Error())

	// Test in error handling context
	testFunc := func() error {
		return quickjsErr
	}

	err := testFunc()
	require.Error(t, err)
	require.EqualValues(t, "TestError: test error message", err.Error())

	// Test struct equality
	err1 := quickjs.Error{Name: "TestError", Message: "test message"}
	err2 := quickjs.Error{Name: "TestError", Message: "test message"}
	err3 := quickjs.Error{Name: "DifferentError", Message: "test message"}

	require.Equal(t, err1, err2)
	require.NotEqual(t, err1, err3)
	require.EqualValues(t, err1.Error(), err2.Error())
	require.NotEqual(t, err1.Error(), err3.Error())
}

// TestErrorFieldManipulation tests field access and modification
func TestErrorFieldManipulation(t *testing.T) {
	err := quickjs.Error{
		Name:    "InitialError",
		Message: "initial message",
	}

	// Test initial values
	require.EqualValues(t, "InitialError: initial message", err.Error())

	// Test field modification
	err.Name = "ModifiedError"
	err.Message = "modified message"
	err.Cause = "new cause"
	err.Stack = "new stack trace"
	err.JSONString = `{"modified": true}`

	// Test after modification
	require.EqualValues(t, "ModifiedError: modified message", err.Error())
	require.EqualValues(t, "ModifiedError", err.Name)
	require.EqualValues(t, "modified message", err.Message)
	require.EqualValues(t, "new cause", err.Cause)
	require.EqualValues(t, "new stack trace", err.Stack)
	require.EqualValues(t, `{"modified": true}`, err.JSONString)

	// Test zero value behavior
	var zeroErr quickjs.Error
	require.EqualValues(t, "", zeroErr.Name)
	require.EqualValues(t, "", zeroErr.Message)
	require.EqualValues(t, "", zeroErr.Cause)
	require.EqualValues(t, "", zeroErr.Stack)
	require.EqualValues(t, "", zeroErr.JSONString)
	require.EqualValues(t, ": ", zeroErr.Error())
}
