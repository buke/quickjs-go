package quickjs

import (
	"math"
	"runtime/cgo"
	"sync"
	"sync/atomic"
)

// HandleStore manages cgo.Handle storage for function lifecycle management
type handleStore struct {
	handles sync.Map     // map[int32]cgo.Handle - thread-safe storage
	nextID  atomic.Int32 // atomic ID generation to avoid locks
}

// NewHandleStore creates a new handle store
func newHandleStore() *handleStore {
	return newHandleStoreWithStartID(1)
}

// NewHandleStoreWithStartID creates a new handle store with custom start ID (for testing)
func newHandleStoreWithStartID(startID int32) *handleStore {
	hs := &handleStore{}
	hs.nextID.Store(startID) // start from custom ID, 0 is reserved as invalid
	return hs
}

// Store stores a value and returns int32 ID (safe for JS magic parameter)
func (hs *handleStore) Store(value interface{}) int32 {
	if hs == nil {
		return 0
	}

	id := hs.nextID.Add(1)

	// check int32 overflow to prevent magic parameter issues
	if id <= 0 || id == math.MaxInt32 {
		panic("quickjs: HandleStore ID overflow, too many functions registered")
	}

	handle := cgo.NewHandle(value)
	hs.handles.Store(id, handle)
	return id
}

// Load loads value by ID
func (hs *handleStore) Load(id int32) (interface{}, bool) {
	if hs == nil || id <= 0 {
		return nil, false
	}

	if value, ok := hs.handles.Load(id); ok {
		handle, ok := value.(cgo.Handle)
		if !ok {
			hs.handles.Delete(id)
			return nil, false
		}
		return handle.Value(), true
	}
	return nil, false
}

// Delete deletes handle by ID and properly releases cgo.Handle
func (hs *handleStore) Delete(id int32) bool {
	if hs == nil || id <= 0 {
		return false
	}

	if value, ok := hs.handles.LoadAndDelete(id); ok {
		handle, ok := value.(cgo.Handle)
		if !ok {
			return false
		}
		handle.Delete() // critical: release cgo.Handle to prevent memory leak
		return true
	}
	return false
}

// Clear clears all handles (called on Context.Close)
func (hs *handleStore) Clear() {
	if hs == nil {
		return
	}

	hs.handles.Range(func(key, value interface{}) bool {
		handle, ok := value.(cgo.Handle)
		if ok {
			handle.Delete() // ensure all cgo.Handle are properly released
		}
		hs.handles.Delete(key)
		return true
	})
}

// Count returns number of stored handles (for debugging)
func (hs *handleStore) Count() int {
	if hs == nil {
		return 0
	}

	count := 0
	hs.handles.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
