package checker

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"strings"
)

// promoteVarGroupType allows Number → Length/Angle coercion in var generic groups.
// Returns the promoted type and true if coercion is valid, or zero and false otherwise.
func promoteVarGroupType(a, b typeInfo) (typeInfo, bool) {
	isNum := func(t typeInfo) bool { return t.ft == typeNumber }
	isLen := func(t typeInfo) bool { return t.ft == typeLength }
	isAng := func(t typeInfo) bool { return t.ft == typeAngle }
	if isNum(a) && (isLen(b) || isAng(b)) {
		return b, true
	}
	if isNum(b) && (isLen(a) || isAng(a)) {
		return a, true
	}
	return typeInfo{}, false
}

// checkCall checks a function call and returns its inferred return type.
func (c *checker) checkCall(call *parser.CallExpr, env *typeEnv) typeInfo {
	// Infer argument types
	argTypes := make([]typeInfo, len(call.Args))
	for i, arg := range call.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	// Require named arguments for all function calls
	for _, arg := range call.Args {
		if _, ok := arg.(*parser.NamedArg); !ok {
			c.addError(call.Pos, fmt.Sprintf("%s() arguments must be named (e.g. name: value)", call.Name))
			return unknown()
		}
	}

	// Check if the callee is a function-typed variable (e.g., lambda in scope).
	// NOTE: no reference is recorded for this path — the lambda variable
	// doesn't resolve to a *parser.Function, and call.Name isn't visited as an
	// IdentExpr. Recording a ref to the binding site (via env.lookupPos) is a
	// potential Phase-2 enhancement.
	if varType, ok := env.lookup(call.Name); ok && varType.ft == typeFunc {
		// Validate arg count
		if len(argTypes) != len(varType.funcParams) {
			c.addError(call.Pos, fmt.Sprintf("%s() expects %d arguments, got %d",
				call.Name, len(varType.funcParams), len(argTypes)))
		} else {
			for i, expected := range varType.funcParams {
				if expected.ft != typeUnknown && argTypes[i].ft != typeUnknown && !c.typeCompatible(expected, argTypes[i]) {
					c.addError(call.Pos, fmt.Sprintf("%s() argument %d must be %s, got %s",
						call.Name, i+1, expected.displayName(), argTypes[i].displayName()))
				}
			}
		}
		if varType.funcReturn != nil {
			return *varType.funcReturn
		}
		return unknown()
	}

	// Check user-defined functions (type-based overload resolution)
	userCandidates, userFallback := parser.CollectCandidates(
		c.prog.Sources[c.currentSrcKey].Functions(), call.Name, len(argTypes), true)
	if len(userCandidates) == 1 {
		return c.checkFuncArgs(call.Name, call.Pos, userCandidates[0], call.Args, argTypes)
	} else if len(userCandidates) > 1 {
		if fn := c.findOverload(userCandidates, call.Args, argTypes); fn != nil {
			return c.checkFuncArgs(call.Name, call.Pos, fn, call.Args, argTypes)
		} else {
			c.addOverloadError(call.Name, call.Pos, userCandidates, argTypes)
		}
		return unknown()
	}

	// Check stdlib functions (type-based overload resolution)
	stdCandidates, stdFallback := parser.CollectCandidates(c.stdFuncs, call.Name, len(argTypes), false)
	if len(stdCandidates) == 1 {
		return c.checkFuncArgs(call.Name, call.Pos, stdCandidates[0], call.Args, argTypes)
	} else if len(stdCandidates) > 1 {
		if fn := c.findOverload(stdCandidates, call.Args, argTypes); fn != nil {
			return c.checkFuncArgs(call.Name, call.Pos, fn, call.Args, argTypes)
		} else {
			c.addOverloadError(call.Name, call.Pos, stdCandidates, argTypes)
		}
		return unknown()
	}

	// No candidates matched — use fallback for arity error reporting
	fb := userFallback
	if fb == nil {
		fb = stdFallback
	}
	if fb != nil {
		return c.checkFuncArgs(call.Name, call.Pos, fb, call.Args, argTypes)
	}

	c.addError(call.Pos, fmt.Sprintf("unknown function %q", call.Name))
	return unknown()
}

// checkBuiltinCall checks an internal _-prefixed builtin call and returns its inferred return type.
func (c *checker) checkBuiltinCall(call *parser.BuiltinCallExpr, env *typeEnv) typeInfo {
	argTypes := make([]typeInfo, len(call.Args))
	for i, arg := range call.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	// Check builtin signatures
	if sig, ok := builtinSigs[call.Name]; ok {
		if len(argTypes) != len(sig.params) {
			c.addError(call.Pos, fmt.Sprintf("%s() expects %d arguments, got %d",
				call.Name, len(sig.params), len(argTypes)))
			return sig.ret
		}
		for i, expected := range sig.params {
			if argTypes[i].ft != typeUnknown && !c.typeCompatible(expected, argTypes[i]) {
				c.addError(call.Pos, fmt.Sprintf("%s() argument %d must be %s, got %s",
					call.Name, i+1, expected.displayName(), argTypes[i].displayName()))
			}
		}
		// Infer return types for builtins that accept var[] arrays
		switch call.Name {
		case "_hull":
			if len(argTypes) == 1 && argTypes[0].ft == typeArray && argTypes[0].elem != nil {
				switch {
				case argTypes[0].elem.ft == typeSolid:
					return simple(typeSolid)
				case argTypes[0].elem.ft == typeSketch:
					return simple(typeSketch)
				case argTypes[0].elem.ft == typeStruct && argTypes[0].elem.structName == "Vec3":
					return simple(typeSolid)
				}
			}
		case "_union", "_difference", "_intersection":
			if len(argTypes) == 1 && argTypes[0].ft == typeArray && argTypes[0].elem != nil {
				return *argTypes[0].elem
			}
		}
		return sig.ret
	}

	// Special-case polymorphic builtins: _min, _max, _abs, _lerp
	switch call.Name {
	case "_min", "_max":
		if len(argTypes) != 2 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 2 arguments, got %d", call.Name, len(argTypes)))
			return unknown()
		}
		if argTypes[0].ft != argTypes[1].ft && argTypes[0].ft != typeUnknown && argTypes[1].ft != typeUnknown {
			c.addError(call.Pos, fmt.Sprintf("%s() arguments must have the same type, got %s and %s", call.Name, argTypes[0].displayName(), argTypes[1].displayName()))
		}
		return argTypes[0] // return type = first arg type
	case "_abs":
		if len(argTypes) != 1 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 argument, got %d", call.Name, len(argTypes)))
			return unknown()
		}
		return argTypes[0]
	case "_lerp":
		if len(argTypes) != 3 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 3 arguments, got %d", call.Name, len(argTypes)))
			return unknown()
		}
		return argTypes[0]
	case "_string":
		if len(argTypes) != 1 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 argument, got %d", call.Name, len(argTypes)))
			return simple(typeString)
		}
		if argTypes[0].ft != typeUnknown && argTypes[0].ft != typeVar && argTypes[0].ft != typeLength && argTypes[0].ft != typeAngle && argTypes[0].ft != typeNumber && argTypes[0].ft != typeBool && argTypes[0].ft != typeString {
			c.addError(call.Pos, fmt.Sprintf("_string() expects Length, Angle, Number, Bool, or String, got %s", argTypes[0].displayName()))
		}
		return simple(typeString)
	case "_number":
		if len(argTypes) != 1 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 argument, got %d", call.Name, len(argTypes)))
			return simple(typeNumber)
		}
		if argTypes[0].ft != typeUnknown && argTypes[0].ft != typeVar && argTypes[0].ft != typeLength && argTypes[0].ft != typeAngle && argTypes[0].ft != typeNumber && argTypes[0].ft != typeString {
			c.addError(call.Pos, fmt.Sprintf("_number() expects Length, Angle, Number, or String, got %s", argTypes[0].displayName()))
		}
		return simple(typeNumber)
	case "_size":
		if len(argTypes) != 1 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 argument, got %d", call.Name, len(argTypes)))
			return simple(typeNumber)
		}
		if argTypes[0].ft != typeUnknown && argTypes[0].ft != typeArray && argTypes[0].ft != typeString {
			c.addError(call.Pos, fmt.Sprintf("_size() expects Array or String, got %s", argTypes[0].displayName()))
		}
		return simple(typeNumber)
	case "_sphere":
		if len(argTypes) < 1 || len(argTypes) > 2 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 or 2 arguments, got %d", call.Name, len(argTypes)))
		}
		return simple(typeSolid)
	case "_cylinder":
		if len(argTypes) < 3 || len(argTypes) > 4 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 3 or 4 arguments, got %d", call.Name, len(argTypes)))
		}
		return simple(typeSolid)
	case "_circle":
		if len(argTypes) < 1 || len(argTypes) > 2 {
			c.addError(call.Pos, fmt.Sprintf("%s() expects 1 or 2 arguments, got %d", call.Name, len(argTypes)))
		}
		return simple(typeSketch)
	}

	return unknown()
}

// resolveNamedArgs maps named call arguments to their parameter positions,
// validates param names, reports duplicates and missing required params,
// and returns a param-aligned argTypes slice for type checking.
func (c *checker) resolveNamedArgs(name string, pos parser.Pos, fn *parser.Function, callArgs []parser.Expr, argTypes []typeInfo) []typeInfo {
	// If no named args, pass through as-is (internal _-prefixed builtins).
	hasNamed := false
	for _, a := range callArgs {
		if _, ok := a.(*parser.NamedArg); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return argTypes
	}

	paramIdx := make(map[string]int, len(fn.Params))
	for i, p := range fn.Params {
		paramIdx[p.Name] = i
	}

	resolved := make([]typeInfo, len(fn.Params))
	filled := make([]bool, len(fn.Params))

	for i, a := range callArgs {
		na, ok := a.(*parser.NamedArg)
		if !ok {
			continue
		}
		idx, exists := paramIdx[na.Name]
		if !exists {
			c.addError(pos, fmt.Sprintf("%s() has no parameter named %q", name, na.Name))
			continue
		}
		if filled[idx] {
			c.addError(pos, fmt.Sprintf("%s() parameter %q specified multiple times", name, na.Name))
			continue
		}
		resolved[idx] = argTypes[i]
		filled[idx] = true
		c.addRef(na.Pos, DeclLocation{
			Line:       fn.Params[idx].Pos.Line,
			Col:        fn.Params[idx].Pos.Col,
			File:       c.fileForFunction(fn),
			Kind:       "param",
			ReturnType: fn.Params[idx].Type,
		})
	}

	for i, p := range fn.Params {
		if !filled[i] && p.Default == nil {
			c.addError(pos, fmt.Sprintf("%s() missing required argument %q", name, p.Name))
		}
	}

	// Trim unfilled trailing optionals
	last := -1
	for i := len(resolved) - 1; i >= 0; i-- {
		if filled[i] {
			last = i
			break
		}
	}
	return resolved[:last+1]
}

// checkFuncArgs validates arguments against a function definition and returns the return type.
func (c *checker) checkFuncArgs(name string, pos parser.Pos, fn *parser.Function, callArgs []parser.Expr, argTypes []typeInfo) typeInfo {
	// Record reference: callsite -> function declaration. Covers all call paths
	// in checkCall (single/multi/fallback for user and stdlib candidates) and
	// method-call paths in checkMethodCall, which also route here.
	c.addRef(pos, DeclLocation{
		Line:       fn.Pos.Line,
		Col:        fn.Pos.Col,
		File:       c.fileForFunction(fn),
		Kind:       "fn",
		ReturnType: fn.ReturnType,
	})
	// Reorder named arguments to match parameter positions
	argTypes = c.resolveNamedArgs(name, pos, fn, callArgs, argTypes)

	// Count required params
	required := 0
	for _, p := range fn.Params {
		if p.Default == nil {
			required++
		}
	}

	if len(argTypes) < required || len(argTypes) > len(fn.Params) {
		if required == len(fn.Params) {
			c.addError(pos, fmt.Sprintf("%s() expects %d arguments, got %d",
				name, len(fn.Params), len(argTypes)))
		} else {
			c.addError(pos, fmt.Sprintf("%s() expects %d to %d arguments, got %d",
				name, required, len(fn.Params), len(argTypes)))
		}
	} else {
		// Validate provided arg types with top-down coercion for untyped arrays
		for i := 0; i < len(argTypes) && i < len(fn.Params); i++ {
			expected := c.resolveParamType(fn, fn.Params[i].Type)
			if expected.ft == typeUnknown {
				continue
			}
			// Top-down coercion: untyped array arg with unknown type
			// coerces to the expected array type
			if expected.ft == typeArray && expected.elem != nil && argTypes[i].ft == typeUnknown {
				if arr, ok := callArgs[i].(*parser.ArrayLitExpr); ok && arr.TypeName == "" {
					argTypes[i] = expected
					continue
				}
			}
			if argTypes[i].ft != typeUnknown && !c.typeCompatible(expected, argTypes[i]) {
				c.addError(pos, fmt.Sprintf("%s() argument %d (%s) must be %s, got %s",
					name, i+1, fn.Params[i].Name, fn.Params[i].Type, argTypes[i].displayName()))
			}
		}
	}

	// Var-group consistency check: consecutive params with the same var/var[]
	// type form a group. All args in a group must resolve to the same concrete type.
	var firstGroupConcreteType typeInfo
	hasVarParams := false
	if len(argTypes) > 0 && len(fn.Params) > 0 {
		type varGroup struct {
			typeStr   string    // "var" or "[]var"
			indices   []int     // param indices in this group
			first     bool      // is this the first var group?
		}
		var groups []varGroup
		var currentGroup *varGroup
		for i, p := range fn.Params {
			pType := p.Type
			isVar := pType == "var" || pType == "[]var"
			if isVar {
				hasVarParams = true
				if currentGroup != nil && currentGroup.typeStr == pType {
					// Same var type as previous — extend group
					currentGroup.indices = append(currentGroup.indices, i)
				} else {
					// New var group
					groups = append(groups, varGroup{typeStr: pType, indices: []int{i}, first: len(groups) == 0})
					currentGroup = &groups[len(groups)-1]
				}
			} else {
				currentGroup = nil // break the group
			}
		}

		for _, grp := range groups {
			var groupType typeInfo
			for _, idx := range grp.indices {
				if idx >= len(argTypes) {
					continue
				}
				argT := argTypes[idx]
				if argT.ft == typeUnknown {
					continue
				}
				// For var[], extract the element type from the array arg
				concreteT := argT
				if grp.typeStr == "[]var" && argT.ft == typeArray && argT.elem != nil {
					concreteT = *argT.elem
				}
				if groupType.ft == typeUnknown {
					groupType = concreteT
				} else if concreteT.ft != typeUnknown && !c.typeInfoEqual(groupType, concreteT) {
					// Allow Number → Length/Angle coercion in var groups.
					if promoted, ok := promoteVarGroupType(groupType, concreteT); ok {
						groupType = promoted
					} else {
						c.addError(pos, fmt.Sprintf("%s() generic type conflict: arguments have types %s and %s",
							name, groupType.displayName(), concreteT.displayName()))
						break
					}
				}
			}
			if grp.first {
				firstGroupConcreteType = groupType
			}
		}
	}

	// Determine return type — resolve var return from first var group's concrete type
	retType := c.resolveReturnType(fn)
	if hasVarParams && (retType.ft == typeVar || (retType.ft == typeUnknown && (fn.ReturnType == "" || fn.ReturnType == "var"))) {
		if fn.ReturnType == "[]var" {
			return arrayOf(firstGroupConcreteType)
		}
		if firstGroupConcreteType.ft != typeUnknown {
			return firstGroupConcreteType
		}
	}
	return retType
}

// checkLibFuncArgs validates arguments against a library function and returns
// the return type, resolving struct types with library-qualified names.
func (c *checker) checkLibFuncArgs(name string, pos parser.Pos, fn *parser.Function, callArgs []parser.Expr, argTypes []typeInfo) typeInfo {
	// Validate arguments using the shared logic
	c.checkFuncArgs(name, pos, fn, callArgs, argTypes)
	// Override return type: resolve structs that may come from the library
	retType := typeFromNameStr(fn.ReturnType)
	if retType.ft == typeUnknown && fn.ReturnType != "" {
		retType = structTI(fn.ReturnType)
	}
	return retType
}

// qualifyLibReturn qualifies a struct return type with the library variable
// name so that method dispatch can find library-registered methods.
// e.g. structTI("Thread") → structTI("T.Thread") when libVarName is "T".
func (c *checker) qualifyLibReturn(libVarName string, ti typeInfo) typeInfo {
	if ti.ft == typeStruct && ti.structName != "" && !strings.ContainsRune(ti.structName, '.') {
		qualified := libVarName + "." + ti.structName
		if _, ok := c.structDecls[qualified]; ok {
			return structTI(qualified)
		}
	}
	return ti
}

// resolveParamType resolves a parameter type string to typeInfo.
func (c *checker) resolveParamType(fn *parser.Function, typeName string) typeInfo {
	ti := typeFromNameStr(typeName)
	if ti.ft != typeUnknown {
		return ti
	}
	// Check for struct type
	if _, ok := c.structDecls[typeName]; ok {
		return structTI(typeName)
	}
	// Function types like "fn(Vec3) Number" — parse into structured type.
	if strings.HasPrefix(typeName, "fn(") {
		return parseFuncTypeInfo(typeName)
	}
	return unknown()
}

// resolveReturnType resolves a function's return type to typeInfo.
func (c *checker) resolveReturnType(fn *parser.Function) typeInfo {
	if fn.ReturnType == "" {
		// Fallback: use inferred return type for unannotated functions
		if inferred, ok := c.inferredReturns[fn.Name]; ok {
			return inferred
		}
		return unknown()
	}
	// "var" return type → caller resolves from var group
	if fn.ReturnType == "var" {
		return simple(typeVar)
	}
	if fn.ReturnType == "[]var" {
		return arrayOf(simple(typeVar))
	}
	ti := typeFromNameStr(fn.ReturnType)
	if ti.ft != typeUnknown {
		return ti
	}
	// Check for struct type
	if _, ok := c.structDecls[fn.ReturnType]; ok {
		return structTI(fn.ReturnType)
	}
	return unknown()
}

// checkMethodCall checks a method call and returns its inferred return type.
func (c *checker) checkMethodCall(mc *parser.MethodCallExpr, env *typeEnv) typeInfo {
	recvType := c.inferExpr(mc.Receiver, env)

	// Infer arg types
	argTypes := make([]typeInfo, len(mc.Args))
	for i, arg := range mc.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	if recvType.ft == typeUnknown {
		return unknown()
	}

	// Library method calls — resolve from loaded library programs
	if recvType.ft == typeLibrary {
		if ident, ok := mc.Receiver.(*parser.IdentExpr); ok {
			if libPath, ok := c.libVarToPath[ident.Name]; ok {
				libSrc := c.prog.Sources[c.prog.Resolve(libPath)]
				// Collect candidates by name and arg count, then use type-based resolution
				var libCandidates []*parser.Function
				for _, fn := range libSrc.Functions() {
					if fn.Name == mc.Method && fn.ReceiverType == "" && fn.ArgsInRange(len(argTypes)) {
						libCandidates = append(libCandidates, fn)
					}
				}
				if len(libCandidates) == 1 {
					ret := c.checkLibFuncArgs(mc.Method, mc.Pos, libCandidates[0], mc.Args, argTypes)
					return c.qualifyLibReturn(ident.Name, ret)
				} else if len(libCandidates) > 1 {
					if fn := c.findOverload(libCandidates, mc.Args, argTypes); fn != nil {
						ret := c.checkLibFuncArgs(mc.Method, mc.Pos, fn, mc.Args, argTypes)
						return c.qualifyLibReturn(ident.Name, ret)
					}
					c.addOverloadError(mc.Method, mc.Pos, libCandidates, argTypes)
					return unknown()
				}
				// Fallback: if method name matches a struct type, treat as constructor
				qualified := ident.Name + "." + mc.Method
				if _, ok := c.structDecls[qualified]; ok {
					return structTI(qualified)
				}
				c.addError(mc.Pos, fmt.Sprintf("library %s has no function or type %q", ident.Name, mc.Method))
				return unknown()
			}
			// Library variable exists but couldn't be loaded — error already
			// reported during Check() initialization
			return unknown()
		}
		return unknown()
	}

	// Builtin methods that return struct types
	if (recvType.ft == typeSolid || recvType.ft == typeSketch) && mc.Method == "_bounding_box" {
		return structTI("Box")
	}
	if recvType.ft == typeSolid && mc.Method == "_mesh" {
		return structTI("Mesh")
	}

	// Builtin struct methods (Mesh._face_normals, Mesh._vertex_normals)
	if recvType.ft == typeStruct && (mc.Method == "_face_normals" || mc.Method == "_vertex_normals") {
		return arrayOf(structTI("Vec3"))
	}
	// PolyMesh builtin methods
	if recvType.ft == typeStruct {
		switch mc.Method {
		case "_dual", "_ambo", "_kis", "_truncate", "_expand", "_snub",
			"_canonicalize", "_scale_to_radius", "_scale_uniform":
			return structTI("PolyMesh")
		case "_solid":
			return simple(typeSolid)
		case "_display_mesh":
			return unknown() // internal
		}
	}

	// Map typeInfo to receiver type name for stdlib method lookup
	recvTypeName := recvType.displayName()

	// For struct receivers, look up methods in program functions and stdlib
	if recvType.ft == typeStruct {
		structName := c.resolveStructName(mc.Receiver, env)
		if structName == "" {
			return unknown()
		}
		// Check user-defined methods — type-based overload resolution
		var structMethodCandidates []*parser.Function
		for _, fn := range c.prog.Sources[c.currentSrcKey].Functions() {
			if fn.ReceiverType == structName && fn.Name == mc.Method && fn.ArgsInRange(len(argTypes)) {
				structMethodCandidates = append(structMethodCandidates, fn)
			}
		}
		if len(structMethodCandidates) == 1 {
			return c.checkFuncArgs(mc.Method, mc.Pos, structMethodCandidates[0], mc.Args, argTypes)
		} else if len(structMethodCandidates) > 1 {
			if fn := c.findOverload(structMethodCandidates, mc.Args, argTypes); fn != nil {
				return c.checkFuncArgs(mc.Method, mc.Pos, fn, mc.Args, argTypes)
			} else {
				c.addOverloadError(mc.Method, mc.Pos, structMethodCandidates, argTypes)
			}
			return unknown()
		}
		// Check stdlib/library methods for this struct type (type-based overload resolution)
		if methods, ok := c.stdMethods[structName]; ok {
			var stdMethodCandidates []*parser.Function
			for _, fn := range methods {
				if fn.Name == mc.Method && fn.ArgsInRange(len(argTypes)) {
					stdMethodCandidates = append(stdMethodCandidates, fn)
				}
			}
			if len(stdMethodCandidates) == 1 {
				return c.checkFuncArgs(mc.Method, mc.Pos, stdMethodCandidates[0], mc.Args, argTypes)
			} else if len(stdMethodCandidates) > 1 {
				if fn := c.findOverload(stdMethodCandidates, mc.Args, argTypes); fn != nil {
					return c.checkFuncArgs(mc.Method, mc.Pos, fn, mc.Args, argTypes)
				}
				c.addOverloadError(mc.Method, mc.Pos, stdMethodCandidates, argTypes)
				return unknown()
			}
		}
		// Method exists but with wrong arity?
		if c.methodExistsAnyArity(structName, mc.Method) {
			c.addError(mc.Pos, fmt.Sprintf("%s.%s() expects a different number of arguments (got %d)", bareStructName(structName), mc.Method, len(argTypes)))
		} else {
			c.addError(mc.Pos, fmt.Sprintf("%s has no method %q", bareStructName(structName), mc.Method))
		}
		return unknown()
	}

	// Check stdlib methods by receiver type (with type-based overload resolution).
	if methods, ok := c.stdMethods[recvTypeName]; ok {
		var stdMethodCandidates []*parser.Function
		for _, fn := range methods {
			if fn.Name == mc.Method && fn.ArgsInRange(len(argTypes)) {
				stdMethodCandidates = append(stdMethodCandidates, fn)
			}
		}
		if len(stdMethodCandidates) == 1 {
			return c.checkFuncArgs(mc.Method, mc.Pos, stdMethodCandidates[0], mc.Args, argTypes)
		} else if len(stdMethodCandidates) > 1 {
			if fn := c.findOverload(stdMethodCandidates, mc.Args, argTypes); fn != nil {
				return c.checkFuncArgs(mc.Method, mc.Pos, fn, mc.Args, argTypes)
			} else {
				c.addOverloadError(mc.Method, mc.Pos, stdMethodCandidates, argTypes)
			}
			return unknown()
		}
	}

	c.addError(mc.Pos, fmt.Sprintf("%s has no method %q", recvTypeName, mc.Method))
	return unknown()
}

// methodExistsAnyArity returns true if any function with the given receiver type
// and method name exists, regardless of arity. Used for better error messages.
func (c *checker) methodExistsAnyArity(receiverType, method string) bool {
	for _, fn := range c.prog.Sources[c.currentSrcKey].Functions() {
		if fn.ReceiverType == receiverType && fn.Name == method {
			return true
		}
	}
	if methods, ok := c.stdMethods[receiverType]; ok {
		for _, fn := range methods {
			if fn.Name == method {
				return true
			}
		}
	}
	return false
}

// tryConstFloat extracts a constant float64 from a literal Number expression.
func tryConstFloat(expr parser.Expr) (float64, bool) {
	switch e := expr.(type) {
	case *parser.NumberLit:
		return e.Value, true
	case *parser.UnaryExpr:
		if e.Op == "-" {
			if v, ok := tryConstFloat(e.Operand); ok {
				return -v, true
			}
		}
	}
	return 0, false
}

// tryConstLengthMM extracts a constant Length value in mm.
func tryConstLengthMM(expr parser.Expr) (float64, bool) {
	switch e := expr.(type) {
	case *parser.UnitExpr:
		if !e.IsAngle {
			if v, ok := tryConstFloat(e.Expr); ok {
				return v * e.Factor, true
			}
		}
	case *parser.UnaryExpr:
		if e.Op == "-" {
			if v, ok := tryConstLengthMM(e.Operand); ok {
				return -v, true
			}
		}
	}
	return 0, false
}

// tryConstAngleDeg extracts a constant Angle value in degrees.
func tryConstAngleDeg(expr parser.Expr) (float64, bool) {
	switch e := expr.(type) {
	case *parser.UnitExpr:
		if e.IsAngle {
			if v, ok := tryConstFloat(e.Expr); ok {
				return v * e.Factor, true
			}
		}
	case *parser.UnaryExpr:
		if e.Op == "-" {
			if v, ok := tryConstAngleDeg(e.Operand); ok {
				return -v, true
			}
		}
	}
	return 0, false
}

// tryConstString extracts a constant string from a StringLit expression.
func tryConstString(expr parser.Expr) (string, bool) {
	if e, ok := expr.(*parser.StringLit); ok {
		return e.Value, true
	}
	return "", false
}

// checkConstraint validates that a variable's constraint is compatible with its type.
func (c *checker) checkConstraint(g *parser.VarStmt, varType typeInfo, env *typeEnv) {
	switch con := g.Constraint.(type) {
	case *parser.ConstrainedRange:
		// Unit range constraint — validate bounds are numeric and unit matches var type
		st := c.inferExpr(con.Range.Start, env)
		et := c.inferExpr(con.Range.End, env)
		if st.ft != typeUnknown && st.ft != typeNumber {
			c.addError(g.Pos, fmt.Sprintf("constraint range start must be a Number, got %s", st.displayName()))
		}
		if et.ft != typeUnknown && et.ft != typeNumber {
			c.addError(g.Pos, fmt.Sprintf("constraint range end must be a Number, got %s", et.displayName()))
		}
		if con.Range.Step != nil {
			stept := c.inferExpr(con.Range.Step, env)
			if stept.ft != typeUnknown && stept.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range step must be a Number, got %s", stept.displayName()))
			}
		}
		// Check unit matches var type
		if _, isAngle := parser.AngleFactors[con.Unit]; isAngle {
			if varType.ft != typeUnknown && varType.ft != typeAngle {
				c.addError(g.Pos, fmt.Sprintf("constraint unit %q is an angle unit, but variable is %s", con.Unit, varType.displayName()))
			}
		} else if _, isUnit := parser.UnitFactors[con.Unit]; isUnit {
			if varType.ft != typeUnknown && varType.ft != typeLength {
				c.addError(g.Pos, fmt.Sprintf("constraint unit %q is a length unit, but variable is %s", con.Unit, varType.displayName()))
			}
		}
		// Value range check for constant expressions
		if startN, ok1 := tryConstFloat(con.Range.Start); ok1 {
			if endN, ok2 := tryConstFloat(con.Range.End); ok2 {
				if factor, isAngle := parser.AngleFactors[con.Unit]; isAngle {
					lo, hi := startN*factor, endN*factor
					if valDeg, ok := tryConstAngleDeg(g.Value); ok {
						if con.Range.Exclusive {
							if valDeg < lo || valDeg >= hi {
								c.addError(g.Pos, fmt.Sprintf("value %.4g %s is out of range [%g:<%g] %s", valDeg/factor, con.Unit, startN, endN, con.Unit))
							}
						} else if valDeg < lo || valDeg > hi {
							c.addError(g.Pos, fmt.Sprintf("value %.4g %s is out of range [%g:%g] %s", valDeg/factor, con.Unit, startN, endN, con.Unit))
						}
					}
				} else if factor, isUnit := parser.UnitFactors[con.Unit]; isUnit {
					lo, hi := startN*factor, endN*factor
					if valMM, ok := tryConstLengthMM(g.Value); ok {
						if con.Range.Exclusive {
							if valMM < lo || valMM >= hi {
								c.addError(g.Pos, fmt.Sprintf("value %.4g %s is out of range [%g:<%g] %s", valMM/factor, con.Unit, startN, endN, con.Unit))
							}
						} else if valMM < lo || valMM > hi {
							c.addError(g.Pos, fmt.Sprintf("value %.4g %s is out of range [%g:%g] %s", valMM/factor, con.Unit, startN, endN, con.Unit))
						}
					}
				}
			}
		}

	case *parser.RangeExpr:
		st := c.inferExpr(con.Start, env)
		et := c.inferExpr(con.End, env)
		// Range bounds should match var type or be Number
		if varType.ft == typeLength {
			if st.ft != typeUnknown && st.ft != typeLength && st.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range start must be Length or Number, got %s", st.displayName()))
			}
			if et.ft != typeUnknown && et.ft != typeLength && et.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range end must be Length or Number, got %s", et.displayName()))
			}
		} else {
			if st.ft != typeUnknown && st.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range start must be a Number, got %s", st.displayName()))
			}
			if et.ft != typeUnknown && et.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range end must be a Number, got %s", et.displayName()))
			}
		}
		if con.Step != nil {
			stept := c.inferExpr(con.Step, env)
			if stept.ft != typeUnknown && stept.ft != typeNumber {
				c.addError(g.Pos, fmt.Sprintf("constraint range step must be a Number, got %s", stept.displayName()))
			}
		}
		// Validate that constant range bounds form a non-empty range (start <= end)
		if startN, ok1 := tryConstFloat(con.Start); ok1 {
			if endN, ok2 := tryConstFloat(con.End); ok2 {
				if startN > endN {
					c.addError(g.Pos, fmt.Sprintf("constraint range is empty: start %g > end %g", startN, endN))
				}
			}
		}
		if startMM, ok1 := tryConstLengthMM(con.Start); ok1 {
			if endMM, ok2 := tryConstLengthMM(con.End); ok2 {
				if startMM > endMM {
					c.addError(g.Pos, "constraint range is empty: start > end")
				}
			}
		}
		// Value range check — try length bounds first, then plain number
		if startMM, ok1 := tryConstLengthMM(con.Start); ok1 {
			if endMM, ok2 := tryConstLengthMM(con.End); ok2 {
				if valMM, ok3 := tryConstLengthMM(g.Value); ok3 {
					if con.Exclusive {
						if valMM < startMM || valMM >= endMM {
							c.addError(g.Pos, "value is out of range")
						}
					} else if valMM < startMM || valMM > endMM {
						c.addError(g.Pos, "value is out of range")
					}
				}
			}
		} else if startN, ok1 := tryConstFloat(con.Start); ok1 {
			if endN, ok2 := tryConstFloat(con.End); ok2 {
				if valN, ok3 := tryConstFloat(g.Value); ok3 {
					if con.Exclusive {
						if valN < startN || valN >= endN {
							c.addError(g.Pos, fmt.Sprintf("value %g is out of range [%g:<%g]", valN, startN, endN))
						}
					} else if valN < startN || valN > endN {
						c.addError(g.Pos, fmt.Sprintf("value %g is out of range [%g:%g]", valN, startN, endN))
					}
				}
			}
		}

	case *parser.ArrayLitExpr:
		if len(con.Elems) == 0 {
			return // free-form, no validation needed
		}
		// Enum — check all elements are the same type and match the var type
		for i, elem := range con.Elems {
			elemType := c.inferExpr(elem, env)
			if elemType.ft != typeUnknown && varType.ft != typeUnknown && !c.typeCompatible(varType, elemType) {
				c.addError(g.Pos, fmt.Sprintf("constraint enum element %d has type %s, expected %s",
					i+1, elemType.displayName(), varType.displayName()))
			}
		}
		// Value membership check for string enums
		if valStr, ok := tryConstString(g.Value); ok {
			found := false
			for _, elem := range con.Elems {
				if s, ok := tryConstString(elem); ok && s == valStr {
					found = true
					break
				}
			}
			if !found {
				c.addError(g.Pos, fmt.Sprintf("value %q is not in the allowed set", valStr))
			}
		}
		// Value membership check for number enums
		if valN, ok := tryConstFloat(g.Value); ok {
			found := false
			for _, elem := range con.Elems {
				if n, ok := tryConstFloat(elem); ok && n == valN {
					found = true
					break
				}
			}
			if !found {
				c.addError(g.Pos, fmt.Sprintf("value %g is not in the allowed set", valN))
			}
		}
	}
}

// checkParamConstraint validates a function parameter's constraint expression.
// This is a lightweight version of checkConstraint for Param (not VarStmt).
func (c *checker) checkParamConstraint(fn *parser.Function, p *parser.Param, paramType typeInfo, env *typeEnv) {
	switch con := p.Constraint.(type) {
	case *parser.ConstrainedRange:
		st := c.inferExpr(con.Range.Start, env)
		et := c.inferExpr(con.Range.End, env)
		if st.ft != typeUnknown && st.ft != typeNumber {
			c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range start must be a Number, got %s", fn.Name, p.Name, st.displayName()))
		}
		if et.ft != typeUnknown && et.ft != typeNumber {
			c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range end must be a Number, got %s", fn.Name, p.Name, et.displayName()))
		}
	case *parser.RangeExpr:
		st := c.inferExpr(con.Start, env)
		et := c.inferExpr(con.End, env)
		if paramType.ft == typeLength {
			if st.ft != typeUnknown && st.ft != typeLength && st.ft != typeNumber {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range start must be Length or Number, got %s", fn.Name, p.Name, st.displayName()))
			}
			if et.ft != typeUnknown && et.ft != typeLength && et.ft != typeNumber {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range end must be Length or Number, got %s", fn.Name, p.Name, et.displayName()))
			}
		} else {
			if st.ft != typeUnknown && st.ft != typeNumber {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range start must be a Number, got %s", fn.Name, p.Name, st.displayName()))
			}
			if et.ft != typeUnknown && et.ft != typeNumber {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint range end must be a Number, got %s", fn.Name, p.Name, et.displayName()))
			}
		}
	case *parser.ArrayLitExpr:
		for i, elem := range con.Elems {
			elemType := c.inferExpr(elem, env)
			if elemType.ft != typeUnknown && paramType.ft != typeUnknown && !c.typeCompatible(paramType, elemType) {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: constraint enum element %d has type %s, expected %s",
					fn.Name, p.Name, i+1, elemType.displayName(), paramType.displayName()))
			}
		}
	}
}

// Overload resolution scoring constants.
const (
	overloadScoreExact    = 2 // exact type match
	overloadScoreCoercion = 1 // compatible via coercion (e.g. Number→Length/Angle)
)

// findOverload selects the best-matching function using score-based resolution.
// Named args are first mapped to parameter positions: a candidate whose params
// don't include every given NamedArg name is disqualified (matches runtime
// dispatch in resolveOverload). Remaining candidates are then type-scored —
// exact type match scores overloadScoreExact, coercion scores overloadScoreCoercion,
// var match scores 0, mismatch disqualifies. Returns nil if no candidate matches.
func (c *checker) findOverload(candidates []*parser.Function, callArgs []parser.Expr, argTypes []typeInfo) *parser.Function {
	bestScore := -1
	var bestFn *parser.Function

	for _, fn := range candidates {
		// Map call args to parameter positions, disqualifying candidates
		// whose params don't include a given NamedArg name.
		paramIdx := make(map[string]int, len(fn.Params))
		for i, p := range fn.Params {
			paramIdx[p.Name] = i
		}
		reordered := make([]typeInfo, len(fn.Params))
		filled := make([]bool, len(fn.Params))
		invalid := false
		for i, a := range callArgs {
			if na, ok := a.(*parser.NamedArg); ok {
				idx, exists := paramIdx[na.Name]
				if !exists || filled[idx] {
					invalid = true
					break
				}
				reordered[idx] = argTypes[i]
				filled[idx] = true
				continue
			}
			// Positional: fill next unfilled slot.
			slot := -1
			for j := range filled {
				if !filled[j] {
					slot = j
					break
				}
			}
			if slot < 0 {
				invalid = true
				break
			}
			reordered[slot] = argTypes[i]
			filled[slot] = true
		}
		if invalid {
			continue
		}
		score := 0
		valid := true
		for i, p := range fn.Params {
			if !filled[i] {
				continue
			}
			expected := c.resolveParamType(fn, p.Type)
			got := reordered[i]
			if expected.ft == typeUnknown || got.ft == typeUnknown {
				continue
			}
			// Untyped array (nil elem) vs typed array — we don't know the element
			// type, so skip this arg like we skip unknowns.
			if expected.ft == typeArray && got.ft == typeArray && expected.elem != nil && got.elem == nil {
				continue
			}
			if expected.ft == typeVar || (expected.ft == typeArray && expected.elem != nil && expected.elem.ft == typeVar) {
				// var or var[] match — score 0
			} else if c.typeInfoEqual(expected, got) {
				score += overloadScoreExact
			} else if (expected.ft == typeLength || expected.ft == typeAngle) && got.ft == typeNumber {
				score += overloadScoreCoercion
			} else if c.typeCompatible(expected, got) {
				score += overloadScoreCoercion
			} else {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestFn = fn
		}
	}
	return bestFn
}

// addOverloadError reports a descriptive error when no overload matches,
// listing the actual argument types and available candidate signatures.
func (c *checker) addOverloadError(name string, pos parser.Pos, candidates []*parser.Function, argTypes []typeInfo) {
	argNames := make([]string, len(argTypes))
	for i, t := range argTypes {
		argNames[i] = t.displayName()
	}
	msg := fmt.Sprintf("no matching overload for %s(%s); candidates are:", name, strings.Join(argNames, ", "))
	for _, fn := range candidates {
		paramTypes := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			paramTypes[i] = p.Type
		}
		ret := ""
		if fn.ReturnType != "" {
			ret = " " + fn.ReturnType
		}
		msg += fmt.Sprintf("\n  %s(%s)%s", name, strings.Join(paramTypes, ", "), ret)
	}
	c.addError(pos, msg)
}
