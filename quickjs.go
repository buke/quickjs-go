/*
Package quickjs Go bindings to QuickJS: a fast, small, and embeddable ES2020 JavaScript interpreter
*/
package quickjs

/*
#cgo CFLAGS: -I./deps/include
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/deps/libs/darwin_amd64 -lquickjs -lm
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/deps/libs/darwin_arm64 -lquickjs -lm
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/deps/libs/linux_amd64 -lquickjs -lm
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/deps/libs/linux_arm64 -lquickjs -lm
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/deps/libs/windows_amd64 -lquickjs -lm
#cgo windows,386 LDFLAGS: -L${SRCDIR}/deps/libs/windows_386 -lquickjs -lm
*/
import "C"
