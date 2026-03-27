package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func assertClassHandleCountStable(t *testing.T, ctx *Context, baseHandles int) {
	t.Helper()
	require.Equal(t, baseHandles, ctx.handleStore.Count())
}
