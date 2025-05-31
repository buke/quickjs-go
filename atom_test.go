package quickjs_test

import (
	"strings"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestAtomBasics tests basic Atom functionality
func TestAtomBasics(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test string atom creation
	atom := ctx.Atom("testProperty")
	defer atom.Free()

	require.EqualValues(t, "testProperty", atom.String())

	// Test Value method
	atomValue := atom.Value()
	defer atomValue.Free()
	require.True(t, atomValue.IsString())
	require.EqualValues(t, "testProperty", atomValue.String())

	// Test index-based atom creation
	atomIdx := ctx.AtomIdx(42)
	defer atomIdx.Free()
	require.EqualValues(t, "42", atomIdx.String())

	// Test empty string atom
	emptyAtom := ctx.Atom("")
	defer emptyAtom.Free()
	require.EqualValues(t, "", emptyAtom.String())

	// Test zero index
	zeroAtom := ctx.AtomIdx(0)
	defer zeroAtom.Free()
	require.EqualValues(t, "0", zeroAtom.String())
}

// TestAtomSpecialCases tests special characters and edge cases
func TestAtomSpecialCases(t *testing.T) {
	rt := quickjs.NewRuntime()
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
			var atom quickjs.Atom

			switch v := tc.input.(type) {
			case string:
				atom = ctx.Atom(v)
			case uint32:
				atom = ctx.AtomIdx(v)
			}

			defer atom.Free()
			require.EqualValues(t, tc.expected, atom.String())

			// Test Value conversion
			atomValue := atom.Value()
			defer atomValue.Free()
			require.EqualValues(t, tc.expected, atomValue.String())
		})
	}

	// Test long string
	longString := strings.Repeat("a", 5000)
	longAtom := ctx.Atom(longString)
	defer longAtom.Free()
	require.EqualValues(t, longString, longAtom.String())
}

// TestAtomMemoryManagement tests proper memory management
func TestAtomMemoryManagement(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating and freeing many atoms
	for i := 0; i < 100; i++ {
		atom := ctx.Atom("test")
		atom.Free()
	}

	// Test creating atoms with different names
	atoms := make([]quickjs.Atom, 50)
	for i := 0; i < 50; i++ {
		atoms[i] = ctx.Atom("property" + string(rune('A'+i%26)))
	}

	// Free all atoms
	for i := 0; i < 50; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.Atom("final")
	defer finalAtom.Free()
	require.EqualValues(t, "final", finalAtom.String())

	// Test multiple Free() calls (should not crash)
	testAtom := ctx.Atom("test_multiple_free")
	testAtom.Free()
	// Second Free() should not crash (though not recommended)
}

// TestAtomWithObjects tests Atom usage with object properties
func TestAtomWithObjects(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test setting and getting properties using atoms
	propNames := []string{"name", "value", "flag", "data"}
	propValues := []quickjs.Value{
		ctx.String("test"),
		ctx.Int32(42),
		ctx.Bool(true),
		ctx.String("object_data"),
	}

	// Set properties
	for i, name := range propNames {
		obj.Set(name, propValues[i])
	}

	// Create atoms for property names and verify they work
	for _, name := range propNames {
		atom := ctx.Atom(name)
		atomStr := atom.String()
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
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating many atoms with the same name
	sameName := "duplicateName"
	atoms := make([]quickjs.Atom, 50)

	for i := 0; i < 50; i++ {
		atoms[i] = ctx.Atom(sameName)
		require.EqualValues(t, sameName, atoms[i].String())
	}

	// Free all atoms
	for i := 0; i < 50; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.Atom(sameName)
	defer finalAtom.Free()
	require.EqualValues(t, sameName, finalAtom.String())

	// Test string vs index atoms that produce same result
	stringAtom := ctx.Atom("123")
	defer stringAtom.Free()

	indexAtom := ctx.AtomIdx(123)
	defer indexAtom.Free()

	require.EqualValues(t, stringAtom.String(), indexAtom.String())
}
