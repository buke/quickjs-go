package quickjs

import "unsafe"

/*
#include <stdint.h>
#include "bridge.h"
*/
import "C"

func setValueOpaqueForTest(val C.JSValue, opaqueID int32) {
	C.JS_SetOpaque(val, C.IntToOpaque(C.int32_t(opaqueID)))
}

func opaqueFromIDForTest(opaqueID int32) unsafe.Pointer {
	return C.IntToOpaque(C.int32_t(opaqueID))
}
