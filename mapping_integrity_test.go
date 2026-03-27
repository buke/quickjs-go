package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFailClosedPollutionTableDriven(t *testing.T) {
	type pollutionCase struct {
		name        string
		assertClean func(t *testing.T)
	}

	cases := []pollutionCase{
		{
			name: "ConstructorRegistryCorruption",
			assertClean: func(t *testing.T) {
				rt := NewRuntime()
				defer rt.Close()
				ctx := rt.NewContext()
				defer ctx.Close()

				constructor, _ := createPointClass(ctx)
				require.False(t, constructor.IsException())
				defer constructor.Free()

				registerConstructorClassID(ctx, constructor.ref, 123)
				got, ok := getConstructorClassID(ctx, constructor.ref)
				require.True(t, ok)
				require.Equal(t, uint32(123), got)

				storeConstructorRegistryEntryForTest(ctx, constructor.ref, "bad-class-id")
				got, ok = getConstructorClassID(ctx, constructor.ref)
				require.False(t, ok)
				require.Equal(t, uint32(0), got)

				_, stillExists := loadConstructorRegistryEntryForTest(ctx, constructor.ref)
				require.False(t, stillExists)
			},
		},
		{
			name: "ContextMappingCorruption",
			assertClean: func(t *testing.T) {
				rt := NewRuntime()
				defer rt.Close()
				ctx := rt.NewContext()
				defer ctx.Close()

				require.Same(t, ctx, getContextFromJS(ctx.ref))

				contextMapping.Store(ctx.ref, "bad-context")
				require.Nil(t, getContextFromJS(ctx.ref))

				_, stillExists := contextMapping.Load(ctx.ref)
				require.False(t, stillExists)
			},
		},
		{
			name: "RuntimeMappingCorruption",
			assertClean: func(t *testing.T) {
				rt := NewRuntime()
				defer rt.Close()

				require.Same(t, rt, getRuntimeFromJS(rt.ref))

				runtimeMapping.Store(rt.ref, "bad-runtime")
				require.Nil(t, getRuntimeFromJS(rt.ref))

				_, stillExists := runtimeMapping.Load(rt.ref)
				require.False(t, stillExists)
			},
		},
		{
			name: "HandleStoreCorruption",
			assertClean: func(t *testing.T) {
				hs := newHandleStore()
				id := hs.Store("ok")

				value, ok := hs.Load(id)
				require.True(t, ok)
				require.Equal(t, "ok", value)

				hs.handles.Store(id, "bad-handle")
				value, ok = hs.Load(id)
				require.False(t, ok)
				require.Nil(t, value)

				_, stillExists := hs.handles.Load(id)
				require.False(t, stillExists)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assertClean(t)
		})
	}
}

func TestMappingIntegrityContracts(t *testing.T) {
	TestFailClosedPollutionTableDriven(t)
}
