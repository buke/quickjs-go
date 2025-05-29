package quickjs

/*
#include "bridge.h"
*/
import "C"

// Atom represents a QuickJS atom - unique strings used for object property names.
// Object property names and some strings are stored as Atoms (unique strings) to save memory and allow fast comparison.
// Atoms are represented as a 32 bit integer. Half of the atom range is reserved for immediate integer literals from 0 to 2^{31}-1.
type Atom struct {
	ctx *Context
	ref C.JSAtom
}

// Free decrements the reference count of the atom.
func (a Atom) Free() {
	C.JS_FreeAtom(a.ctx.ref, a.ref)
}

// String returns the string representation of the atom.
func (a Atom) String() string {
	ptr := C.JS_AtomToCString(a.ctx.ref, a.ref)
	defer C.JS_FreeCString(a.ctx.ref, ptr)
	return C.GoString(ptr)
}

// Value returns the value representation of the atom.
func (a Atom) Value() Value {
	return Value{ctx: a.ctx, ref: C.JS_AtomToValue(a.ctx.ref, a.ref)}
}

// propertyEnum is a wrapper around JSAtom for property enumeration.
type propertyEnum struct {
	IsEnumerable bool
	atom         Atom
}

// String returns the atom string representation of the property.
func (p propertyEnum) String() string {
	return p.atom.String()
}
