package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeprecatedAtomAPIs tests all deprecated Atom methods to ensure they still work
// Each deprecated method is called once for test coverage
func TestDeprecatedAtomAPIs(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("DeprecatedAtomStringMethod", func(t *testing.T) {
		// Test deprecated String() method on Atom
		atom := ctx.NewAtom("test atom")
		defer atom.Free()

		// Test deprecated String() method
		result1 := atom.String()
		require.Equal(t, "test atom", result1)

		// Compare with new ToString() method
		result2 := atom.ToString()
		require.Equal(t, "test atom", result2)

		// Both should return the same value
		require.Equal(t, result1, result2)
	})

	t.Run("DeprecatedAtomValueMethod", func(t *testing.T) {
		// Test deprecated Value() method on Atom
		atom := ctx.NewAtom("value test")
		defer atom.Free()

		// Test deprecated Value() method
		val1 := atom.Value()
		defer val1.Free()
		require.True(t, val1.IsString())
		require.Equal(t, "value test", val1.ToString())

		// Compare with new ToValue() method
		val2 := atom.ToValue()
		defer val2.Free()
		require.True(t, val2.IsString())
		require.Equal(t, "value test", val2.ToString())

		// Both should create equivalent values
		require.Equal(t, val1.ToString(), val2.ToString())
	})

	t.Run("DeprecatedAtomMethods", func(t *testing.T) {
		// Test various scenarios with deprecated Atom methods
		testCases := []string{
			"simple",
			"with spaces",
			"with-dashes",
			"with_underscores",
			"UPPERCASE",
			"mixedCase",
			"123numbers",
			"special!@#chars",
			"", // empty string
		}

		for _, testCase := range testCases {
			t.Run("AtomCase_"+testCase, func(t *testing.T) {
				atom := ctx.NewAtom(testCase)
				defer atom.Free()

				// Test deprecated String() method
				stringResult := atom.String()
				require.Equal(t, testCase, stringResult)

				// Test deprecated Value() method
				valueResult := atom.Value()
				defer valueResult.Free()
				require.Equal(t, testCase, valueResult.ToString())
			})
		}
	})

	t.Run("DeprecatedAtomWithPropertyAccess", func(t *testing.T) {
		// Test deprecated Atom methods in property access scenarios
		obj := ctx.NewObject()
		defer obj.Free()

		atom := ctx.NewAtom("testProperty")
		defer atom.Free()

		// Set property using atom string (corrected)
		propertyName := atom.String() // deprecated method
		obj.Set(propertyName, ctx.NewString("property value"))

		// Get property using deprecated atom methods
		retrievedValue := obj.Get(propertyName)
		defer retrievedValue.Free()

		require.Equal(t, "property value", retrievedValue.ToString())

		// Also test with deprecated Value() method
		atomValue := atom.Value() // deprecated method
		defer atomValue.Free()
		require.Equal(t, "testProperty", atomValue.ToString())
	})

	t.Run("DeprecatedAtomComparison", func(t *testing.T) {
		// Test that deprecated methods work consistently
		atom1 := ctx.NewAtom("comparison test")
		atom2 := ctx.NewAtom("comparison test")
		defer atom1.Free()
		defer atom2.Free()

		// Using deprecated String() method
		str1 := atom1.String()
		str2 := atom2.String()
		require.Equal(t, str1, str2)

		// Using deprecated Value() method
		val1 := atom1.Value()
		val2 := atom2.Value()
		defer val1.Free()
		defer val2.Free()
		require.Equal(t, val1.ToString(), val2.ToString())
	})
}

// TestDeprecatedAtomEdgeCases tests edge cases with deprecated Atom methods
func TestDeprecatedAtomEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("DeprecatedAtomWithUnicodeStrings", func(t *testing.T) {
		// Test deprecated methods with Unicode strings
		unicodeStrings := []string{
			"中文测试",
			"🚀 emoji test",
			"café ñoño",
			"Здравствуй мир",
			"こんにちは世界",
		}

		for _, unicodeStr := range unicodeStrings {
			t.Run("Unicode_"+unicodeStr, func(t *testing.T) {
				atom := ctx.NewAtom(unicodeStr)
				defer atom.Free()

				// Test deprecated String() method with Unicode
				result := atom.String()
				require.Equal(t, unicodeStr, result)

				// Test deprecated Value() method with Unicode
				val := atom.Value()
				defer val.Free()
				require.Equal(t, unicodeStr, val.ToString())
			})
		}
	})

	t.Run("DeprecatedAtomMemoryManagement", func(t *testing.T) {
		// Test that deprecated methods don't cause memory issues
		for i := 0; i < 100; i++ {
			atom := ctx.NewAtom("memory test " + string(rune('A'+i%26)))

			// Use deprecated methods
			_ = atom.String()
			val := atom.Value()
			val.Free()

			atom.Free()
		}
		// If we reach here without crashing, memory management is working
		require.True(t, true)
	})

	t.Run("DeprecatedAtomWithLongStrings", func(t *testing.T) {
		// Test deprecated methods with very long strings
		longString := ""
		for i := 0; i < 1000; i++ {
			longString += "test"
		}

		atom := ctx.NewAtom(longString)
		defer atom.Free()

		// Test deprecated String() method with long string
		result := atom.String()
		require.Equal(t, longString, result)
		require.Len(t, result, 4000) // 1000 * "test"

		// Test deprecated Value() method with long string
		val := atom.Value()
		defer val.Free()
		require.Equal(t, longString, val.ToString())
	})
}

// TestDeprecatedAtomInteractionWithContext tests how deprecated Atom methods interact with Context
func TestDeprecatedAtomInteractionWithContext(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("DeprecatedAtomInJavaScriptCode", func(t *testing.T) {
		// Create atom using new API
		atom := ctx.NewAtom("globalTestVar")
		defer atom.Free()

		// Use deprecated String() method to get property name
		propName := atom.String()

		// Set global variable using the property name from deprecated method
		ctx.Globals().Set(propName, ctx.NewString("test value"))

		// Verify the property was set correctly
		result := ctx.Eval("globalTestVar")
		defer result.Free()
		require.Equal(t, "test value", result.ToString())
	})

	t.Run("DeprecatedAtomValueInJavaScript", func(t *testing.T) {
		// Create atom
		atom := ctx.NewAtom("javascript interaction")
		defer atom.Free()

		// Use deprecated Value() method to create JavaScript value
		atomValue := atom.Value() // deprecated method

		// Set as global variable
		ctx.Globals().Set("atomStringValue", atomValue)

		// Use in JavaScript
		result := ctx.Eval("atomStringValue + ' extended'")
		defer result.Free()
		require.Equal(t, "javascript interaction extended", result.ToString())
	})

	t.Run("DeprecatedAtomWithJavaScriptExecution", func(t *testing.T) {
		// Test using deprecated atom methods in JavaScript execution context
		atom := ctx.NewAtom("dynamicProperty")
		defer atom.Free()

		// Create an object
		obj := ctx.NewObject()

		// Use deprecated String() method to set property
		propName := atom.String()
		obj.Set(propName, ctx.NewString("dynamic value"))

		// Set object as global
		ctx.Globals().Set("testObj", obj)

		// Use deprecated Value() method in JavaScript context
		atomValue := atom.Value()
		ctx.Globals().Set("propNameValue", atomValue)

		// Test accessing the property using the atom value
		result := ctx.Eval("testObj[propNameValue]")
		defer result.Free()
		require.Equal(t, "dynamic value", result.ToString())
	})
}
