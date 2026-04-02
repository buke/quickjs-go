/*
Package quickjs Go bindings to QuickJS: a fast, small, and embeddable ES2020 JavaScript interpreter
*/
package quickjs

/*
#cgo CFLAGS: -I${SRCDIR}/deps/quickjs -D_GNU_SOURCE
#cgo LDFLAGS: -lm
*/
import "C"
