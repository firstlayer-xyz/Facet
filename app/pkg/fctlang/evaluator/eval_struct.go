package evaluator

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"strings"
)


// buildStructDecls creates a lookup map from a program's struct declarations,
// merged with stdlib struct declarations.
func buildStructDecls(prog loader.Program, currentKey string) map[string]*parser.StructDecl {
	decls := make(map[string]*parser.StructDecl)
	if stdSrc := prog.Std(); stdSrc != nil {
		for _, sd := range stdSrc.StructDecls() {
			decls[sd.Name] = sd
		}
	}
	if src := prog.Sources[currentKey]; src != nil {
		for _, sd := range src.StructDecls() {
			decls[sd.Name] = sd
		}
	}
	return decls
}

// ---------------------------------------------------------------------------
// Struct evaluation
// ---------------------------------------------------------------------------

func (e *evaluator) evalStructLit(ex *parser.StructLitExpr, locals map[string]value) (value, error) {
	// Anonymous struct literal: no type name, no declaration lookup
	if ex.TypeName == "" {
		fieldMap := make(map[string]value, len(ex.Fields))
		provided := make(map[string]bool, len(ex.Fields))
		for _, fi := range ex.Fields {
			if provided[fi.Name] {
				return nil, e.errAt(ex.Pos, "duplicate field %q in anonymous struct literal", fi.Name)
			}
			provided[fi.Name] = true
			v, err := e.evalExpr(fi.Value, locals)
			if err != nil {
				return nil, err
			}
			fieldMap[fi.Name] = v
		}
		return &structVal{typeName: "", fields: fieldMap, lib: e.currentLib}, nil
	}

	decl, ok := e.structDecls[ex.TypeName]
	if !ok {
		if dotIdx := strings.IndexByte(ex.TypeName, '.'); dotIdx >= 0 {
			libName := ex.TypeName[:dotIdx]
			bareName := ex.TypeName[dotIdx+1:]
			if lv, lvOk := e.resolveLibraryVar(libName); lvOk {
				for _, sd := range e.prog.Sources[e.prog.Resolve(lv.path)].StructDecls() {
					if sd.Name == bareName {
						decl = sd
						ok = true
						break
					}
				}
			}
		}
	}
	if !ok {
		return nil, e.errAt(ex.Pos, "unknown struct type %q", ex.TypeName)
	}
	// Validate all required fields are provided and no extras
	fieldMap := make(map[string]value, len(decl.Fields))

	provided := make(map[string]bool, len(ex.Fields))
	for _, fi := range ex.Fields {
		if provided[fi.Name] {
			return nil, e.errAt(ex.Pos, "duplicate field %q in %s literal", fi.Name, ex.TypeName)
		}
		provided[fi.Name] = true
		v, err := e.evalExpr(fi.Value, locals)
		if err != nil {
			return nil, err
		}
		fieldMap[fi.Name] = v
	}
	for _, f := range decl.Fields {
		// Validate the field's declared type is accessible in this scope.
		if f.Type != "" && !e.isAccessibleType(f.Type) {
			return nil, e.errAt(ex.Pos, "field %q of %s has unknown type %q", f.Name, ex.TypeName, f.Type)
		}
		if !provided[f.Name] {
			// Use explicit default if available
			if f.Default != nil {
				def, err := e.evalExpr(f.Default, locals)
				if err != nil {
					return nil, err
				}
				fieldMap[f.Name] = def
				continue
			}
			// Use zero value for the field's type (primitive or struct)
			if f.Type != "" {
				if zv, err := zeroValue(f.Type); err == nil {
					fieldMap[f.Name] = zv
					continue
				}
				if sv, err := e.zeroStruct(f.Type); err == nil {
					fieldMap[f.Name] = sv
					continue
				}
			}
			return nil, e.errAt(ex.Pos, "missing field %q in %s literal", f.Name, ex.TypeName)
		}
		// Coerce and type-check the field value
		fieldMap[f.Name] = e.coerceToType(f.Type, fieldMap[f.Name], locals)
		if !checkType(f.Type, fieldMap[f.Name]) {
			return nil, e.errAt(ex.Pos, "field %q of %s must be %s, got %s", f.Name, ex.TypeName, f.Type, typeName(fieldMap[f.Name]))
		}
		// Validate field constraint if present
		if f.Constraint != nil {
			if err := e.validateConstraint(f.Name, f.Constraint, fieldMap[f.Name], locals); err != nil {
				return nil, e.wrapErr(ex.Pos, err)
			}
		}
	}
	// Check for unknown fields
	{
		declFields := make(map[string]bool, len(decl.Fields))
		for _, f := range decl.Fields {
			declFields[f.Name] = true
		}
		for _, fi := range ex.Fields {
			if !declFields[fi.Name] {
				return nil, e.errAt(ex.Pos, "unknown field %q in %s", fi.Name, ex.TypeName)
			}
		}
	}
	// Use bare type name: "T.Thread" → "Thread" — the qualifier is for
	// namespace resolution only; the struct's intrinsic name is always bare.
	bareName := ex.TypeName
	if dotIdx := strings.IndexByte(ex.TypeName, '.'); dotIdx >= 0 {
		bareName = ex.TypeName[dotIdx+1:]
	}
	return &structVal{typeName: bareName, fields: fieldMap, decl: decl, lib: e.currentLib}, nil
}

// resolveFieldDefault returns the default value for a struct field.
// It first checks for an explicit default expression, then falls back to
// the zero value for the field's type.
func (e *evaluator) resolveFieldDefault(f *parser.StructField, locals map[string]value) (value, error) {
	if f.Default != nil {
		return e.evalExpr(f.Default, locals)
	}
	return zeroValue(f.Type)
}

// zeroValue returns the zero value for a type name.
// Primitive types return their zero. Struct types return a struct with all fields zeroed.
func zeroValue(tn string) (value, error) {
	switch tn {
	case "Number":
		return float64(0), nil
	case "Length":
		return length{mm: 0}, nil
	case "Angle":
		return angle{deg: 0}, nil
	case "Bool":
		return false, nil
	case "String":
		return "", nil
	default:
		return nil, fmt.Errorf("no zero value for type %s", tn)
	}
}

// zeroStruct returns a zero-valued struct instance, recursively zeroing all fields.
func (e *evaluator) zeroStruct(typeName string) (value, error) {
	decl, ok := e.structDecls[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown struct type %q", typeName)
	}
	fields := make(map[string]value, len(decl.Fields))
	for _, f := range decl.Fields {
		if f.Default != nil {
			def, err := e.evalExpr(f.Default, e.globals)
			if err != nil {
				return nil, err
			}
			fields[f.Name] = def
		} else if zv, err := zeroValue(f.Type); err == nil {
			fields[f.Name] = zv
		} else if _, ok := e.structDecls[f.Type]; ok {
			sv, err := e.zeroStruct(f.Type)
			if err != nil {
				return nil, err
			}
			fields[f.Name] = sv
		} else {
			return nil, fmt.Errorf("no zero value for field %q of type %s", f.Name, f.Type)
		}
	}
	return &structVal{typeName: typeName, fields: fields, decl: decl, lib: e.currentLib}, nil
}

// coerceAnonymousStruct coerces an anonymous struct to a named struct type.
// The target type's declaration must be reachable from the evaluator; otherwise
// coercion fails with a hard error (no silent duck-typing).
func (e *evaluator) coerceAnonymousStruct(sv *structVal, targetType string, locals map[string]value) error {
	decl, ok := e.structDecls[targetType]
	if !ok {
		// Try qualified name: split into libVar.StructName and search specific library.
		if dotIdx := strings.IndexByte(targetType, '.'); dotIdx >= 0 {
			libName := targetType[:dotIdx]
			bareName := targetType[dotIdx+1:]
			if lv, lvOk := e.resolveLibraryVar(libName); lvOk {
				for _, sd := range e.prog.Sources[e.prog.Resolve(lv.path)].StructDecls() {
					if sd.Name == bareName {
						decl = sd
						sv.lib = lv
						break
					}
				}
			}
		}
		if decl == nil {
			// Search loaded libraries for the defining declaration.
			for libPath := range e.libEvalCache {
				for _, sd := range e.prog.Sources[e.prog.Resolve(libPath)].StructDecls() {
					if sd.Name == targetType {
						decl = sd
						sv.lib = &libRef{path: libPath}
						break
					}
				}
				if decl != nil {
					break
				}
			}
		}
		if decl == nil {
			return fmt.Errorf("cannot coerce anonymous struct to %s: no such struct type in scope", targetType)
		}
	}
	// Check for unknown fields
	declFields := make(map[string]bool, len(decl.Fields))
	for _, f := range decl.Fields {
		declFields[f.Name] = true
	}
	for name := range sv.fields {
		if !declFields[name] {
			return fmt.Errorf("anonymous struct has field %q which is not in %s", name, targetType)
		}
	}
	// Validate declared fields: fill defaults for missing, coerce and type-check provided
	for _, f := range decl.Fields {
		if f.Type != "" && !e.isAccessibleType(f.Type) {
			return fmt.Errorf("field %q of %s has unknown type %q", f.Name, targetType, f.Type)
		}
		v, provided := sv.fields[f.Name]
		if !provided {
			def, err := e.resolveFieldDefault(f, locals)
			if err != nil {
				return fmt.Errorf("anonymous struct missing field %q (required by %s)", f.Name, targetType)
			}
			sv.fields[f.Name] = def
			continue
		}
		v = e.coerceToType(f.Type, unwrap(v), locals)
		sv.fields[f.Name] = v
		if !checkType(f.Type, v) {
			return fmt.Errorf("field %q of %s must be %s, got %s", f.Name, targetType, f.Type, typeName(v))
		}
	}
	// Use the bare type name (strip library qualifier) so method dispatch
	// matches the function's ReceiverType which is always bare.
	if dotIdx := strings.IndexByte(targetType, '.'); dotIdx >= 0 {
		sv.typeName = targetType[dotIdx+1:]
	} else {
		sv.typeName = targetType
	}
	sv.decl = decl
	// Ensure library context is set so method dispatch can find library methods.
	if sv.lib == nil {
		sv.lib = e.currentLib
	}
	return nil
}

func (e *evaluator) evalFieldAccess(ex *parser.FieldAccessExpr, locals map[string]value) (value, error) {
	recv, err := e.evalExpr(ex.Receiver, locals)
	if err != nil {
		return nil, err
	}
	sv, ok := unwrap(recv).(*structVal)
	if !ok {
		return nil, e.errAt(ex.Pos, "cannot access field %q on %s", ex.Field, typeName(recv))
	}
	v, ok := sv.fields[ex.Field]
	if !ok {
		return nil, e.errAt(ex.Pos, "struct %s has no field %q", sv.typeName, ex.Field)
	}
	return v, nil
}

// resolveLibraryVar looks up a variable name in the evaluator's globals and
// returns the corresponding libRef if it is a library. This is used to
// resolve qualified struct type names like "T.Thread".
func (e *evaluator) resolveLibraryVar(name string) (*libRef, bool) {
	if v, ok := e.globals[name]; ok {
		if lv, ok := unwrap(v).(*libRef); ok {
			return lv, true
		}
	}
	return nil, false
}
