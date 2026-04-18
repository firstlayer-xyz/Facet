package evaluator

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"strings"

	"facet/app/pkg/manifold"
)

// copyValue returns a shallow copy of v if it is a struct, otherwise returns v unchanged.
// This gives structs value semantics: assignment copies the struct rather than aliasing it.
func copyValue(v value) value {
	v = unwrap(v)
	if sv, ok := v.(*structVal); ok {
		fields := make(map[string]value, len(sv.fields))
		for k, fv := range sv.fields {
			fields[k] = copyValue(fv)
		}
		return &structVal{
			typeName: sv.typeName,
			fields:   fields,
			decl:     sv.decl,
			lib:      sv.lib,
		}
	}
	return v
}

// inferElemType returns the common element type for a slice of values.
// Returns "" if elements are heterogeneous or empty.
func inferElemType(elems []value) string {
	if len(elems) == 0 {
		return ""
	}
	t := typeName(elems[0])
	for _, el := range elems[1:] {
		if typeName(el) != t {
			return ""
		}
	}
	return t
}

// typeName returns the facet type name for a runtime value.
func typeName(v value) string {
	switch v := v.(type) {
	case *constVal:
		return typeName(v.inner)
	case *constrainedVal:
		return typeName(v.inner)
	case length:
		return "Length"
	case angle:
		return "Angle"
	case *manifold.Solid:
		return "Solid"
	case *manifold.Sketch:
		return "Sketch"
	case bool:
		return "Bool"
	case array:
		if v.elemType != "" {
			return "[]" + v.elemType
		}
		if len(v.elems) > 0 {
			elem0 := typeName(v.elems[0])
			for _, el := range v.elems[1:] {
				if typeName(el) != elem0 {
					return "Array"
				}
			}
			return "[]" + elem0
		}
		return "Array"
	case float64:
		return "Number"
	case string:
		return "String"
	case *libRef:
		return "Library"
	case *structVal:
		if v.typeName == "" {
			return "anonymous struct"
		}
		return v.typeName
	case *functionVal:
		return "Function"
	default:
		return fmt.Sprintf("%T", v)
	}
}

// checkType verifies a value matches the declared type name.
func checkType(declaredType string, v value) bool {
	v = unwrap(v)
	// var accepts any value
	if declaredType == "var" {
		return true
	}
	// fn(...) R accepts a functionVal
	if strings.HasPrefix(declaredType, "fn(") {
		_, ok := v.(*functionVal)
		return ok
	}
	// []var accepts any array
	if declaredType == "[]var" {
		_, ok := v.(array)
		return ok
	}
	// Handle []Type — check v is array and element type matches.
	if strings.HasPrefix(declaredType, "[]") {
		arr, ok := v.(array)
		if !ok {
			return false
		}
		elemType := declaredType[2:]
		// Fast path: if array has an elemType tag, check it directly.
		if arr.elemType != "" && arr.elemType == elemType {
			return true
		}
		// Slow path: check each element individually.
		for _, elem := range arr.elems {
			if !checkType(elemType, elem) {
				return false
			}
		}
		return true
	}
	switch declaredType {
	case "Solid":
		_, ok := v.(*manifold.Solid)
		return ok
	case "Length":
		_, ok := v.(length)
		if !ok {
			_, ok = v.(float64) // bare numbers auto-cast to Length
		}
		return ok
	case "Angle":
		_, ok := v.(angle)
		if !ok {
			_, ok = v.(float64) // bare numbers auto-cast to Angle
		}
		return ok
	case "Vec2", "Vec3":
		sv, ok := v.(*structVal)
		return ok && sv.typeName == declaredType
	case "Sketch":
		_, ok := v.(*manifold.Sketch)
		return ok
	case "Bool":
		_, ok := v.(bool)
		return ok
	case "Number":
		_, ok := v.(float64)
		return ok
	case "String":
		_, ok := v.(string)
		return ok
	case "Array":
		_, ok := v.(array)
		return ok
	default:
		// Check for user-defined struct types
		if sv, ok := v.(*structVal); ok {
			if sv.typeName == declaredType {
				return true
			}
			// Match qualified type: "T.Thread" matches a struct with typeName "Thread"
			if dotIdx := strings.IndexByte(declaredType, '.'); dotIdx >= 0 {
				return sv.typeName == declaredType[dotIdx+1:]
			}
			return false
		}
		return false
	}
}

// coerceToType coerces a value to match a declared type.
//
// Coercion rules (applied at every type boundary — assignments, arguments,
// return values, and struct literal fields):
//
//   - Number → Length:  bare float64 becomes length{mm: n}  (default unit mm)
//   - Number → Angle:   bare float64 becomes angle{deg: n}  (default unit deg)
//   - Anonymous struct → named struct:  fields are validated and defaults filled
//
// If the value already matches the declared type or no coercion applies,
// the original value is returned unchanged.
func (e *evaluator) coerceToType(declType string, v value, locals map[string]value) value {
	v = unwrap(v)
	// var types — no coercion needed
	if declType == "var" || declType == "[]var" {
		return v
	}
	// Handle []Type — coerce each element to the element type.
	if strings.HasPrefix(declType, "[]") {
		arr, ok := v.(array)
		if !ok {
			return v
		}
		elemType := declType[2:]
		changed := false
		newElems := make([]value, len(arr.elems))
		for i, elem := range arr.elems {
			coerced := e.coerceToType(elemType, elem, locals)
			newElems[i] = coerced
			if !changed && !valueEqual(coerced, elem) {
				changed = true
			}
		}
		if changed {
			return array{elems: newElems, elemType: elemType}
		}
		return v
	}
	switch declType {
	case "Length":
		if n, ok := v.(float64); ok {
			return length{mm: n}
		}
	case "Angle":
		if n, ok := v.(float64); ok {
			return angle{deg: n}
		}
	default:
		if sv, ok := v.(*structVal); ok && sv.typeName == "" {
			if err := e.coerceAnonymousStruct(sv, declType, locals); err == nil {
				return sv
			}
		}
	}
	return v
}

// coerceArgs coerces each argument to its declared parameter type.
func (e *evaluator) coerceArgs(fnName string, params []*parser.Param, args map[string]value, locals map[string]value) error {
	for _, param := range params {
		v, ok := args[param.Name]
		if !ok {
			continue
		}
		// Validate the parameter's declared type is accessible.
		if !e.isAccessibleType(param.Type) {
			return fmt.Errorf("%s() parameter %q has unknown type %q", fnName, param.Name, param.Type)
		}
		coerced := e.coerceToType(param.Type, v, locals)
		args[param.Name] = coerced
		if checkType(param.Type, coerced) {
			continue
		}
		// Surface detailed error for anonymous struct coercion failures
		if sv, ok := unwrap(coerced).(*structVal); ok && sv.typeName == "" {
			if err := e.coerceAnonymousStruct(sv, param.Type, locals); err != nil {
				return fmt.Errorf("%s() parameter %q: %s", fnName, param.Name, err)
			}
		}
		return fmt.Errorf("%s() parameter %q must be %s, got %s", fnName, param.Name, param.Type, typeName(coerced))
	}
	return nil
}

// isAccessibleType checks whether a type name is accessible in the current
// evaluator scope.  Built-in types, local/stdlib struct declarations, and
// qualified library types (e.g. "T.Thread") are accessible.  Bare names that
// only exist in a library (e.g. "Thread" without qualification) are NOT.
func (e *evaluator) isAccessibleType(typeName string) bool {
	// Handle []Type — strip prefix and recurse.
	if strings.HasPrefix(typeName, "[]") {
		return e.isAccessibleType(typeName[2:])
	}
	// fn(...) R is always a valid type.
	if strings.HasPrefix(typeName, "fn(") {
		return true
	}
	switch typeName {
	case "", "Solid", "Sketch", "Length", "Angle",
		"Bool", "Number", "String", "Array", "var":
		return true
	default:
		// Local or stdlib struct
		if _, ok := e.structDecls[typeName]; ok {
			return true
		}
		// Qualified library type: "T.Thread" → look up library var "T", find struct "Thread"
		if dotIdx := strings.IndexByte(typeName, '.'); dotIdx >= 0 {
			libName := typeName[:dotIdx]
			bareName := typeName[dotIdx+1:]
			if lv, ok := e.resolveLibraryVar(libName); ok {
				for _, sd := range e.prog.Sources[e.prog.Resolve(lv.path)].StructDecls() {
					if sd.Name == bareName {
						return true
					}
				}
			}
		}
		return false
	}
}

// valueEqual safely compares two values for identity/equality.
// Unlike ==, this does not panic on uncomparable types (array, struct):
// arrays are compared element-wise (structurally), structs are compared
// by pointer identity.
func valueEqual(a, b value) bool {
	switch av := a.(type) {
	case array:
		bv, ok := b.(array)
		if !ok || len(av.elems) != len(bv.elems) {
			return false
		}
		for i := range av.elems {
			if !valueEqual(av.elems[i], bv.elems[i]) {
				return false
			}
		}
		return true
	case *structVal:
		bv, ok := b.(*structVal)
		return ok && av == bv
	default:
		return a == b
	}
}

// extractVec2 extracts x,y mm values from a Vec2 *structVal.
func extractVec2(v value) (x, y float64, ok bool) {
	v = unwrap(v)
	sv, svOk := v.(*structVal)
	if !svOk || sv.typeName != "Vec2" {
		return 0, 0, false
	}
	xl, xOk := unwrap(sv.fields["x"]).(length)
	yl, yOk := unwrap(sv.fields["y"]).(length)
	if !xOk || !yOk {
		return 0, 0, false
	}
	return xl.mm, yl.mm, true
}

// extractVec3 extracts x,y,z mm values from a Vec3 *structVal.
func extractVec3(v value) (x, y, z float64, ok bool) {
	v = unwrap(v)
	sv, svOk := v.(*structVal)
	if !svOk || sv.typeName != "Vec3" {
		return 0, 0, 0, false
	}
	xl, xOk := unwrap(sv.fields["x"]).(length)
	yl, yOk := unwrap(sv.fields["y"]).(length)
	zl, zOk := unwrap(sv.fields["z"]).(length)
	if !xOk || !yOk || !zOk {
		return 0, 0, 0, false
	}
	return xl.mm, yl.mm, zl.mm, true
}

// asNumber extracts a plain float64 from a value (float64 directly, or length via .mm).
func asNumber(v value) (float64, bool) {
	v = unwrap(v)
	switch n := v.(type) {
	case float64:
		return n, true
	case length:
		return n.mm, true
	default:
		return 0, false
	}
}
