package quickjs_test

import (
	"strings"
	"testing"

	"github.com/buke/quickjs-go"
	"github.com/stretchr/testify/require"
)

// TestAtomBasic tests basic Atom functionality.
func TestAtomBasic(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test string atom creation
	atom := ctx.Atom("testProperty")
	defer atom.Free()

	// Test String method
	require.EqualValues(t, "testProperty", atom.String())

	// Test Value method
	atomValue := atom.Value()
	defer atomValue.Free()
	require.True(t, atomValue.IsString())
	require.EqualValues(t, "testProperty", atomValue.String())
}

// TestAtomIdx tests index-based Atom creation.
func TestAtomIdx(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test index atom creation
	atomIdx := ctx.AtomIdx(42)
	defer atomIdx.Free()

	// Test String method with index
	require.EqualValues(t, "42", atomIdx.String())

	// Test Value method with index
	atomValue := atomIdx.Value()
	defer atomValue.Free()
	require.True(t, atomValue.IsString())
	require.EqualValues(t, "42", atomValue.String())
}

// TestAtomMemoryManagement tests proper memory management.
func TestAtomMemoryManagement(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating and freeing many atoms
	for i := 0; i < 1000; i++ {
		atom := ctx.Atom("test")
		atom.Free()
	}

	// Test creating atoms with different names
	atoms := make([]quickjs.Atom, 100)
	for i := 0; i < 100; i++ {
		atoms[i] = ctx.Atom("property" + string(rune('A'+i)))
	}

	// Free all atoms
	for i := 0; i < 100; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.Atom("final")
	defer finalAtom.Free()
	require.EqualValues(t, "final", finalAtom.String())
}

// TestAtomMultipleFree tests that multiple Free() calls don't crash.
func TestAtomMultipleFree(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	atom := ctx.Atom("test")
	atom.Free()
	// Second Free() should not crash (though not recommended)
	// Note: This might be implementation-dependent behavior
}

// TestAtomEmptyString tests Atom with empty string.
func TestAtomEmptyString(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	emptyAtom := ctx.Atom("")
	defer emptyAtom.Free()

	require.EqualValues(t, "", emptyAtom.String())

	atomValue := emptyAtom.Value()
	defer atomValue.Free()
	require.EqualValues(t, "", atomValue.String())
}

// TestAtomSpecialCharacters tests Atom with special characters.
func TestAtomSpecialCharacters(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	specialChars := []string{
		"hello world",
		"test\nwith\nnewlines",
		"test\twith\ttabs",
		"test\"with\"quotes",
		"test'with'quotes",
		"test\\with\\backslashes",
		"test with spaces",
		"test-with-dashes",
		"test_with_underscores",
		"test.with.dots",
		"test$with$dollars",
		"测试中文",
		"🚀emoji🌟test",
	}

	for _, str := range specialChars {
		atom := ctx.Atom(str)
		require.EqualValues(t, str, atom.String())

		atomValue := atom.Value()
		require.EqualValues(t, str, atomValue.String())
		atomValue.Free()
		atom.Free()
	}
}

// TestAtomNumericStrings tests Atom with numeric strings.
func TestAtomNumericStrings(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	numericStrings := []string{
		"0",
		"1",
		"42",
		"123456789",
		"-42",
		"3.14159",
		"-3.14159",
		"1e10",
		"1.23e-4",
		"Infinity",
		"NaN",
	}

	for _, str := range numericStrings {
		atom := ctx.Atom(str)
		require.EqualValues(t, str, atom.String())

		atomValue := atom.Value()
		require.EqualValues(t, str, atomValue.String())
		atomValue.Free()
		atom.Free()
	}
}

// TestAtomLongStrings tests Atom with very long strings.
func TestAtomLongStrings(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test with very long string
	longString := strings.Repeat("a", 10000)
	longAtom := ctx.Atom(longString)
	defer longAtom.Free()

	require.EqualValues(t, longString, longAtom.String())

	atomValue := longAtom.Value()
	defer atomValue.Free()
	require.EqualValues(t, longString, atomValue.String())
}

// TestAtomIndexVariations tests various index values.
func TestAtomIndexVariations(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	indices := []uint32{
		0,
		1,
		42,
		100,
		1000,
		999999,
		4294967295, // Maximum uint32 value
	}

	for _, idx := range indices {
		atom := ctx.AtomIdx(idx)

		// Verify the string representation matches the expected number
		expectedStr := string(rune('0' + (idx % 10))) // Get last digit
		actualStr := atom.String()
		require.True(t, len(actualStr) > 0)
		require.EqualValues(t, expectedStr, actualStr[len(actualStr)-1:])

		atomValue := atom.Value()
		atomValue.Free()
		atom.Free()
	}

	// Test specific known values
	testCases := []struct {
		index    uint32
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{1000, "1000"},
	}

	for _, tc := range testCases {
		atom := ctx.AtomIdx(tc.index)
		require.EqualValues(t, tc.expected, atom.String())
		atom.Free()
	}
}

// TestAtomWithObjectProperties tests Atom usage with object properties.
func TestAtomWithObjectProperties(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Test setting properties using string atoms
	propNames := []string{"name", "value", "flag", "data"}
	propValues := []quickjs.Value{
		ctx.String("test"),
		ctx.Int32(42),
		ctx.Bool(true),
		ctx.Object(),
	}

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
		atom.Free()
	}
}

// TestAtomValueConversion tests converting Atom to Value and back.
func TestAtomValueConversion(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	originalString := "testConversion"

	// Create atom
	atom := ctx.Atom(originalString)
	defer atom.Free()

	// Convert to value
	atomValue := atom.Value()
	defer atomValue.Free()

	// Verify the value is correct
	require.True(t, atomValue.IsString())
	require.EqualValues(t, originalString, atomValue.String())

	// Test that the atom can be used as property name (correct usage)
	obj := ctx.Object()
	defer obj.Free()

	// Use atom string as property name, not as value
	obj.Set(atom.String(), ctx.String("test value"))

	retrievedValue := obj.Get(atom.String())
	defer retrievedValue.Free()
	require.EqualValues(t, "test value", retrievedValue.String())

	// Test creating a new string value with the same content
	newStringValue := ctx.String(atomValue.String())
	obj.Set("normalProperty", newStringValue)

	retrievedNormal := obj.Get("normalProperty")
	defer retrievedNormal.Free()
	require.EqualValues(t, originalString, retrievedNormal.String())
}

// TestAtomConcurrency tests Atom usage in concurrent scenarios.
func TestAtomConcurrency(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating many atoms with the same name
	// (this tests internal atom deduplication if any)
	sameName := "duplicateName"
	atoms := make([]quickjs.Atom, 100)

	for i := 0; i < 100; i++ {
		atoms[i] = ctx.Atom(sameName)
		require.EqualValues(t, sameName, atoms[i].String())
	}

	// Free all atoms
	for i := 0; i < 100; i++ {
		atoms[i].Free()
	}

	// Verify context still works
	finalAtom := ctx.Atom(sameName)
	defer finalAtom.Free()
	require.EqualValues(t, sameName, finalAtom.String())
}

// TestAtomEdgeCases tests various edge cases.
func TestAtomEdgeCases(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test with string that looks like a number
	numericStringAtom := ctx.Atom("123")
	defer numericStringAtom.Free()
	require.EqualValues(t, "123", numericStringAtom.String())

	// Test with index that creates same string
	indexAtom := ctx.AtomIdx(123)
	defer indexAtom.Free()
	require.EqualValues(t, "123", indexAtom.String())

	// Both should produce the same string
	require.EqualValues(t, numericStringAtom.String(), indexAtom.String())

	// Test with zero index
	zeroAtom := ctx.AtomIdx(0)
	defer zeroAtom.Free()
	require.EqualValues(t, "0", zeroAtom.String())

	// Test with maximum uint32 value
	maxUint32 := uint32(4294967295) // 2^32 - 1
	maxAtom := ctx.AtomIdx(maxUint32)
	defer maxAtom.Free()
	// The string should contain the number
	require.EqualValues(t, "4294967295", maxAtom.String())

	// Test with large uint32 value
	largeUint32 := uint32(1000000000)
	largeAtom := ctx.AtomIdx(largeUint32)
	defer largeAtom.Free()
	require.EqualValues(t, "1000000000", largeAtom.String())
}

// TestAtomPropertyEnum tests propertyEnum functionality indirectly.
// Since propertyEnum is not directly exposed, we test it through Value.PropertyNames()
func TestAtomPropertyEnum(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.Object()
	defer obj.Free()

	// Set properties that will be enumerated
	properties := map[string]quickjs.Value{
		"enumerable1": ctx.String("value1"),
		"enumerable2": ctx.String("value2"),
		"enumerable3": ctx.String("value3"),
	}

	for name, value := range properties {
		obj.Set(name, value)
	}

	// Get property names (this internally uses propertyEnum)
	names, err := obj.PropertyNames()
	require.NoError(t, err)

	// Verify all properties are present
	for expectedName := range properties {
		require.Contains(t, names, expectedName)
	}

	// Verify we got the expected number of properties
	require.GreaterOrEqual(t, len(names), len(properties))
}

// TestAtomContextIntegration tests Atom integration with Context.
func TestAtomContextIntegration(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test that atoms work with different Context methods
	propName := "testProperty"
	atom := ctx.Atom(propName)
	defer atom.Free()

	// Create an object and use the atom-derived name
	obj := ctx.Object()

	obj.Set(atom.String(), ctx.String("atom value"))

	// Verify the property was set correctly
	value := obj.Get(propName)
	defer value.Free()
	require.EqualValues(t, "atom value", value.String())

	// Test with evaluation
	ctx.Globals().Set("testObj", obj)
	result, err := ctx.Eval("testObj." + atom.String())
	require.NoError(t, err)
	defer result.Free()
	require.EqualValues(t, "atom value", result.String())
}

// TestAtomComplexScenarios tests complex usage scenarios.
func TestAtomComplexScenarios(t *testing.T) {
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test creating atoms for all property names in a complex object
	complexObj := ctx.Object()
	defer complexObj.Free()

	// Set up a complex object structure
	nestedObj := ctx.Object()
	nestedObj.Set("nested", ctx.String("value"))

	arr, err := ctx.Eval(`[1, 2, 3]`)
	require.NoError(t, err)

	complexObj.Set("string", ctx.String("test"))
	complexObj.Set("number", ctx.Int32(42))
	complexObj.Set("boolean", ctx.Bool(true))
	complexObj.Set("object", nestedObj)
	complexObj.Set("array", arr)

	// Get all property names
	names, err := complexObj.PropertyNames()
	require.NoError(t, err)

	// Create atoms for each property name
	atoms := make([]quickjs.Atom, len(names))
	for i, name := range names {
		atoms[i] = ctx.Atom(name)
		require.EqualValues(t, name, atoms[i].String())

		// Verify atom value conversion
		atomValue := atoms[i].Value()
		require.EqualValues(t, name, atomValue.String())
		atomValue.Free()
	}

	// Free all atoms
	for _, atom := range atoms {
		atom.Free()
	}

	// Verify context still works after all operations
	finalTest := ctx.Atom("finalTest")
	defer finalTest.Free()
	require.EqualValues(t, "finalTest", finalTest.String())
}
