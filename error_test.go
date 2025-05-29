package quickjs_test

import (
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestErrorBasic tests basic Error functionality.
func TestErrorBasic(t *testing.T) {
	// Test creating an Error with all fields
	err := quickjs.Error{
		Name:       "TestError",
		Message:    "test message",
		Cause:      "test cause",
		Stack:      "test stack trace",
		JSONString: `{"name":"TestError","message":"test message"}`,
	}

	// Test Error() method
	require.EqualValues(t, "TestError: test message", err.Error())

	// Test all fields are accessible
	require.EqualValues(t, "TestError", err.Name)
	require.EqualValues(t, "test message", err.Message)
	require.EqualValues(t, "test cause", err.Cause)
	require.EqualValues(t, "test stack trace", err.Stack)
	require.EqualValues(t, `{"name":"TestError","message":"test message"}`, err.JSONString)
}

// TestErrorEmpty tests Error with empty fields.
func TestErrorEmpty(t *testing.T) {
	// Test with empty Error
	emptyErr := quickjs.Error{}
	require.EqualValues(t, ": ", emptyErr.Error())

	// Test with only Name
	nameOnlyErr := quickjs.Error{Name: "OnlyName"}
	require.EqualValues(t, "OnlyName: ", nameOnlyErr.Error())

	// Test with only Message
	messageOnlyErr := quickjs.Error{Message: "only message"}
	require.EqualValues(t, ": only message", messageOnlyErr.Error())
}

// TestErrorStandardTypes tests Error with standard JavaScript error types.
func TestErrorStandardTypes(t *testing.T) {
	testCases := []struct {
		name     string
		errorObj quickjs.Error
		expected string
	}{
		{
			name: "Error",
			errorObj: quickjs.Error{
				Name:    "Error",
				Message: "generic error",
			},
			expected: "Error: generic error",
		},
		{
			name: "TypeError",
			errorObj: quickjs.Error{
				Name:    "TypeError",
				Message: "type mismatch",
			},
			expected: "TypeError: type mismatch",
		},
		{
			name: "ReferenceError",
			errorObj: quickjs.Error{
				Name:    "ReferenceError",
				Message: "variable not defined",
			},
			expected: "ReferenceError: variable not defined",
		},
		{
			name: "SyntaxError",
			errorObj: quickjs.Error{
				Name:    "SyntaxError",
				Message: "unexpected token",
			},
			expected: "SyntaxError: unexpected token",
		},
		{
			name: "RangeError",
			errorObj: quickjs.Error{
				Name:    "RangeError",
				Message: "value out of range",
			},
			expected: "RangeError: value out of range",
		},
		{
			name: "InternalError",
			errorObj: quickjs.Error{
				Name:    "InternalError",
				Message: "internal system error",
			},
			expected: "InternalError: internal system error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.EqualValues(t, tc.expected, tc.errorObj.Error())
		})
	}
}

// TestErrorWithCause tests Error with cause information.
func TestErrorWithCause(t *testing.T) {
	err := quickjs.Error{
		Name:    "CustomError",
		Message: "something went wrong",
		Cause:   "underlying database connection failed",
	}

	require.EqualValues(t, "CustomError: something went wrong", err.Error())
	require.EqualValues(t, "underlying database connection failed", err.Cause)
}

// TestErrorWithStack tests Error with stack trace.
func TestErrorWithStack(t *testing.T) {
	stackTrace := `at testFunction (test.js:10:5)
at main (test.js:15:3)
at Object.<anonymous> (test.js:20:1)`

	err := quickjs.Error{
		Name:    "RuntimeError",
		Message: "execution failed",
		Stack:   stackTrace,
	}

	require.EqualValues(t, "RuntimeError: execution failed", err.Error())
	require.EqualValues(t, stackTrace, err.Stack)
}

// TestErrorWithJSONString tests Error with JSON representation.
func TestErrorWithJSONString(t *testing.T) {
	jsonStr := `{"name":"ParseError","message":"invalid JSON format","line":5,"column":12}`

	err := quickjs.Error{
		Name:       "ParseError",
		Message:    "invalid JSON format",
		JSONString: jsonStr,
	}

	require.EqualValues(t, "ParseError: invalid JSON format", err.Error())
	require.EqualValues(t, jsonStr, err.JSONString)
}

// TestErrorWithAllFields tests Error with all fields populated.
func TestErrorWithAllFields(t *testing.T) {
	err := quickjs.Error{
		Name:       "ComplexError",
		Message:    "complex error occurred",
		Cause:      "network timeout",
		Stack:      "at complexFunction (complex.js:42:10)\nat caller (complex.js:50:5)",
		JSONString: `{"name":"ComplexError","message":"complex error occurred","code":500}`,
	}

	// Test Error() method returns correct format
	require.EqualValues(t, "ComplexError: complex error occurred", err.Error())

	// Test all fields are preserved
	require.EqualValues(t, "ComplexError", err.Name)
	require.EqualValues(t, "complex error occurred", err.Message)
	require.EqualValues(t, "network timeout", err.Cause)
	require.EqualValues(t, "at complexFunction (complex.js:42:10)\nat caller (complex.js:50:5)", err.Stack)
	require.EqualValues(t, `{"name":"ComplexError","message":"complex error occurred","code":500}`, err.JSONString)
}

// TestErrorSpecialCharacters tests Error with special characters.
func TestErrorSpecialCharacters(t *testing.T) {
	testCases := []struct {
		name        string
		errorName   string
		message     string
		expectedErr string
	}{
		{
			name:        "unicode characters",
			errorName:   "UnicodeError",
			message:     "处理中文字符时出错",
			expectedErr: "UnicodeError: 处理中文字符时出错",
		},
		{
			name:        "special symbols",
			errorName:   "SymbolError",
			message:     "Error with symbols: @#$%^&*()",
			expectedErr: "SymbolError: Error with symbols: @#$%^&*()",
		},
		{
			name:        "newlines and tabs",
			errorName:   "FormatError",
			message:     "Error with\nnewlines\tand\ttabs",
			expectedErr: "FormatError: Error with\nnewlines\tand\ttabs",
		},
		{
			name:        "quotes",
			errorName:   "QuoteError",
			message:     `Error with "double" and 'single' quotes`,
			expectedErr: `QuoteError: Error with "double" and 'single' quotes`,
		},
		{
			name:        "backslashes",
			errorName:   "PathError",
			message:     `Path error: C:\Windows\System32\file.txt`,
			expectedErr: `PathError: Path error: C:\Windows\System32\file.txt`,
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
}

// TestErrorLongStrings tests Error with very long strings.
func TestErrorLongStrings(t *testing.T) {
	longName := make([]byte, 1000)
	for i := range longName {
		longName[i] = 'A' + byte(i%26)
	}

	longMessage := make([]byte, 5000)
	for i := range longMessage {
		longMessage[i] = 'a' + byte(i%26)
	}

	err := quickjs.Error{
		Name:    string(longName),
		Message: string(longMessage),
	}

	expectedErr := string(longName) + ": " + string(longMessage)
	require.EqualValues(t, expectedErr, err.Error())
	require.EqualValues(t, string(longName), err.Name)
	require.EqualValues(t, string(longMessage), err.Message)
}

// TestErrorAsGoError tests Error implementing Go's error interface.
func TestErrorAsGoError(t *testing.T) {
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
}

// TestErrorComparison tests Error comparison and equality.
func TestErrorComparison(t *testing.T) {
	err1 := quickjs.Error{
		Name:    "TestError",
		Message: "test message",
	}

	err2 := quickjs.Error{
		Name:    "TestError",
		Message: "test message",
	}

	err3 := quickjs.Error{
		Name:    "DifferentError",
		Message: "test message",
	}

	// Test that errors with same content have same string representation
	require.EqualValues(t, err1.Error(), err2.Error())

	// Test that errors with different content have different string representation
	require.NotEqual(t, err1.Error(), err3.Error())

	// Test struct equality
	require.Equal(t, err1, err2)
	require.NotEqual(t, err1, err3)
}

// TestErrorZeroValue tests Error zero value behavior.
func TestErrorZeroValue(t *testing.T) {
	var err quickjs.Error

	// Test zero value
	require.EqualValues(t, "", err.Name)
	require.EqualValues(t, "", err.Message)
	require.EqualValues(t, "", err.Cause)
	require.EqualValues(t, "", err.Stack)
	require.EqualValues(t, "", err.JSONString)

	// Test Error() method with zero value
	require.EqualValues(t, ": ", err.Error())
}

// TestErrorFieldAccess tests direct field access and modification.
func TestErrorFieldAccess(t *testing.T) {
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
}

// TestErrorEdgeCases tests various edge cases.
func TestErrorEdgeCases(t *testing.T) {
	// Test with only colons in name and message
	colonErr := quickjs.Error{
		Name:    ":",
		Message: ":",
	}
	require.EqualValues(t, ":: :", colonErr.Error())

	// Test with whitespace only
	whitespaceErr := quickjs.Error{
		Name:    "   ",
		Message: "\t\n  ",
	}
	require.EqualValues(t, "   : \t\n  ", whitespaceErr.Error())

	// Test with control characters
	controlErr := quickjs.Error{
		Name:    "ControlError",
		Message: "Message with \x00 null byte and \x07 bell",
	}
	require.EqualValues(t, "ControlError: Message with \x00 null byte and \x07 bell", controlErr.Error())
}

// TestErrorStringer tests fmt.Stringer interface compliance.
func TestErrorStringer(t *testing.T) {
	err := quickjs.Error{
		Name:    "StringerTest",
		Message: "testing stringer interface",
	}

	// The Error() method should be used by fmt package
	require.EqualValues(t, "StringerTest: testing stringer interface", err.Error())

	// Test that it works with string formatting
	formatted := err.Error()
	require.EqualValues(t, "StringerTest: testing stringer interface", formatted)
}

// TestErrorIntegrationWithQuickJS tests Error integration with QuickJS context.
func TestErrorIntegrationWithQuickJS(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create a JavaScript error and convert it to Go error
	jsError, err := ctx.Eval(`new Error("JavaScript error")`)
	require.NoError(t, err)
	defer jsError.Free()

	// Convert to quickjs.Error
	convertedErr := jsError.ToError()
	require.NotNil(t, convertedErr)

	// Test that it's a quickjs.Error
	quickjsErr, ok := convertedErr.(*quickjs.Error)
	require.True(t, ok)
	require.Contains(t, quickjsErr.Error(), "JavaScript error")

	// Test different JavaScript error types
	errorTypes := []string{
		"new TypeError('type error')",
		"new ReferenceError('reference error')",
		"new SyntaxError('syntax error')",
		"new RangeError('range error')",
	}

	for _, errorCode := range errorTypes {
		jsErr, evalErr := ctx.Eval(errorCode)
		require.NoError(t, evalErr)
		defer jsErr.Free()

		convertedErr := jsErr.ToError()
		require.NotNil(t, convertedErr)

		quickjsErr, ok := convertedErr.(*quickjs.Error)
		require.True(t, ok)
		require.NotEmpty(t, quickjsErr.Error())
	}
}
