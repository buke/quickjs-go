package quickjs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBridgeRecoverHelperFunctions(t *testing.T) {
	require.True(t, triggerRecoverToJSInternalErrorNilContextForTest())
	require.Equal(t, -1, triggerRecoverToModuleErrorNilContextForTest())
	require.Equal(t, 0, triggerRecoverToInterruptNoopForTest(7, true))
	require.Equal(t, 7, triggerRecoverToInterruptNoopForTest(7, false))
}
