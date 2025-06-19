package quickjs

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// =============================================================================
// LEVEL 1: REFLECTION-BASED AUTO BINDING
// =============================================================================

// ReflectOptions configures automatic class binding behavior
type ReflectOptions struct {
	// MethodPrefix filters methods by prefix (empty = all methods)
	MethodPrefix string

	// IgnoredMethods lists method names to skip during binding
	IgnoredMethods []string

	// IgnoredFields lists field names to skip during binding
	IgnoredFields []string
}

// ReflectOption configures ReflectOptions using functional options pattern
type ReflectOption func(*ReflectOptions)

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
// Only exported fields and methods are bound to maintain Go encapsulation principles.
//
// Example usage:
//
//	constructor, classID, err := ctx.BindClass(&MyStruct{})
//	if err != nil { return err }
//	ctx.Globals().Set("MyStruct", constructor)
func (ctx *Context) BindClass(structType interface{}, options ...ReflectOption) (*Value, uint32, error) {
	builder, err := ctx.BindClassBuilder(structType, options...)
	if err != nil {
		return nil, 0, err
	}

	return builder.Build(ctx)
}

// BindClassBuilder automatically creates a ClassBuilder from a Go struct type using reflection.
// Returns a ClassBuilder that can be further customized using chain methods before Build().
// Only exported fields and methods are analyzed to maintain Go encapsulation principles.
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
//	    ReadOnlyAccessor("version", myVersionGetter).
//	    Build(ctx)
func (ctx *Context) BindClassBuilder(structType interface{}, options ...ReflectOption) (*ClassBuilder, error) {
	// Parse options with defaults (zero values are appropriate defaults)
	opts := &ReflectOptions{}
	for _, option := range options {
		option(opts)
	}

	// Extract reflect.Type from input
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
	return buildClassFromReflection(ctx, className, typ, opts)
}

// getReflectType extracts reflect.Type from various input types
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
// MODIFIED FOR SCHEME C: Constructor signature changed to receive instance and return Go object
func buildClassFromReflection(ctx *Context, className string, typ reflect.Type, opts *ReflectOptions) (*ClassBuilder, error) {
	builder := NewClassBuilder(className)

	// SCHEME C: Modified constructor with new signature
	// Constructor now receives pre-created instance and returns Go object to associate
	builder.Constructor(func(ctx *Context, instance *Value, args []*Value) (interface{}, error) {
		// Create new Go object instance
		goObject := reflect.New(typ).Interface()

		// Initialize from constructor arguments using mixed parameter strategy
		if len(args) > 0 {
			if err := initializeFromArgs(goObject, args, typ, ctx); err != nil {
				return nil, fmt.Errorf("constructor initialization failed: %w", err)
			}
		}

		// SCHEME C: Return Go object for automatic association with instance
		// The framework handles accessor-based synchronization automatically
		return goObject, nil
	})

	// Add accessors (only exported fields) - unchanged
	addReflectionAccessors(builder, typ, opts)

	// Add methods (only exported methods) - unchanged
	addReflectionMethods(builder, typ, opts)

	return builder, nil
}

// initializeFromArgs implements mixed parameter constructor strategy
func initializeFromArgs(instance interface{}, args []*Value, typ reflect.Type, ctx *Context) error {
	// Smart strategy: single object argument uses named parameters
	if len(args) == 1 && args[0].IsObject() && !args[0].IsArray() {
		return initializeFromObjectArgs(instance, args[0], typ, ctx)
	}

	// Otherwise use positional parameters
	return initializeFromPositionalArgs(instance, args, typ, ctx)
}

// initializeFromPositionalArgs initializes struct fields from positional arguments
func initializeFromPositionalArgs(instance interface{}, args []*Value, typ reflect.Type, ctx *Context) error {
	val := reflect.ValueOf(instance).Elem() // Dereference pointer to get struct value

	argIndex := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Break if no more arguments
		if argIndex >= len(args) {
			break
		}

		fieldValue := val.Field(i)
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
func initializeFromObjectArgs(instance interface{}, obj *Value, typ reflect.Type, ctx *Context) error {
	val := reflect.ValueOf(instance).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Use unified tag parsing from marshal.go
		tagInfo := parseFieldTag(field)
		if tagInfo.Skip {
			continue // Field marked with "-" tag
		}

		// Check if object has this property and set field value
		if obj.Has(tagInfo.Name) {
			propValue := obj.Get(tagInfo.Name)
			defer propValue.Free()

			fieldValue := val.Field(i)
			if fieldValue.CanSet() {
				if err := ctx.unmarshal(propValue, fieldValue); err != nil {
					return fmt.Errorf("failed to set field %s from property %s: %w", field.Name, tagInfo.Name, err)
				}
			}
		}
	}

	return nil
}

// addReflectionMethods scans struct methods and adds them to the ClassBuilder
// Only exported methods are processed due to Go reflection limitations
func addReflectionMethods(builder *ClassBuilder, typ reflect.Type, opts *ReflectOptions) {
	// Get pointer type to include pointer receiver methods
	ptrTyp := reflect.PointerTo(typ)

	for i := 0; i < ptrTyp.NumMethod(); i++ {
		method := ptrTyp.Method(i)

		if shouldSkipMethod(method, opts) {
			continue
		}

		// Create method wrapper and add to builder
		methodWrapper := createMethodWrapper(method)
		builder.Method(method.Name, methodWrapper)
	}
}

// addReflectionAccessors scans struct fields and adds them as accessors
// Only exported fields are processed to maintain Go encapsulation principles
func addReflectionAccessors(builder *ClassBuilder, typ reflect.Type, opts *ReflectOptions) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if shouldSkipField(field, opts) {
			continue
		}

		// Use unified tag parsing from marshal.go
		tagInfo := parseFieldTag(field)
		if tagInfo.Skip {
			continue // Field marked with "-" tag
		}

		// Create getter and setter functions
		getter := createFieldGetter(field, i)
		setter := createFieldSetter(field, i)

		// Add accessor to builder (instance accessors by default)
		builder.Accessor(tagInfo.Name, getter, setter)
	}
}

// =============================================================================
// OPTIMIZED HELPER FUNCTIONS
// =============================================================================

// getValidObjectValue extracts and validates object value from JavaScript instance
func getValidObjectValue(ctx *Context, this *Value) (reflect.Value, error) {
	obj, err := this.GetGoObject()
	if err != nil {
		return reflect.Value{}, fmt.Errorf("failed to get instance data: %w", err)
	}

	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	return objValue, nil
}

// shouldSkipMethod determines if a method should be skipped during binding
func shouldSkipMethod(method reflect.Method, opts *ReflectOptions) bool {
	// Check prefix filter
	if opts.MethodPrefix != "" && !strings.HasPrefix(method.Name, opts.MethodPrefix) {
		return true
	}

	// Check ignored list
	if slices.Contains(opts.IgnoredMethods, method.Name) {
		return true
	}

	// Check special methods
	return isSpecialMethod(method.Name)
}

// shouldSkipField determines if a field should be skipped during binding
func shouldSkipField(field reflect.StructField, opts *ReflectOptions) bool {
	// Only exported fields are bound to maintain Go encapsulation principles
	if !field.IsExported() {
		return true
	}

	// Check ignored list
	return slices.Contains(opts.IgnoredFields, field.Name)
}

// createMethodWrapper creates a ClassMethodFunc wrapper around a reflect.Method
func createMethodWrapper(method reflect.Method) ClassMethodFunc {
	return func(ctx *Context, this *Value, args []*Value) *Value {
		objValue, err := getValidObjectValue(ctx, this)
		if err != nil {
			return ctx.ThrowError(err)
		}

		// Ensure we have a pointer receiver if the method requires it
		if objValue.Kind() != reflect.Ptr && method.Type.In(0).Kind() == reflect.Ptr {
			// In our implementation, objValue is always addressable because it comes
			// from reflect.New().Elem(), so we can safely take its address
			objValue = objValue.Addr()
		}

		// Get the method value
		methodValue := objValue.Method(method.Index)

		// Convert JavaScript arguments to Go method arguments
		methodArgs, err := convertJSArgsToMethodArgs(&method, args, ctx)
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to prepare method arguments: %w", err))
		}

		// Call the method
		results := methodValue.Call(methodArgs)

		// Convert results back to JavaScript values
		return convertMethodResults(results, ctx)
	}
}

// createFieldGetter creates a getter function for a struct field
func createFieldGetter(field reflect.StructField, fieldIndex int) ClassGetterFunc {
	return func(ctx *Context, this *Value) *Value {
		objValue, err := getValidObjectValue(ctx, this)
		if err != nil {
			return ctx.ThrowError(err)
		}

		// In our implementation, fieldIndex is always valid since it comes from
		// the loop in addReflectionAccessors, and fieldValue is always valid
		// for exported fields that we bind
		fieldValue := objValue.Field(fieldIndex)

		jsValue, err := ctx.Marshal(fieldValue.Interface())
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to marshal field %s: %w", field.Name, err))
		}
		return jsValue
	}
}

// createFieldSetter creates a setter function for a struct field
func createFieldSetter(field reflect.StructField, fieldIndex int) ClassSetterFunc {
	return func(ctx *Context, this *Value, value *Value) *Value {
		objValue, err := getValidObjectValue(ctx, this)
		if err != nil {
			return ctx.ThrowError(err)
		}

		// In our implementation, fieldIndex is always valid and fieldValue
		// is always settable for exported fields that we bind
		fieldValue := objValue.Field(fieldIndex)

		// Use Unmarshal to convert JavaScript value to Go value
		tempVar := reflect.New(fieldValue.Type())
		if err := ctx.Unmarshal(value, tempVar.Interface()); err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to unmarshal value for field %s: %w", field.Name, err))
		}

		// Set the field value
		fieldValue.Set(tempVar.Elem())

		// Return the new JavaScript value
		// Note: In practice, if a JavaScript value can be successfully unmarshaled to a Go field,
		// that field value can almost always be marshaled back to JavaScript. The error case
		// is extremely rare and would indicate a deeper issue with the marshal/unmarshal system.
		returnValue, _ := ctx.Marshal(fieldValue.Interface())
		return returnValue
	}
}

// convertJSArgsToMethodArgs converts JavaScript arguments to Go reflect.Values for method calls
func convertJSArgsToMethodArgs(method *reflect.Method, args []*Value, ctx *Context) ([]reflect.Value, error) {
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
func convertMethodResults(results []reflect.Value, ctx *Context) *Value {
	switch len(results) {
	case 0:
		return ctx.Undefined()

	case 1:
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

// specialMethods contains method names that should be skipped during reflection binding
// Using map for O(1) lookup performance instead of slice
var specialMethods = map[string]bool{
	"String":      true, // String() string - handled specially for debugging
	"Error":       true, // Error() string - for error interface
	"GoString":    true, // GoString() string - for debugging
	"Format":      true, // Format() - for fmt package
	"Finalize":    true, // Finalize() - our cleanup interface
	"MarshalJS":   true, // MarshalJS() - our marshal interface
	"UnmarshalJS": true, // UnmarshalJS() - our unmarshal interface
}

// isSpecialMethod checks if a method name should be skipped during reflection binding
func isSpecialMethod(name string) bool {
	return specialMethods[name]
}
