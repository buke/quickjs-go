package quickjs

import (
	"errors"
	"fmt"
	"unsafe"
)

/*
#include "bridge.h"
*/
import "C"

// =============================================================================
// MODULE TYPES AND STRUCTURES
// =============================================================================

// ModuleExportEntry represents a single module export
type ModuleExportEntry struct {
	Name string    // Export name ("default" for default export)
	Spec ValueSpec // Export definition for Build-time materialization
}

// ModuleBuilder provides a fluent API for building JavaScript modules
// Uses builder pattern for easy and readable module definition
type ModuleBuilder struct {
	name    string              // Module name
	exports []ModuleExportEntry // All exports (including default)
}

// =============================================================================
// MODULE BUILDER API
// =============================================================================

// NewModuleBuilder creates a new ModuleBuilder with the specified name
// This is the entry point for building JavaScript modules
func NewModuleBuilder(name string) *ModuleBuilder {
	return &ModuleBuilder{
		name:    name,
		exports: make([]ModuleExportEntry, 0),
	}
}

// Export adds an export to the module
// This is the core method that handles all types of exports including default
// For default export, use name="default"
// Deprecated: Use ExportValue or ExportLiteral for declarative, reusable module definitions.
func (mb *ModuleBuilder) Export(name string, value *Value) *ModuleBuilder {
	var spec ValueSpec
	if value != nil {
		spec = contextValueSpec{value: value}
	}
	return mb.ExportValue(name, spec)
}

// ExportValue adds an export ValueSpec to the module.
func (mb *ModuleBuilder) ExportValue(name string, spec ValueSpec) *ModuleBuilder {
	mb.exports = append(mb.exports, ModuleExportEntry{
		Name: name,
		Spec: spec,
	})
	return mb
}

// ExportLiteral adds a literal export definition to the module.
func (mb *ModuleBuilder) ExportLiteral(name string, value interface{}) *ModuleBuilder {
	return mb.ExportValue(name, MarshalSpec{Value: value})
}

// Build creates and registers the JavaScript module in the given context
// The module will be available for import in JavaScript code
// ValueSpec entries are captured by shallow snapshot. Do not mutate pointer-based
// ValueSpec implementations after Build, or module initialization may observe changes.
func (mb *ModuleBuilder) Build(ctx *Context) error {
	return createModule(ctx, mb)
}

// =============================================================================
// MODULE CREATION IMPLEMENTATION
// =============================================================================

// validateModuleBuilder validates ModuleBuilder configuration
func validateModuleBuilder(builder *ModuleBuilder) error {
	if builder.name == "" {
		return errors.New("module name cannot be empty")
	}

	// Check for duplicate export names
	nameSet := make(map[string]bool)
	for _, export := range builder.exports {
		if export.Name == "" {
			return errors.New("export name cannot be empty")
		}
		if export.Spec == nil {
			return fmt.Errorf("export value is required: %s", export.Name)
		}
		if nameSet[export.Name] {
			return fmt.Errorf("duplicate export name: %s", export.Name)
		}
		nameSet[export.Name] = true
	}

	return nil
}

func cloneModuleBuilder(builder *ModuleBuilder) *ModuleBuilder {
	if builder == nil {
		return nil
	}

	clonedExports := make([]ModuleExportEntry, len(builder.exports))
	copy(clonedExports, builder.exports)

	return &ModuleBuilder{
		name:    builder.name,
		exports: clonedExports,
	}
}

// createModule implements the core module creation logic
// This function handles the QuickJS module creation and registration:
// 1. Module creation phase: create C module and declare exports
// 2. Module initialization phase: set actual export values via proxy
// The module will be available for import in JavaScript code
func createModule(ctx *Context, builder *ModuleBuilder) error {
	// Step 1: Validate module builder
	if err := validateModuleBuilder(builder); err != nil {
		return fmt.Errorf("module validation failed: %v", err)
	}
	snapshot := cloneModuleBuilder(builder)

	// Step 2: Store a build snapshot in HandleStore for initialization access.
	builderID := ctx.handleStore.Store(snapshot)

	// Step 3: Prepare export names for C function
	exportNames := make([]*C.char, len(snapshot.exports))
	exportCount := len(snapshot.exports)

	// Convert Go strings to C strings
	for i, export := range snapshot.exports {
		exportNames[i] = C.CString(export.Name)
	}

	// Prepare parameters for C function call
	moduleName := C.CString(snapshot.name)
	var exportNamesPtr **C.char
	if exportCount > 0 {
		exportNamesPtr = &exportNames[0]
	}

	// Step 4: Call C function to create module
	result := C.CreateModule(
		ctx.ref,
		moduleName,
		exportNamesPtr,
		C.int(exportCount),
		C.int32_t(builderID),
	)

	// Step 5: Clean up C strings
	C.free(unsafe.Pointer(moduleName))
	for _, cStr := range exportNames {
		C.free(unsafe.Pointer(cStr))
	}

	// Step 6: Check result and handle errors
	if result < 0 {
		// Clean up HandleStore on failure
		ctx.handleStore.Delete(builderID)
		return ctx.Exception()
	}

	// Module is now created and registered, ready for import
	return nil
}
