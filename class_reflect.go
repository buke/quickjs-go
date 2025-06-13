package quickjs

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// =============================================================================
// LEVEL 1: REFLECTION-BASED AUTO BINDING
// =============================================================================

// ReflectOptions configures automatic class binding behavior
type ReflectOptions struct {
	// IncludePrivate includes private fields/methods in binding (default: false)
	IncludePrivate bool

	// MethodPrefix filters methods by prefix (empty = all methods)
	MethodPrefix string

	// IgnoredMethods lists method names to skip during binding
	IgnoredMethods []string

	// IgnoredFields lists field names to skip during binding
	IgnoredFields []string
}

// ReflectOption configures ReflectOptions using functional options pattern
type ReflectOption func(*ReflectOptions)

// WithIncludePrivate includes private fields and methods in binding
func WithIncludePrivate(include bool) ReflectOption {
	return func(opts *ReflectOptions) {
		opts.IncludePrivate = include
	}
}

// WithMethodPrefix filters methods by name prefix
func WithMethodPrefix(prefix string) ReflectOption {
	return func(opts *ReflectOptions) {
		opts.MethodPrefix = prefix
	}
}

// WithIgnoredMethods specifies method names to skip during binding
func WithIgnoredMethods(methods ...string) ReflectOption {
	return func(opts *ReflectOptions) {
		opts.IgnoredMethods = append(opts.IgnoredMethods, methods...)
	}
}

// WithIgnoredFields specifies field names to skip during binding
func WithIgnoredFields(fields ...string) ReflectOption {
	return func(opts *ReflectOptions) {
		opts.IgnoredFields = append(opts.IgnoredFields, fields...)
	}
}

// BindClass automatically creates and builds a JavaScript class from a Go struct type using reflection.
// This is a convenience method that combines BindClassBuilder and Build.
//
// Example usage:
//
//	constructor, classID, err := ctx.BindClass(&MyStruct{})
//	if err != nil { return err }
//	ctx.Globals().Set("MyStruct", constructor)
func (ctx *Context) BindClass(structType interface{}, options ...ReflectOption) (Value, uint32, error) {
	builder, err := ctx.BindClassBuilder(structType, options...)
	if err != nil {
		return Value{}, 0, err
	}

	// Directly use ClassBuilder.Build, which automatically registers to unified mapping table
	return builder.Build(ctx)
}

// BindClassBuilder automatically creates a ClassBuilder from a Go struct type using reflection.
// Returns a ClassBuilder that can be further customized using chain methods before Build().
//
// Example usage:
//
//	builder, err := ctx.BindClassBuilder(&MyStruct{})
//	if err != nil { return err }
//	constructor, classID, err := builder.Build(ctx)
//
//	// Or with additional customization:
//	builder, err := ctx.BindClassBuilder(&MyStruct{})
//	if err != nil { return err }
//	constructor, classID, err := builder.
//	    StaticMethod("Create", myCreateFunc).
//	    ReadOnlyProperty("version", myVersionGetter).
//	    Build(ctx)
func (ctx *Context) BindClassBuilder(structType interface{}, options ...ReflectOption) (*ClassBuilder, error) {
	// Parse options with defaults
	opts := &ReflectOptions{
		IncludePrivate: false,
		IgnoredMethods: make([]string, 0),
		IgnoredFields:  make([]string, 0),
	}

	for _, option := range options {
		option(opts)
	}

	// Extract reflect.Type from input (simplified function)
	typ, err := getReflectType(structType)
	if err != nil {
		return nil, err
	}

	// Determine class name from type
	className := typ.Name()
	if className == "" {
		return nil, errors.New("cannot determine class name from anonymous type")
	}

	// Build ClassBuilder using reflection analysis
	return buildClassFromReflection(className, typ, opts)
}

// getReflectType extracts reflect.Type from various input types (simplified)
func getReflectType(structType interface{}) (reflect.Type, error) {
	switch v := structType.(type) {
	case reflect.Type:
		// Direct reflect.Type input
		if v.Kind() != reflect.Struct {
			return nil, errors.New("type must be a struct type")
		}
		return v, nil

	default:
		// Interface{} input - analyze the actual value
		typ := reflect.TypeOf(v)
		if typ == nil {
			return nil, errors.New("cannot get type from nil value")
		}

		// Handle pointer to struct
		if typ.Kind() == reflect.Ptr {
			if typ.Elem().Kind() != reflect.Struct {
				return nil, errors.New("value must be a struct or pointer to struct")
			}
			return typ.Elem(), nil
		}

		// Handle direct struct value
		if typ.Kind() != reflect.Struct {
			return nil, errors.New("value must be a struct or pointer to struct")
		}

		return typ, nil
	}
}

// buildClassFromReflection creates a ClassBuilder from reflection analysis
func buildClassFromReflection(className string, typ reflect.Type, opts *ReflectOptions) (*ClassBuilder, error) {
	builder := NewClassBuilder(className)

	// Add default constructor with mixed parameter support
	builder.Constructor(func(ctx *Context, newTarget Value, args []Value) Value {
		// Create new instance of the struct
		instance := reflect.New(typ).Interface()

		// Initialize from constructor arguments using mixed parameter strategy
		if len(args) > 0 {
			if err := initializeFromArgs(instance, args, typ, ctx); err != nil {
				return ctx.ThrowError(fmt.Errorf("constructor initialization failed: %w", err))
			}
		}

		// Use simplified NewInstance (automatic classID retrieval)
		return newTarget.NewInstance(instance)
	})

	// Add properties
	if err := addReflectionProperties(builder, typ, opts); err != nil {
		return nil, fmt.Errorf("failed to add properties: %w", err)
	}

	// Add methods
	if err := addReflectionMethods(builder, typ, opts); err != nil {
		return nil, fmt.Errorf("failed to add methods: %w", err)
	}

	return builder, nil
}

// initializeFromArgs implements mixed parameter constructor strategy
func initializeFromArgs(instance interface{}, args []Value, typ reflect.Type, ctx *Context) error {
	// Smart strategy selection based on argument types
	if len(args) == 1 && args[0].IsObject() && !args[0].IsArray() {
		// Single object argument -> named parameter mode
		return initializeFromObjectArgs(instance, args[0], typ, ctx)
	} else {
		// Multiple arguments or non-object -> positional parameter mode
		return initializeFromPositionalArgs(instance, args, typ, ctx)
	}
}

// initializeFromPositionalArgs initializes struct fields from positional arguments
func initializeFromPositionalArgs(instance interface{}, args []Value, typ reflect.Type, ctx *Context) error {
	val := reflect.ValueOf(instance).Elem() // Dereference pointer to get struct value

	argIndex := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Break if no more arguments
		if argIndex >= len(args) {
			break
		}

		// Set field value from argument using marshal.go's unmarshal logic
		if fieldValue.CanSet() {
			if err := ctx.unmarshal(args[argIndex], fieldValue); err != nil {
				return fmt.Errorf("failed to set field %s: %w", field.Name, err)
			}
			argIndex++
		}
	}

	return nil
}

// initializeFromObjectArgs initializes struct fields from object properties (named parameters)
func initializeFromObjectArgs(instance interface{}, obj Value, typ reflect.Type, ctx *Context) error {
	val := reflect.ValueOf(instance).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JavaScript property name using same logic as marshal.go
		propName, skip := parseFieldTagForProperty(field)
		if skip {
			continue // Field marked with "-" tag
		}
		if propName == "" {
			propName = field.Name // Use field name if no tag
		}

		// Check if object has this property and set field value
		if obj.Has(propName) {
			propValue := obj.Get(propName)
			defer propValue.Free()

			if fieldValue.CanSet() {
				if err := ctx.unmarshal(propValue, fieldValue); err != nil {
					return fmt.Errorf("failed to set field %s from property %s: %w", field.Name, propName, err)
				}
			}
		}
	}

	return nil
}

// parseFieldTagForProperty parses struct field tags for property names
// This reuses the same logic as marshal.go for consistency
func parseFieldTagForProperty(field reflect.StructField) (string, bool) {
	// Check "js" tag first
	if tag := field.Tag.Get("js"); tag != "" {
		if tag == "-" {
			return "", true // skip this field
		}
		// Parse "name,options" format if needed
		if idx := strings.Index(tag, ","); idx != -1 {
			return tag[:idx], false
		}
		return tag, false
	}

	// Check "json" tag as fallback
	if tag := field.Tag.Get("json"); tag != "" {
		if tag == "-" {
			return "", true // skip this field
		}
		// Parse "name,omitempty" format
		if idx := strings.Index(tag, ","); idx != -1 {
			return tag[:idx], false
		}
		return tag, false
	}

	// No tag found, use field name
	return field.Name, false
}

// addReflectionMethods scans struct methods and adds them to the ClassBuilder
func addReflectionMethods(builder *ClassBuilder, typ reflect.Type, opts *ReflectOptions) error {
	// Get pointer type to include pointer receiver methods
	ptrTyp := reflect.PointerTo(typ)

	for i := 0; i < ptrTyp.NumMethod(); i++ {
		method := ptrTyp.Method(i)

		// Apply filtering rules
		if !isExported(method.Name) && !opts.IncludePrivate {
			continue // Skip unexported methods unless explicitly included
		}

		if opts.MethodPrefix != "" && !strings.HasPrefix(method.Name, opts.MethodPrefix) {
			continue // Skip methods that don't match prefix filter
		}

		if contains(opts.IgnoredMethods, method.Name) {
			continue // Skip explicitly ignored methods
		}

		if isSpecialMethod(method.Name) {
			continue // Skip special methods like String, Error, etc.
		}

		// Create method wrapper and add to builder
		methodWrapper := createMethodWrapper(method)
		builder.Method(method.Name, methodWrapper)
	}

	return nil
}

// addReflectionProperties scans struct fields and adds them as properties
func addReflectionProperties(builder *ClassBuilder, typ reflect.Type, opts *ReflectOptions) error {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Skip explicitly ignored fields
		if contains(opts.IgnoredFields, field.Name) {
			continue
		}

		// Get property configuration from struct tags
		propName, skip := parseFieldTagForProperty(field)
		if skip {
			continue // Field marked with "-" tag
		}

		if propName == "" {
			propName = field.Name // Use field name if no tag
		}

		// Create getter and setter functions
		getter := createFieldGetter(field, i)
		setter := createFieldSetter(field, i)

		// Add property to builder (instance properties by default)
		builder.Property(propName, getter, setter)
	}

	return nil
}

// createMethodWrapper creates a ClassMethodFunc wrapper around a reflect.Method
func createMethodWrapper(method reflect.Method) ClassMethodFunc {
	return func(ctx *Context, this Value, args []Value) Value {
		// Get the Go object from the JavaScript instance
		obj, err := this.GetGoObject()
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to get instance data: %w", err))
		}

		// Validate the object type
		if obj == nil {
			return ctx.ThrowError(errors.New("instance data is nil"))
		}

		// Get reflect.Value of the object
		objValue := reflect.ValueOf(obj)
		if !objValue.IsValid() {
			return ctx.ThrowError(errors.New("invalid object value"))
		}

		// Ensure we have a pointer receiver if the method requires it
		if objValue.Kind() != reflect.Ptr && method.Type.In(0).Kind() == reflect.Ptr {
			if objValue.CanAddr() {
				objValue = objValue.Addr()
			} else {
				return ctx.ThrowError(errors.New("cannot take address of object for pointer receiver method"))
			}
		}

		// Validate method exists
		if method.Index >= objValue.NumMethod() {
			return ctx.ThrowError(fmt.Errorf("invalid method index: %d", method.Index))
		}

		// Get the method value
		methodValue := objValue.Method(method.Index)
		if !methodValue.IsValid() {
			return ctx.ThrowError(fmt.Errorf("invalid method value for %s", method.Name))
		}

		// Convert JavaScript arguments to Go method arguments
		methodArgs, err := convertJSArgsToMethodArgs(&method, args, ctx)
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to prepare method arguments: %w", err))
		}

		// Call the method - let it panic if it wants to
		results := methodValue.Call(methodArgs)

		// Convert results back to JavaScript values
		return convertMethodResults(results, ctx)
	}
}

// createFieldGetter creates a getter function for a struct field
func createFieldGetter(field reflect.StructField, fieldIndex int) ClassGetterFunc {
	return func(ctx *Context, this Value) Value {
		// Get the Go object from the JavaScript instance
		obj, err := this.GetGoObject()

		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to get instance data: %w", err))
		}

		// Validate object
		if obj == nil {
			return ctx.ThrowError(errors.New("instance data is nil"))
		}

		// Get field value using reflection
		objValue := reflect.ValueOf(obj)
		if !objValue.IsValid() {
			return ctx.ThrowError(errors.New("invalid object value"))
		}

		if objValue.Kind() == reflect.Ptr {
			objValue = objValue.Elem() // Dereference pointer to access struct
		}

		if fieldIndex >= objValue.NumField() {
			return ctx.ThrowError(fmt.Errorf("invalid field index: %d >= %d", fieldIndex, objValue.NumField()))
		}

		fieldValue := objValue.Field(fieldIndex)
		if !fieldValue.IsValid() {
			return ctx.ThrowError(fmt.Errorf("invalid field value for %s", field.Name))
		}

		// Use Marshal to convert field value to JavaScript value
		if fieldValue.CanInterface() {
			fieldInterface := fieldValue.Interface()
			jsValue, err := ctx.Marshal(fieldInterface)
			if err != nil {
				return ctx.ThrowError(fmt.Errorf("failed to marshal field %s: %w", field.Name, err))
			}
			return jsValue
		} else {
			return ctx.Null()
		}
	}
}

// createFieldSetter creates a setter function for a struct field
func createFieldSetter(field reflect.StructField, fieldIndex int) ClassSetterFunc {
	return func(ctx *Context, this Value, value Value) Value {
		// Get the Go object from the JavaScript instance
		obj, err := this.GetGoObject()
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to get instance data: %w", err))
		}

		// Validate object
		if obj == nil {
			return ctx.ThrowError(errors.New("instance data is nil"))
		}

		// Get field value using reflection
		objValue := reflect.ValueOf(obj)
		if !objValue.IsValid() {
			return ctx.ThrowError(errors.New("invalid object value"))
		}

		if objValue.Kind() == reflect.Ptr {
			objValue = objValue.Elem() // Dereference pointer to access struct
		}

		if fieldIndex >= objValue.NumField() {
			return ctx.ThrowError(fmt.Errorf("invalid field index: %d >= %d", fieldIndex, objValue.NumField()))
		}

		fieldValue := objValue.Field(fieldIndex)
		if !fieldValue.IsValid() {
			return ctx.ThrowError(fmt.Errorf("invalid field value for %s", field.Name))
		}

		if !fieldValue.CanSet() {
			return ctx.ThrowError(fmt.Errorf("field %s is not settable", field.Name))
		}

		// Use Unmarshal to convert JavaScript value to Go value
		tempVar := reflect.New(fieldValue.Type())
		if err := ctx.Unmarshal(value, tempVar.Interface()); err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to unmarshal value for field %s: %w", field.Name, err))
		}

		// Set the field value
		fieldValue.Set(tempVar.Elem())

		// Return the new JavaScript value (safe approach)
		if fieldValue.CanInterface() {
			returnValue, err := ctx.Marshal(fieldValue.Interface())
			if err != nil {
				// If marshal fails, return undefined instead of crashing
				return ctx.Undefined()
			}
			return returnValue
		} else {
			return ctx.Undefined()
		}
	}
}

// convertJSArgsToMethodArgs converts JavaScript arguments to Go reflect.Values for method calls
func convertJSArgsToMethodArgs(method *reflect.Method, args []Value, ctx *Context) ([]reflect.Value, error) {
	methodType := method.Type
	numIn := methodType.NumIn()

	// First argument is the receiver, so we skip it
	numArgs := numIn - 1

	if len(args) > numArgs {
		return nil, fmt.Errorf("too many arguments: expected %d, got %d", numArgs, len(args))
	}

	// Prepare argument slice
	reflectArgs := make([]reflect.Value, numArgs)

	for i := 0; i < numArgs; i++ {
		argType := methodType.In(i + 1) // +1 to skip receiver

		if i < len(args) {
			// Convert JavaScript value to Go value using marshal.go logic
			argValue := reflect.New(argType).Elem()
			if err := ctx.unmarshal(args[i], argValue); err != nil {
				return nil, fmt.Errorf("failed to convert argument %d: %w", i, err)
			}
			reflectArgs[i] = argValue
		} else {
			// Use zero value for missing arguments
			reflectArgs[i] = reflect.Zero(argType)
		}
	}

	return reflectArgs, nil
}

// convertMethodResults converts method return values to JavaScript value
func convertMethodResults(results []reflect.Value, ctx *Context) Value {
	switch len(results) {
	case 0:
		// No return values
		return ctx.Undefined()

	case 1:
		// Single return value - marshal it directly
		jsValue, err := ctx.marshal(results[0])
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to marshal return value: %w", err))
		}
		return jsValue

	default:
		// Multiple return values - create an array
		returnArray := make([]interface{}, len(results))
		for i, result := range results {
			returnArray[i] = result.Interface()
		}

		jsValue, err := ctx.Marshal(returnArray)
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to marshal return values: %w", err))
		}
		return jsValue
	}
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// isExported checks if a name is exported (starts with uppercase letter)
func isExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isSpecialMethod checks if a method name should be skipped during reflection binding
func isSpecialMethod(name string) bool {
	specialMethods := []string{
		"String",      // String() string - handled specially for debugging
		"Error",       // Error() string - for error interface
		"GoString",    // GoString() string - for debugging
		"Format",      // Format() - for fmt package
		"Finalize",    // Finalize() - our cleanup interface
		"MarshalJS",   // MarshalJS() - our marshal interface
		"UnmarshalJS", // UnmarshalJS() - our unmarshal interface
	}

	return contains(specialMethods, name)
}
