package quickjs

import (
	"runtime/cgo"
	"testing"

	"github.com/stretchr/testify/require"
)

func findHandleByPredicate(t *testing.T, hs *handleStore, predicate func(interface{}) bool) (int32, cgo.Handle) {
	t.Helper()
	require.NotNil(t, hs)

	var foundID int32
	var foundHandle cgo.Handle
	hs.handles.Range(func(key, value interface{}) bool {
		handle, ok := value.(cgo.Handle)
		if !ok {
			return true
		}
		if predicate(handle.Value()) {
			id, ok := key.(int32)
			if !ok {
				return true
			}
			foundID = id
			foundHandle = handle
			return false
		}
		return true
	})

	require.NotZero(t, foundID)
	return foundID, foundHandle
}

func replaceHandleWithValueForTest(t *testing.T, hs *handleStore, id int32, replacementValue interface{}) func() {
	t.Helper()
	require.NotNil(t, hs)

	raw, ok := hs.handles.Load(id)
	require.True(t, ok)
	original, ok := raw.(cgo.Handle)
	require.True(t, ok)

	replacement := cgo.NewHandle(replacementValue)
	hs.handles.Store(id, replacement)

	return func() {
		hs.handles.Store(id, original)
		replacement.Delete()
	}
}
