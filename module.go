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
	Name  string // Export name ("default" for default export)
	Value Value  // Export value
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
func (mb *ModuleBuilder) Export(name string, value Value) *ModuleBuilder {
	mb.exports = append(mb.exports, ModuleExportEntry{
		Name:  name,
		Value: value,
	})
	return mb
}

// Build creates and registers the JavaScript module in the given context
// The module will be available for import in JavaScript code
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
		if nameSet[export.Name] {
			return fmt.Errorf("duplicate export name: %s", export.Name)
		}
		nameSet[export.Name] = true
	}

	return nil
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

	// Step 2: Create C module with initialization function
	moduleName := C.CString(builder.name)
	defer C.free(unsafe.Pointer(moduleName))

	// Store ModuleBuilder in HandleStore for initialization function access
	builderID := ctx.handleStore.Store(builder)

	// Create C module with proxy initialization function
	cModule := C.JS_NewCModule(ctx.ref, moduleName, (*C.JSModuleInitFunc)(C.GoModuleInitProxy))
	if cModule == nil {
		ctx.handleStore.Delete(builderID)
		return fmt.Errorf("failed to create C module: %s", builder.name)
	}

	// Step 3: Pre-declare all exports (JS_AddModuleExport phase)
	// This must be done before module instantiation
	for _, export := range builder.exports {
		exportName := C.CString(export.Name)
		result := C.JS_AddModuleExport(ctx.ref, cModule, exportName)
		C.free(unsafe.Pointer(exportName))

		if result < 0 {
			ctx.handleStore.Delete(builderID)
			return fmt.Errorf("failed to add module export: %s", export.Name)
		}
	}

	// Step 4: Store builderID as module private value for initialization access
	builderValue := ctx.Int32(int32(builderID))
	C.JS_SetModulePrivateValue(ctx.ref, cModule, builderValue.ref)

	// Module is now created and registered, ready for import
	return nil
}
