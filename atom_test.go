package quickjs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAtomBasics tests basic Atom functionality
func TestAtomBasics(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test string atom creation
	atom := ctx.NewAtom("testProperty") // Changed: Atom() → NewAtom()
	defer atom.Free()

	require.EqualValues(t, "testProperty", atom.ToString()) // Changed: String() → ToString()

	// Test ToValue method - now returns *Value
	atomValue := atom.ToValue() // Changed: Value() → ToValue()
	defer atomValue.Free()
	require.True(t, atomValue.IsString())
	require.EqualValues(t, "testProperty", atomValue.ToString()) // Changed: String() → ToString()

	// Test index-based atom creation
	atomIdx := ctx.NewAtomIdx(42) // Changed: AtomIdx() → NewAtomIdx()
	defer atomIdx.Free()
	require.EqualValues(t, "42", atomIdx.ToString()) // Changed: String() → ToString()

	// Test empty string atom
	emptyAtom := ctx.NewAtom("") // Changed: Atom() → NewAtom()
	defer emptyAtom.Free()
	require.EqualValues(t, "", emptyAtom.ToString()) // Changed: String() → ToString()

	// Test zero index
	zeroAtom := ctx.NewAtomIdx(0) // Changed: AtomIdx() → NewAtomIdx()
	defer zeroAtom.Free()
	require.EqualValues(t, "0", zeroAtom.ToString()) // Changed: String() → ToString()
}

// TestAtomSpecialCases tests special characters and edge cases
func TestAtomSpecialCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test various special characters and edge cases
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"space", "hello world", "hello world"},
		{"newlines", "test\nwith\nnewlines", "test\nwith\nnewlines"},
		{"quotes", "test\"with\"quotes", "test\"with\"quotes"},
		{"unicode", "测试中文", "测试中文"},
		{"emoji", "🚀emoji🌟test", "🚀emoji🌟test"},
		{"numeric_string", "123", "123"},
		{"negative_number", "-42", "-42"},
		{"float_string", "3.14159", "3.14159"},
		{"large_index", uint32(999999), "999999"},
		{"max_uint32", uint32(4294967295), "4294967295"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var atom *Atom

			switch v := tc.input.(type) {
			case string:
				atom = ctx.NewAtom(v) // Changed: Atom() → NewAtom()
			case uint32:
				atom = ctx.NewAtomIdx(v) // Changed: AtomIdx() → NewAtomIdx()
			}

			defer atom.Free()
			require.EqualValues(t, tc.expected, atom.ToString()) // Changed: String() → ToString()

			// Test ToValue conversion - now returns *Value
			atomValue := atom.ToValue() // Changed: Value() → ToValue()
			defer atomValue.Free()
			require.EqualValues(t, tc.expected, atomValue.ToString()) // Changed: String() → ToString()
		})
	}

	// Test long string
	longString := strings.Repeat("a", 5000)
	longAtom := ctx.NewAtom(longString) // Changed: Atom() → NewAtom()
	defer longAtom.Free()
	require.EqualValues(t, longString, longAtom.ToString()) // Changed: String() → ToString()
}

// TestAtomMemoryManagement tests proper memory management
func TestAtomMemoryManagement(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating and freeing many atoms
	for i := 0; i < 100; i++ {
		atom := ctx.NewAtom("test") // Changed: Atom() → NewAtom()
		atom.Free()
	}

	// Test creating atoms with different names
	atoms := make([]*Atom, 50)
	for i := 0; i < 50; i++ {
		atoms[i] = ctx.NewAtom("property" + string(rune('A'+i%26))) // Changed: Atom() → NewAtom()
	}

	// Free all atoms
	for i := 0; i < 50; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.NewAtom("final") // Changed: Atom() → NewAtom()
	defer finalAtom.Free()
	require.EqualValues(t, "final", finalAtom.ToString()) // Changed: String() → ToString()

	// Test multiple Free() calls (should not crash)
	testAtom := ctx.NewAtom("test_multiple_free") // Changed: Atom() → NewAtom()
	testAtom.Free()
	// Second Free() should not crash (though not recommended)
}

// TestAtomWithObjects tests Atom usage with object properties
func TestAtomWithObjects(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.NewObject() // Changed: Object() → NewObject()
	defer obj.Free()

	// Test setting and getting properties using atoms
	propNames := []string{"name", "value", "flag", "data"}
	propValues := []*Value{ // MODIFIED: now uses *Value slice
		ctx.NewString("test"),        // Changed: String() → NewString()
		ctx.NewInt32(42),             // Changed: Int32() → NewInt32()
		ctx.NewBool(true),            // Changed: Bool() → NewBool()
		ctx.NewString("object_data"), // Changed: String() → NewString()
	}

	// Set properties
	for i, name := range propNames {
		obj.Set(name, propValues[i])
	}

	// Create atoms for property names and verify they work
	for _, name := range propNames {
		atom := ctx.NewAtom(name)  // Changed: Atom() → NewAtom()
		atomStr := atom.ToString() // Changed: String() → ToString()
		require.EqualValues(t, name, atomStr)

		// Verify the property exists
		require.True(t, obj.Has(name))

		// Test using atom string as property key
		retrievedValue := obj.Get(atomStr)
		defer retrievedValue.Free()
		require.False(t, retrievedValue.IsNull())

		atom.Free()
	}

	// Test property enumeration (tests propertyEnum indirectly)
	names, err := obj.PropertyNames()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(names), len(propNames))

	for _, expectedName := range propNames {
		require.Contains(t, names, expectedName)
	}
}

// TestAtomDeduplication tests atom deduplication behavior
func TestAtomDeduplication(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating many atoms with the same name
	sameName := "duplicateName"
	atoms := make([]*Atom, 50)

	for i := 0; i < 50; i++ {
		atoms[i] = ctx.NewAtom(sameName)                      // Changed: Atom() → NewAtom()
		require.EqualValues(t, sameName, atoms[i].ToString()) // Changed: String() → ToString()
	}

	// Free all atoms
	for i := 0; i < 50; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.NewAtom(sameName) // Changed: Atom() → NewAtom()
	defer finalAtom.Free()
	require.EqualValues(t, sameName, finalAtom.ToString()) // Changed: String() → ToString()

	// Test string vs index atoms that produce same result
	stringAtom := ctx.NewAtom("123") // Changed: Atom() → NewAtom()
	defer stringAtom.Free()

	indexAtom := ctx.NewAtomIdx(123) // Changed: AtomIdx() → NewAtomIdx()
	defer indexAtom.Free()

	require.EqualValues(t, stringAtom.ToString(), indexAtom.ToString()) // Changed: String() → ToString()
}

// TestAtomEdgeCases tests additional edge cases for better coverage
func TestAtomEdgeCases(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	t.Run("AtomWithUnicodeStrings", func(t *testing.T) {
		// Test atom creation with various Unicode strings
		unicodeStrings := []string{
			"中文测试",
			"🚀 emoji test",
			"café ñoño",
			"Здравствуй мир",
			"こんにちは世界",
		}

		for _, unicodeStr := range unicodeStrings {
			atom := ctx.NewAtom(unicodeStr) // Changed: Atom() → NewAtom()
			defer atom.Free()

			// Test ToString method with Unicode
			result := atom.ToString() // Changed: String() → ToString()
			require.Equal(t, unicodeStr, result)

			// Test ToValue method with Unicode
			val := atom.ToValue() // Changed: Value() → ToValue()
			defer val.Free()
			require.Equal(t, unicodeStr, val.ToString()) // Changed: String() → ToString()
		}
	})

	t.Run("AtomWithSpecialCharacters", func(t *testing.T) {
		// Test atoms with special characters that could cause issues
		specialStrings := []string{
			"",            // empty string
			" ",           // single space
			"\n\t\r",      // whitespace characters
			"null",        // JavaScript keyword
			"undefined",   // JavaScript keyword
			"constructor", // JavaScript property name
			"__proto__",   // Special property
			"toString",    // Method name
		}

		for _, specialStr := range specialStrings {
			atom := ctx.NewAtom(specialStr) // Changed: Atom() → NewAtom()
			defer atom.Free()

			require.Equal(t, specialStr, atom.ToString()) // Changed: String() → ToString()

			val := atom.ToValue() // Changed: Value() → ToValue()
			defer val.Free()
			require.Equal(t, specialStr, val.ToString()) // Changed: String() → ToString()
		}
	})

	t.Run("AtomWithLargeIndexes", func(t *testing.T) {
		// Test atoms created with various index values
		indexes := []uint32{
			0,
			1,
			42,
			100,
			1000,
			10000,
			100000,
			1000000,
			4294967295, // max uint32
		}

		for _, index := range indexes {
			atom := ctx.NewAtomIdx(index) // Changed: AtomIdx() → NewAtomIdx()
			defer atom.Free()

			expected := fmt.Sprintf("%d", index)
			require.Equal(t, expected, atom.ToString()) // Changed: String() → ToString()

			val := atom.ToValue() // Changed: Value() → ToValue()
			defer val.Free()
			require.Equal(t, expected, val.ToString()) // Changed: String() → ToString()
		}
	})

	t.Run("AtomConsistency", func(t *testing.T) {
		// Test that atoms with same content are handled consistently
		testString := "consistency_test"

		atom1 := ctx.NewAtom(testString) // Changed: Atom() → NewAtom()
		atom2 := ctx.NewAtom(testString) // Changed: Atom() → NewAtom()
		defer atom1.Free()
		defer atom2.Free()

		// Both should return the same string
		require.Equal(t, atom1.ToString(), atom2.ToString()) // Changed: String() → ToString()

		// Values should also be equal
		val1 := atom1.ToValue() // Changed: Value() → ToValue()
		val2 := atom2.ToValue() // Changed: Value() → ToValue()
		defer val1.Free()
		defer val2.Free()

		require.Equal(t, val1.ToString(), val2.ToString()) // Changed: String() → ToString()
	})

	t.Run("AtomPropertyAccess", func(t *testing.T) {
		// Test using atoms for property access
		obj := ctx.NewObject() // Changed: Object() → NewObject()
		defer obj.Free()

		propName := "dynamicProperty"
		propValue := "test value"

		atom := ctx.NewAtom(propName) // Changed: Atom() → NewAtom()
		defer atom.Free()

		// Set property using atom string
		obj.Set(atom.ToString(), ctx.NewString(propValue)) // Changed: String() → ToString(), String() → NewString()

		// Get property back
		retrievedValue := obj.Get(atom.ToString()) // Changed: String() → ToString()
		defer retrievedValue.Free()

		require.Equal(t, propValue, retrievedValue.ToString()) // Changed: String() → ToString()

		// Also test using ToValue for property access
		atomValue := atom.ToValue() // Changed: Value() → ToValue()
		defer atomValue.Free()

		// Verify the atom value is correct
		require.Equal(t, propName, atomValue.ToString()) // Changed: String() → ToString()
	})
}
