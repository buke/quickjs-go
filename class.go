package quickjs

/*
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// =============================================================================
// RUNTIME-SCOPED CONSTRUCTOR REGISTRY FOR UNIFIED MAPPING
// =============================================================================

// Helper function to create a stable key from JSValue
// For constructor functions, we use the object pointer as a unique identifier
func jsValueToKey(jsVal C.JSValue) uint64 {
	// Constructors are JavaScript objects, so we use the object pointer
	// This is stable and unique for each JavaScript object instance
	objPtr := C.JS_VALUE_GET_PTR_Wrapper(jsVal)
	return uint64(uintptr(objPtr))
}

// registerConstructorClassID stores the constructor -> classID mapping
func registerConstructorClassID(ctx *Context, constructor C.JSValue, classID uint32) {
	if ctx == nil || ctx.runtime == nil {
		return
	}
	ctx.runtime.registerConstructorClassID(constructor, classID)
}

// getConstructorClassID retrieves the classID for a given constructor
func getConstructorClassID(ctx *Context, constructor C.JSValue) (uint32, bool) {
	if ctx == nil || ctx.runtime == nil {
		return 0, false
	}
	return ctx.runtime.getConstructorClassID(constructor)
}

func deleteConstructorClassID(ctx *Context, constructor C.JSValue) {
	if ctx == nil || ctx.runtime == nil {
		return
	}
	ctx.runtime.constructorRegistry.Delete(jsValueToKey(constructor))
}

// =============================================================================
// CLASS FINALIZER INTERFACE
// =============================================================================

// Optional cleanup interface for class instances
// Objects implementing this interface will have Finalize() called automatically
// when the JavaScript object is garbage collected
type ClassFinalizer interface {
	Finalize()
}

// =============================================================================
// CLASS-RELATED FUNCTION TYPES - MODIFIED FOR SCHEME C
// =============================================================================

// Class-related function types with consistent Class prefix
// These correspond exactly to QuickJS C API function pointer types

// MODIFIED FOR SCHEME C: ClassConstructorFunc signature changed
// Constructor now receives pre-created instance and returns Go object to associate
// This aligns with Scheme C design where instances are pre-created with bound properties
type ClassConstructorFunc func(ctx *Context, instance *Value, args []*Value) (interface{}, error)

// ClassMethodFunc represents both instance and static methods - changed to use pointers
// this parameter represents the object instance for instance methods,
// or the constructor function for static methods
// Corresponds to QuickJS JSCFunctionType.generic_magic
type ClassMethodFunc func(ctx *Context, this *Value, args []*Value) *Value

// ClassGetterFunc represents accessor getter functions - changed to use pointers
// Corresponds to QuickJS JSCFunctionType.getter_magic
type ClassGetterFunc func(ctx *Context, this *Value) *Value

// ClassSetterFunc represents accessor setter functions - changed to use pointers
// Returns the set value or an exception
// Corresponds to QuickJS JSCFunctionType.setter_magic
type ClassSetterFunc func(ctx *Context, this *Value, value *Value) *Value

// =============================================================================
// CLASS BINDING CONFIGURATION STRUCTURES
// =============================================================================

// MethodEntry represents a method binding configuration - unchanged
type MethodEntry struct {
	Name   string          // Method name in JavaScript
	Func   ClassMethodFunc // Method implementation function
	Static bool            // true for static methods, false for instance methods
	Length int             // Expected parameter count, 0 for default
}

// AccessorEntry represents an accessor binding configuration - unchanged
type AccessorEntry struct {
	Name   string          // Accessor name in JavaScript
	Getter ClassGetterFunc // Optional getter function
	Setter ClassSetterFunc // Optional setter function
	Static bool            // true for static accessors, false for instance accessors
}

// PropertyEntry represents a property binding configuration - changed to use pointer
type PropertyEntry struct {
	Name   string // Property name in JavaScript
	Spec   ValueSpec
	Static bool // true for static properties, false for instance properties
	Flags  int  // Property flags (writable, enumerable, configurable)
}

// =============================================================================
// PROPERTY FLAGS CONSTANTS
// =============================================================================

// Property flags constants matching QuickJS - unchanged
const (
	PropertyConfigurable = 1 << 0 // JS_PROP_CONFIGURABLE
	PropertyWritable     = 1 << 1 // JS_PROP_WRITABLE
	PropertyEnumerable   = 1 << 2 // JS_PROP_ENUMERABLE

	// Default property flags (writable, enumerable, configurable)
	PropertyDefault = PropertyConfigurable | PropertyWritable | PropertyEnumerable
)

// =============================================================================
// CLASS BUILDER - FLUENT API FOR BUILDING JAVASCRIPT CLASSES
// =============================================================================

// ClassBuilder provides a fluent API for building JavaScript classes
// Uses builder pattern for easy and readable class definition
// MODIFIED FOR SCHEME C: Now stores complete class definition including instance properties
type ClassBuilder struct {
	name        string
	constructor ClassConstructorFunc // MODIFIED: Uses new signature
	methods     []MethodEntry
	accessors   []AccessorEntry
	properties  []PropertyEntry // Properties field (both static and instance)
}

// NewClassBuilder creates a new ClassBuilder with the specified name
// This is the entry point for building JavaScript classes
func NewClassBuilder(name string) *ClassBuilder {
	return &ClassBuilder{
		name:       name,
		methods:    make([]MethodEntry, 0),
		accessors:  make([]AccessorEntry, 0),
		properties: make([]PropertyEntry, 0),
	}
}

// =============================================================================
// CLASSBUILDER FLUENT API METHODS
// =============================================================================

// Constructor sets the constructor function for the class
// MODIFIED FOR SCHEME C: Now uses new constructor signature
// The constructor function will be called with pre-created instance
func (cb *ClassBuilder) Constructor(fn ClassConstructorFunc) *ClassBuilder {
	cb.constructor = fn
	return cb
}

// Method adds an instance method to the class - unchanged
// Instance methods are called on object instances
func (cb *ClassBuilder) Method(name string, fn ClassMethodFunc) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: false,
		Length: 0,
	})
	return cb
}

// StaticMethod adds a static method to the class - unchanged
// Static methods are called on the constructor function itself
func (cb *ClassBuilder) StaticMethod(name string, fn ClassMethodFunc) *ClassBuilder {
	cb.methods = append(cb.methods, MethodEntry{
		Name:   name,
		Func:   fn,
		Static: true,
		Length: 0,
	})
	return cb
}

// Accessor adds a read-write accessor to the class instance - unchanged
// Pass nil for getter to create write-only accessor
// Pass nil for setter to create read-only accessor
func (cb *ClassBuilder) Accessor(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.accessors = append(cb.accessors, AccessorEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: false,
	})
	return cb
}

// StaticAccessor adds a read-write static accessor to the class constructor - unchanged
// Pass nil for getter to create write-only accessor
// Pass nil for setter to create read-only accessor
func (cb *ClassBuilder) StaticAccessor(name string, getter ClassGetterFunc, setter ClassSetterFunc) *ClassBuilder {
	cb.accessors = append(cb.accessors, AccessorEntry{
		Name:   name,
		Getter: getter,
		Setter: setter,
		Static: true,
	})
	return cb
}

// =============================================================================
// PROPERTY API METHODS
// =============================================================================

// Property adds a data property to the class instance - changed to use pointer
// Default flags: writable, enumerable, configurable
// SCHEME C: Instance properties will be bound during instance creation
// Deprecated: Use PropertyValue or PropertyLiteral for declarative, reusable class definitions.
func (cb *ClassBuilder) Property(name string, value *Value, flags ...int) *ClassBuilder {
	var spec ValueSpec
	if value != nil {
		spec = contextValueSpec{value: value}
	}
	return cb.PropertyValue(name, spec, flags...)
}

// PropertyValue adds a data property spec to the class instance.
func (cb *ClassBuilder) PropertyValue(name string, spec ValueSpec, flags ...int) *ClassBuilder {
	propFlags := PropertyDefault
	if len(flags) > 0 {
		propFlags = flags[0]
	}

	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Spec:   spec,
		Static: false, // Instance property
		Flags:  propFlags,
	})
	return cb
}

// PropertyLiteral adds a literal data property to the class instance.
func (cb *ClassBuilder) PropertyLiteral(name string, value interface{}, flags ...int) *ClassBuilder {
	return cb.PropertyValue(name, MarshalSpec{Value: value}, flags...)
}

// StaticProperty adds a data property to the class constructor - changed to use pointer
// Default flags: writable, enumerable, configurable
// Deprecated: Use StaticPropertyValue or StaticPropertyLiteral for declarative, reusable class definitions.
func (cb *ClassBuilder) StaticProperty(name string, value *Value, flags ...int) *ClassBuilder {
	var spec ValueSpec
	if value != nil {
		spec = contextValueSpec{value: value}
	}
	return cb.StaticPropertyValue(name, spec, flags...)
}

// StaticPropertyValue adds a data property spec to the class constructor.
func (cb *ClassBuilder) StaticPropertyValue(name string, spec ValueSpec, flags ...int) *ClassBuilder {
	propFlags := PropertyDefault
	if len(flags) > 0 {
		propFlags = flags[0]
	}

	cb.properties = append(cb.properties, PropertyEntry{
		Name:   name,
		Spec:   spec,
		Static: true, // Static property
		Flags:  propFlags,
	})
	return cb
}

// StaticPropertyLiteral adds a literal data property to the class constructor.
func (cb *ClassBuilder) StaticPropertyLiteral(name string, value interface{}, flags ...int) *ClassBuilder {
	return cb.StaticPropertyValue(name, MarshalSpec{Value: value}, flags...)
}

// Build creates and registers the JavaScript class in the given context
// Returns the constructor function and classID for NewInstance
// ValueSpec entries are captured by shallow snapshot. Do not mutate pointer-based
// ValueSpec implementations after Build, or later constructor calls may observe changes.
// Do not modify the state of passed spec objects after Build.
func (cb *ClassBuilder) Build(ctx *Context) (*Value, uint32) {
	return createClass(ctx, cb)
}

// =============================================================================
// CLASS CREATION IMPLEMENTATION - MODIFIED FOR SCHEME C
// =============================================================================

// validateClassBuilder validates ClassBuilder configuration - unchanged
func validateClassBuilder(builder *ClassBuilder) error {
	if builder == nil {
		return errors.New("class builder is required")
	}
	if builder.name == "" {
		return errors.New("class name is required")
	}
	if builder.constructor == nil {
		return errors.New("constructor function is required")
	}
	for _, method := range builder.methods {
		if method.Func == nil {
			return fmt.Errorf("method function is required: %s", method.Name)
		}
	}
	for _, accessor := range builder.accessors {
		if accessor.Getter == nil && accessor.Setter == nil {
			return fmt.Errorf("accessor requires getter or setter: %s", accessor.Name)
		}
	}
	for _, property := range builder.properties {
		if property.Spec == nil {
			return fmt.Errorf("property value is required: %s", property.Name)
		}
	}
	return nil
}

func cloneClassBuilder(builder *ClassBuilder) *ClassBuilder {
	if builder == nil {
		return nil
	}

	clonedMethods := make([]MethodEntry, len(builder.methods))
	copy(clonedMethods, builder.methods)

	clonedAccessors := make([]AccessorEntry, len(builder.accessors))
	copy(clonedAccessors, builder.accessors)

	clonedProperties := make([]PropertyEntry, len(builder.properties))
	copy(clonedProperties, builder.properties)

	return &ClassBuilder{
		name:        builder.name,
		constructor: builder.constructor,
		methods:     clonedMethods,
		accessors:   clonedAccessors,
		properties:  clonedProperties,
	}
}

// createClass implements the core class creation logic using C layer optimization
// MODIFIED FOR SCHEME C: Now stores entire ClassBuilder and separates static/instance properties
func createClass(ctx *Context, builder *ClassBuilder) (*Value, uint32) {
	// Step 1: Input validation (keep in Go layer for business logic) - unchanged
	if err := validateClassBuilder(builder); err != nil {
		return ctx.ThrowError(err), 0
	}
	snapshot := cloneClassBuilder(builder)

	// Step 2: Go layer manages class name and JSClassDef memory - unchanged
	className := C.CString(snapshot.name)
	defer C.free(unsafe.Pointer(className))

	classDef := &C.JSClassDef{
		class_name: className,
		finalizer:  (*C.JSClassFinalizer)(unsafe.Pointer(C.GoClassFinalizerProxy)),
	}

	// Step 3: Prepare classID variable for C function to allocate internally - unchanged
	var classID C.JSClassID
	var methodIDs []int32
	var accessorIDs []int32
	var methodNames []*C.char
	var accessorNames []*C.char

	// SCHEME C STEP 4: Store entire ClassBuilder in HandleStore (not just constructor)
	// This allows constructor proxy to access both constructor function and instance properties
	constructorID := ctx.handleStore.Store(snapshot)
	cleanupStoredHandlers := func() {
		ctx.handleStore.Delete(constructorID)
		for _, id := range methodIDs {
			ctx.handleStore.Delete(id)
		}
		for _, id := range accessorIDs {
			ctx.handleStore.Delete(id)
		}
	}

	// Step 5: Prepare method entries for C layer - unchanged logic, same implementation
	var cMethods []C.MethodEntry

	for _, method := range snapshot.methods {
		// Store method function in handleStore
		handlerID := ctx.handleStore.Store(method.Func)
		methodIDs = append(methodIDs, handlerID)

		// Convert method name to C string
		methodName := C.CString(method.Name)
		methodNames = append(methodNames, methodName)
		// Note: Don't defer free as C layer needs these strings during binding

		// Determine length parameter
		length := method.Length

		// Convert static flag
		isStatic := 0
		if method.Static {
			isStatic = 1
		}

		// Create C method entry
		cMethods = append(cMethods, C.MethodEntry{
			name:       methodName,
			handler_id: C.int32_t(handlerID),
			length:     C.int(length),
			is_static:  C.int(isStatic),
		})
	}

	// Step 6: Prepare accessor entries for C layer - unchanged logic, same implementation
	var cAccessors []C.AccessorEntry

	for _, accessor := range snapshot.accessors {
		// Convert accessor name to C string
		accessorName := C.CString(accessor.Name)
		accessorNames = append(accessorNames, accessorName)
		// Note: Don't defer free as C layer needs these strings during binding

		var getterID, setterID C.int32_t = 0, 0

		// Store getter function if provided
		if accessor.Getter != nil {
			handlerID := ctx.handleStore.Store(accessor.Getter)
			accessorIDs = append(accessorIDs, handlerID)
			getterID = C.int32_t(handlerID)
		}

		// Store setter function if provided
		if accessor.Setter != nil {
			handlerID := ctx.handleStore.Store(accessor.Setter)
			accessorIDs = append(accessorIDs, handlerID)
			setterID = C.int32_t(handlerID)
		}

		// Convert static flag
		isStatic := 0
		if accessor.Static {
			isStatic = 1
		}

		// Create C accessor entry
		cAccessors = append(cAccessors, C.AccessorEntry{
			name:      accessorName,
			getter_id: getterID,
			setter_id: setterID,
			is_static: C.int(isStatic),
		})
	}

	// SCHEME C STEP 7: Prepare property entries - ONLY STATIC PROPERTIES for CreateClass
	// Instance properties are handled separately by constructor proxy
	var cProperties []C.PropertyEntry
	var staticPropertyNames []*C.char
	type materializedStaticProperty struct {
		spec  ValueSpec
		value *Value
	}
	var materializedStatic []materializedStaticProperty
	defer func() {
		// bridge.c/BindPropertyToObject duplicates property values with JS_DupValue
		// before defining properties, so the original Go-held reference remains ours.
		// Free only non-legacy materialized values allocated for this Build call.
		for _, p := range materializedStatic {
			if p.value == nil || isContextValueSpec(p.spec) {
				continue
			}
			p.value.Free()
		}
	}()
	defer func() {
		for _, name := range methodNames {
			C.free(unsafe.Pointer(name))
		}
		for _, name := range accessorNames {
			C.free(unsafe.Pointer(name))
		}
	}()
	defer func() {
		for _, name := range staticPropertyNames {
			C.free(unsafe.Pointer(name))
		}
	}()

	for _, property := range snapshot.properties {
		// SCHEME C: Only include static properties for CreateClass call
		// Instance properties will be handled by constructor proxy during instance creation
		if property.Static {
			propertyValue, err := materializeValueSpecSafely(ctx, property.Spec)
			if err != nil {
				cleanupStoredHandlers()
				return ctx.ThrowError(fmt.Errorf("invalid property value: %s (materialize error: %v)", property.Name, err)), 0
			}
			if propertyValue != nil && propertyValue.ctx == ctx {
				materializedStatic = append(materializedStatic, materializedStaticProperty{spec: property.Spec, value: propertyValue})
			}
			if propertyValue == nil {
				cleanupStoredHandlers()
				return ctx.ThrowError(fmt.Errorf("invalid property value: %s (materialize returned nil)", property.Name)), 0
			}
			if !propertyValue.belongsTo(ctx) {
				cleanupStoredHandlers()
				return ctx.ThrowError(fmt.Errorf("invalid property value: %s (materialized in a different context)", property.Name)), 0
			}

			// Convert property name to C string
			propertyName := C.CString(property.Name)
			staticPropertyNames = append(staticPropertyNames, propertyName)
			// Note: Don't defer free as C layer needs these strings during binding

			// Create C property entry for static property only
			cProperties = append(cProperties, C.PropertyEntry{
				name:      propertyName,
				value:     propertyValue.ref,
				is_static: C.int(1), // Always static for CreateClass
				flags:     C.int(property.Flags),
			})
		}
		// Instance properties are stored in ClassBuilder and accessed by constructor proxy
	}

	// Step 8: Prepare C array pointers (handle empty arrays) - unchanged logic
	var cMethodsPtr *C.MethodEntry
	var cAccessorsPtr *C.AccessorEntry
	var cPropertiesPtr *C.PropertyEntry

	if len(cMethods) > 0 {
		cMethodsPtr = &cMethods[0]
	}
	if len(cAccessors) > 0 {
		cAccessorsPtr = &cAccessors[0]
	}
	if len(cProperties) > 0 {
		cPropertiesPtr = &cProperties[0]
	}

	// SCHEME C STEP 9: Call C function to create class - only static properties passed
	// Instance properties are handled by constructor proxy, not by CreateClass
	constructor := C.CreateClass(
		ctx.ref,
		&classID,                 // C function allocates class_id internally
		classDef,                 // Go layer manages JSClassDef memory
		C.int32_t(constructorID), // SCHEME C: Store ClassBuilder, not individual constructor
		cMethodsPtr,
		C.int(len(cMethods)),
		cAccessorsPtr,
		C.int(len(cAccessors)),
		cPropertiesPtr,          // SCHEME C: Only static properties
		C.int(len(cProperties)), // SCHEME C: Only static property count
	)

	// Step 10: Error handling - clean up all stored handlers on failure - unchanged logic
	if bool(C.JS_IsException(constructor)) {
		cleanupStoredHandlers()

		// Note: Don't clean up className and classDef - let Go GC handle them
		// The C function failed, so QuickJS isn't using them

		return &Value{ctx: ctx, ref: constructor}, 0
	}

	// SCHEME C STEP 11: Register constructor -> classID mapping for constructor proxy access
	// This enables constructor proxy to extract classID from newTarget
	registerConstructorClassID(ctx, constructor, uint32(classID))

	// Success: className, classDef, and classID are all managed properly
	// - className and classDef: Go GC manages lifetime (QuickJS holds references)
	// - classID: returned via pointer from C function
	// - All handlers: stored in handleStore for proper cleanup
	// - ClassBuilder: stored in handleStore for constructor proxy access
	return &Value{ctx: ctx, ref: constructor}, uint32(classID)
}
