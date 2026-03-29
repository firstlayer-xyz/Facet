package checker

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"strings"
)

// inferExpr infers the type of an expression.
// checkTypedArrayElems recursively checks and infers the type of a typed array literal.
// Untyped sub-arrays ([...]) inherit the outer type prefix, adding nesting levels.
// e.g. Number[1,2] → []Number, Number[[1,2],[3,4]] → [][]Number
func (c *checker) checkTypedArrayElems(typeName string, leafType typeInfo, elems []parser.Expr, pos parser.Pos, env *typeEnv) typeInfo {
	if len(elems) == 0 {
		return arrayOf(leafType)
	}
	// Check if first element is a sub-array (determines nesting)
	if inner, ok := elems[0].(*parser.ArrayLitExpr); ok && inner.TypeName == "" {
		// Nested: validate all elements are sub-arrays, recurse into each
		for _, elem := range elems {
			if sub, ok := elem.(*parser.ArrayLitExpr); ok && sub.TypeName == "" {
				c.checkTypedArrayElems(typeName, leafType, sub.Elems, sub.Pos, env)
			} else {
				c.addError(pos, "mixed array and non-array elements in typed array")
			}
		}
		innerType := c.checkTypedArrayElems(typeName, leafType, inner.Elems, inner.Pos, env)
		return arrayOf(innerType)
	}
	// Leaf level: validate each element against the declared type
	for _, elem := range elems {
		et := c.inferExpr(elem, env)
		if et.ft != typeUnknown && leafType.ft != typeUnknown && !c.typeCompatible(leafType, et) {
			c.addError(pos, fmt.Sprintf("typed array %s[]: element must be %s, got %s",
				typeName, leafType.displayName(), et.displayName()))
		}
	}
	return arrayOf(leafType)
}

func (c *checker) inferExpr(expr parser.Expr, env *typeEnv) typeInfo {
	switch ex := expr.(type) {
	case *parser.NumberLit:
		return simple(typeNumber)

	case *parser.BoolLit:
		return simple(typeBool)

	case *parser.StringLit:
		return simple(typeString)

	case *parser.IdentExpr:
		t, ok := env.lookup(ex.Name)
		if !ok {
			c.addErrorSpan(ex.Pos, ex.Pos.Col+len(ex.Name), fmt.Sprintf("undefined variable %q", ex.Name))
			return unknown()
		}
		return t

	case *parser.ArrayLitExpr:
		// Typed array constructor: TypeName[elem, elem, ...]
		if ex.TypeName != "" {
			elemType := c.resolveTypeStr("", ex.TypeName)
			if elemType.ft == typeUnknown {
				// Try as struct type
				if _, ok := c.structDecls[ex.TypeName]; ok {
					elemType = structTI(ex.TypeName)
				} else {
					c.addError(ex.Pos, fmt.Sprintf("unknown type %q in typed array", ex.TypeName))
					return unknown()
				}
			}
			return c.checkTypedArrayElems(ex.TypeName, elemType, ex.Elems, ex.Pos, env)
		}

		// Untyped array literal — infer element type from elements
		if len(ex.Elems) == 0 {
			return simple(typeArray)
		}

		// Check for anonymous struct literals without context.
		// If we have a top-down returnElemType (e.g. from fn Foo() []Point), use it
		// to coerce the array instead of erroring.
		hasAnonStruct := false
		for _, elem := range ex.Elems {
			if sl, ok := elem.(*parser.StructLitExpr); ok && sl.TypeName == "" {
				hasAnonStruct = true
				break
			}
		}
		if hasAnonStruct {
			if c.returnElemType.ft != typeUnknown {
				// Coerce using the declared return element type
				typeName := c.returnElemType.structName
				if typeName == "" {
					typeName = c.returnElemType.displayName()
				}
				return c.checkTypedArrayElems(typeName, c.returnElemType, ex.Elems, ex.Pos, env)
			}
			c.addError(ex.Pos, "cannot infer type of {} in array; use []Type[...] to specify")
			return unknown()
		}

		// Check if first element is a sub-array (nested array inference)
		if inner, ok := ex.Elems[0].(*parser.ArrayLitExpr); ok && inner.TypeName == "" {
			// Nested: validate all elements are sub-arrays, infer inner type
			for _, elem := range ex.Elems {
				if sub, ok := elem.(*parser.ArrayLitExpr); ok && sub.TypeName == "" {
					c.inferExpr(sub, env)
				} else {
					c.addError(ex.Pos, "mixed array and non-array elements")
					return unknown()
				}
			}
			innerType := c.inferExpr(inner, env)
			return arrayOf(innerType)
		}

		// Infer common element type
		var commonElem typeInfo
		hasCommon := false
		for _, elem := range ex.Elems {
			et := c.inferExpr(elem, env)
			if et.ft == typeUnknown {
				continue
			}
			if !hasCommon {
				commonElem = et
				hasCommon = true
				continue
			}
			if c.typeInfoEqual(commonElem, et) {
				continue
			}
			// Number→Length coercion: promote to Length
			if (commonElem.ft == typeNumber && et.ft == typeLength) ||
				(commonElem.ft == typeLength && et.ft == typeNumber) {
				commonElem = simple(typeLength)
				continue
			}
			// Both arrays but different element types — demote to untyped array
			if commonElem.ft == typeArray && et.ft == typeArray {
				commonElem = simple(typeArray)
				continue
			}
			c.addError(ex.Pos, fmt.Sprintf("array has mixed types (%s, %s); use []Type[...] to specify the element type",
				commonElem.displayName(), et.displayName()))
			return unknown()
		}
		if hasCommon {
			return arrayOf(commonElem)
		}
		return simple(typeArray)

	case *parser.RangeExpr:
		st := c.inferExpr(ex.Start, env)
		et := c.inferExpr(ex.End, env)
		if st.ft != typeUnknown && st.ft != typeNumber && st.ft != typeLength {
			c.addError(ex.Pos, fmt.Sprintf("range start must be a Number, got %s", st.displayName()))
		}
		if et.ft != typeUnknown && et.ft != typeNumber && et.ft != typeLength {
			c.addError(ex.Pos, fmt.Sprintf("range end must be a Number, got %s", et.displayName()))
		}
		if ex.Step != nil {
			stept := c.inferExpr(ex.Step, env)
			if stept.ft != typeUnknown && stept.ft != typeNumber && stept.ft != typeLength {
				c.addError(ex.Pos, fmt.Sprintf("range step must be a Number, got %s", stept.displayName()))
			}
		}
		// Determine element type from start/end
		if st.ft == typeLength || et.ft == typeLength {
			return arrayOf(simple(typeLength))
		}
		return arrayOf(simple(typeNumber))

	case *parser.LibExpr:
		return simple(typeLibrary)

	case *parser.UnaryExpr:
		operandType := c.inferExpr(ex.Operand, env)
		if operandType.ft == typeUnknown {
			return unknown()
		}
		// Check operator function map first
		if ret, ok := c.opMap[opMapKey{op: ex.Op, leftType: operandType.displayName()}]; ok {
			return ret
		}
		switch ex.Op {
		case "-":
			if operandType.ft == typeNumber || operandType.ft == typeLength || operandType.ft == typeAngle {
				return operandType
			}
			c.addError(ex.Pos, fmt.Sprintf("unary minus not supported on %s", operandType.displayName()))
			return unknown()
		case "!":
			if operandType.ft != typeBool {
				c.addError(ex.Pos, fmt.Sprintf("operator ! requires Bool, got %s", operandType.displayName()))
				return unknown()
			}
			return simple(typeBool)
		default:
			return unknown()
		}

	case *parser.BinaryExpr:
		return c.inferBinaryOp(ex, env)

	case *parser.CallExpr:
		return c.checkCall(ex, env)

	case *parser.BuiltinCallExpr:
		return c.checkBuiltinCall(ex, env)

	case *parser.MethodCallExpr:
		return c.checkMethodCall(ex, env)

	case *parser.ForYieldExpr:
		childEnv := env.child()
		var yieldType typeInfo
		for _, clause := range ex.Clauses {
			iterType := c.inferExpr(clause.Iter, env)
			if iterType.ft != typeUnknown && iterType.ft != typeArray {
				c.addError(ex.Pos, fmt.Sprintf("for-yield: expected Array to iterate over, got %s", iterType.displayName()))
			}
			if clause.Index != "" {
				childEnv.set(clause.Index, simple(typeNumber))
			}
			// Set loop variable to element type if known
			if iterType.ft == typeArray && iterType.elem != nil {
				childEnv.set(clause.Var, *iterType.elem)
			} else {
				childEnv.set(clause.Var, unknown())
			}
		}
		// Infer yield type from body
		for _, stmt := range ex.Body {
			if ys, ok := stmt.(*parser.YieldStmt); ok {
				yt := c.inferExpr(ys.Value, childEnv)
				if yt.ft != typeUnknown {
					yieldType = yt
				}
			} else {
				// Walk other stmts for side-effect checking
				c.checkStmts([]parser.Stmt{stmt}, childEnv)
			}
		}
		if yieldType.ft != typeUnknown {
			return arrayOf(yieldType)
		}
		return simple(typeArray)

	case *parser.FoldExpr:
		iterType := c.inferExpr(ex.Iter, env)
		if iterType.ft != typeUnknown && iterType.ft != typeArray {
			c.addError(ex.Pos, fmt.Sprintf("fold: expected Array to iterate over, got %s", iterType.displayName()))
		}
		childEnv := env.child()
		// Set acc/elem vars to element type if known
		elemType := unknown()
		if iterType.ft == typeArray && iterType.elem != nil {
			elemType = *iterType.elem
		}
		childEnv.set(ex.AccVar, elemType)
		childEnv.set(ex.ElemVar, elemType)
		c.checkStmts(ex.Body, childEnv)
		// Fold returns the accumulator, which has the same type as the elements
		return elemType

	case *parser.StructLitExpr:
		// Anonymous struct literal: infer field types, defer validation to runtime
		if ex.TypeName == "" {
			for _, fi := range ex.Fields {
				c.inferExpr(fi.Value, env)
			}
			return simple(typeStruct)
		}
		decl, ok := c.structDecls[ex.TypeName]
		if !ok {
			c.addError(ex.Pos, fmt.Sprintf("unknown struct type %q", ex.TypeName))
			return unknown()
		}

		// Named fields only
		declFields := make(map[string]string, len(decl.Fields))
		for _, f := range decl.Fields {
			declFields[f.Name] = f.Type
		}
		provided := make(map[string]bool, len(ex.Fields))
		for _, fi := range ex.Fields {
			if provided[fi.Name] {
				c.addError(ex.Pos, fmt.Sprintf("duplicate field %q in %s literal", fi.Name, ex.TypeName))
				continue
			}
			provided[fi.Name] = true
			valType := c.inferExpr(fi.Value, env)
			expectedTypeName, exists := declFields[fi.Name]
			if !exists {
				c.addError(ex.Pos, fmt.Sprintf("unknown field %q in %s", fi.Name, ex.TypeName))
				continue
			}
			expectedType := c.resolveTypeStr(ex.TypeName, expectedTypeName)
			if valType.ft != typeUnknown && expectedType.ft != typeUnknown && !c.typeCompatible(expectedType, valType) {
				c.addError(ex.Pos, fmt.Sprintf("field %q of %s must be %s, got %s",
					fi.Name, ex.TypeName, expectedTypeName, valType.displayName()))
			}
		}
		// Check for missing fields — allowed if field has a default or a typed zero value
		for _, f := range decl.Fields {
			if !provided[f.Name] && f.Default == nil && f.Type == "" {
				c.addError(ex.Pos, fmt.Sprintf("missing field %q in %s literal", f.Name, ex.TypeName))
			}
		}
		return structTI(ex.TypeName)

	case *parser.IndexExpr:
		recvType := c.inferExpr(ex.Receiver, env)
		idxType := c.inferExpr(ex.Index, env)
		if recvType.ft != typeUnknown && recvType.ft != typeArray {
			c.addError(ex.Pos, fmt.Sprintf("cannot index %s (expected Array)", recvType.displayName()))
		}
		if idxType.ft != typeUnknown && idxType.ft != typeNumber {
			c.addError(ex.Pos, fmt.Sprintf("index must be a Number, got %s", idxType.displayName()))
		}
		// Return element type if known
		if recvType.ft == typeArray && recvType.elem != nil {
			return *recvType.elem
		}
		return unknown()

	case *parser.SliceExpr:
		recvType := c.inferExpr(ex.Receiver, env)
		if ex.Start != nil {
			st := c.inferExpr(ex.Start, env)
			if st.ft != typeUnknown && st.ft != typeNumber {
				c.addError(ex.Pos, fmt.Sprintf("slice start must be a Number, got %s", st.displayName()))
			}
		}
		if ex.End != nil {
			et := c.inferExpr(ex.End, env)
			if et.ft != typeUnknown && et.ft != typeNumber {
				c.addError(ex.Pos, fmt.Sprintf("slice end must be a Number, got %s", et.displayName()))
			}
		}
		if recvType.ft != typeUnknown && recvType.ft != typeArray {
			c.addError(ex.Pos, fmt.Sprintf("cannot slice %s (expected Array)", recvType.displayName()))
		}
		// Slicing an array returns the same array type
		return recvType

	case *parser.UnitExpr:
		innerType := c.inferExpr(ex.Expr, env)
		if innerType.ft != typeUnknown && innerType.ft != typeNumber {
			c.addError(ex.Pos, fmt.Sprintf("cannot apply unit %q to %s (expected Number)", ex.Unit, innerType.displayName()))
			return unknown()
		}
		if ex.IsAngle {
			return simple(typeAngle)
		}
		return simple(typeLength)

	case *parser.FieldAccessExpr:
		recvType := c.inferExpr(ex.Receiver, env)
		if recvType.ft == typeUnknown {
			return unknown()
		}
		if recvType.ft != typeStruct {
			c.addError(ex.Pos, fmt.Sprintf("cannot access field %q on %s", ex.Field, recvType.displayName()))
			return unknown()
		}
		// Find the struct name from the receiver
		structName := c.resolveStructName(ex.Receiver, env)
		if structName == "" {
			return unknown()
		}
		decl, ok := c.structDecls[structName]
		if !ok {
			return unknown()
		}
		for _, f := range decl.Fields {
			if f.Name == ex.Field {
				return c.resolveTypeStr(structName, f.Type)
			}
		}
		c.addError(ex.Pos, fmt.Sprintf("struct %s has no field %q", bareStructName(structName), ex.Field))
		return unknown()

	case *parser.NamedArg:
		return c.inferExpr(ex.Value, env)

	case *parser.LambdaExpr:
		childEnv := env.child()
		var paramTypes []typeInfo
		for _, p := range ex.Params {
			pt := c.resolveTypeStr("", p.Type)
			if pt.ft == typeUnknown && p.Type != "" {
				if _, ok := c.structDecls[p.Type]; ok {
					pt = structTI(p.Type)
				}
			}
			childEnv.set(p.Name, pt)
			paramTypes = append(paramTypes, pt)
		}
		retType := c.checkStmts(ex.Body, childEnv)
		var ret *typeInfo
		if ex.ReturnType != "" {
			r := c.resolveTypeStr("", ex.ReturnType)
			ret = &r
		} else if retType.ft != typeUnknown {
			ret = &retType
		}
		return funcTI(paramTypes, ret)

	default:
		return unknown()
	}
}

// resolveTypeStr resolves a type string in the context of a struct type.
// Handles built-in types, array types ([]Type), and struct types.
func (c *checker) resolveTypeStr(parentStruct, typeName string) typeInfo {
	// Handle array prefix
	if strings.HasPrefix(typeName, "[]") {
		elemStr := typeName[2:]
		elemTI := c.resolveTypeStr(parentStruct, elemStr)
		return arrayOf(elemTI)
	}
	// Handle function types
	if strings.HasPrefix(typeName, "fn(") {
		return parseFuncTypeInfo(typeName)
	}
	ft := typeFromName(typeName)
	if ft != typeUnknown {
		return simple(ft)
	}
	// Try as struct type (exact match first)
	if _, ok := c.structDecls[typeName]; ok {
		return structTI(typeName)
	}
	// Try qualifying with parent struct's library prefix
	if qn := c.qualifyStructType(parentStruct, typeName); qn != "" {
		return structTI(qn)
	}
	// Try all known library prefixes as a fallback for transitive library types
	for prefix := range c.libVarToPath {
		qualified := prefix + "." + typeName
		if _, ok := c.structDecls[qualified]; ok {
			return structTI(qualified)
		}
	}
	return unknown()
}

// resolveStructName tries to determine the struct type name of an expression.
func (c *checker) resolveStructName(expr parser.Expr, env *typeEnv) string {
	switch ex := expr.(type) {
	case *parser.IdentExpr:
		if t, ok := env.lookup(ex.Name); ok && t.ft == typeStruct && t.structName != "" {
			return t.structName
		}
	case *parser.StructLitExpr:
		return ex.TypeName
	case *parser.CallExpr:
		// Check user functions for return type matching struct
		for _, fn := range c.prog.Sources[c.currentSrcKey].Functions() {
			if fn.Name == ex.Name && fn.ReceiverType == "" {
				if _, ok := c.structDecls[fn.ReturnType]; ok {
					return fn.ReturnType
				}
			}
		}
		// Check stdlib functions
		for _, fn := range c.stdFuncs {
			if fn.Name == ex.Name {
				if _, ok := c.structDecls[fn.ReturnType]; ok {
					return fn.ReturnType
				}
			}
		}
		// Check inferred return struct names for unannotated functions
		if sn, ok := c.inferredReturnStructs[ex.Name]; ok {
			return sn
		}
	case *parser.MethodCallExpr:
		recvType := c.inferExpr(ex.Receiver, env)
		// Handle library method calls
		if recvType.ft == typeLibrary {
			if ident, ok := ex.Receiver.(*parser.IdentExpr); ok {
				if libPath, ok := c.libVarToPath[ident.Name]; ok {
					libSrc := c.prog.Sources[c.prog.Resolve(libPath)]
					if libSrc != nil {
						for _, fn := range libSrc.Functions() {
							if fn.Name == ex.Method && fn.ReceiverType == "" {
								if fn.ReturnType != "" && typeFromName(fn.ReturnType) == typeUnknown {
									// If return type is already qualified (e.g. "K.Knurl"
									// from a transitive library), re-qualify with the
									// current library prefix if found, else use bare name.
									if strings.ContainsRune(fn.ReturnType, '.') {
										bare := bareStructName(fn.ReturnType)
										requalified := ident.Name + "." + bare
										if _, ok := c.structDecls[requalified]; ok {
											return requalified
										}
										return bare
									}
									return ident.Name + "." + fn.ReturnType
								}
							}
						}
						// Fallback: method name matches a struct type in this library
						qualified := ident.Name + "." + ex.Method
						if _, ok := c.structDecls[qualified]; ok {
							return qualified
						}
					}
				}
			}
			return ""
		}
		// Check stdlib methods by receiver type
		recvTypeName := recvType.displayName()
		if recvType.ft == typeStruct {
			recvTypeName = c.resolveStructName(ex.Receiver, env)
		}
		if recvTypeName != "" {
			// Check user-defined methods — match receiver type directly
			for _, fn := range c.prog.Sources[c.currentSrcKey].Functions() {
				if fn.ReceiverType == recvTypeName && fn.Name == ex.Method {
					if qn := c.qualifyStructType(recvTypeName, fn.ReturnType); qn != "" {
						return qn
					}
				}
			}
			// Check stdlib/library methods
			if methods, ok := c.stdMethods[recvTypeName]; ok {
				for _, fn := range methods {
					if fn.Name == ex.Method {
						if qn := c.qualifyStructType(recvTypeName, fn.ReturnType); qn != "" {
							return qn
						}
					}
				}
			}
		}
	case *parser.FieldAccessExpr:
		parentStruct := c.resolveStructName(ex.Receiver, env)
		if parentStruct == "" {
			return ""
		}
		decl, ok := c.structDecls[parentStruct]
		if !ok {
			return ""
		}
		for _, f := range decl.Fields {
			if f.Name == ex.Field {
				if qn := c.qualifyStructType(parentStruct, f.Type); qn != "" {
					return qn
				}
			}
		}
	}
	return ""
}
