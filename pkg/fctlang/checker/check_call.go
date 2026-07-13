package checker

import (
	"facet/pkg/fctlang/parser"
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

// requireNamedArgs enforces the rule that every argument to a user-facing call
// must be named (e.g. `name: value`). It reports the first positional argument
// and returns false so the caller can bail. Shared by the free-function
// (checkCall) and method (checkMethodCall) paths so the rule lives in one place
// and applies to both. Internal _-prefixed builtins use a separate positional
// path (checkBuiltinCall) and never reach here.
func (c *checker) requireNamedArgs(name string, pos parser.Pos, args []parser.Expr) bool {
	for _, arg := range args {
		if _, ok := arg.(*parser.NamedArg); !ok {
			c.addError(pos, fmt.Sprintf("%s() arguments must be named (e.g. name: value)", name))
			return false
		}
	}
	return true
}

// checkCall checks a function call and returns its inferred return type.
func (c *checker) checkCall(call *parser.CallExpr, env *typeEnv) typeInfo {
	// Infer argument types
	argTypes := make([]typeInfo, len(call.Args))
	for i, arg := range call.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	// All user-facing calls require named arguments.
	if !c.requireNamedArgs(call.Name, call.Pos, call.Args) {
		return unknown()
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
		} else if len(varType.funcParamNames) == len(varType.funcParams) {
			// Lambda value: parameter names are known, so resolve each named arg
			// to its parameter position and type-check there. Args are named on
			// this path (requireNamedArgs), and the runtime dispatches by name —
			// so a positional zip would mis-check reordered args.
			nameToIdx := make(map[string]int, len(varType.funcParamNames))
			for i, n := range varType.funcParamNames {
				nameToIdx[n] = i
			}
			seen := make([]bool, len(varType.funcParams))
			for i, arg := range call.Args {
				na, ok := arg.(*parser.NamedArg)
				if !ok {
					continue // requireNamedArgs already reported the positional arg
				}
				idx, ok := nameToIdx[na.Name]
				if !ok {
					c.addError(call.Pos, fmt.Sprintf("%s() has no parameter named %q", call.Name, na.Name))
					continue
				}
				if seen[idx] {
					c.addError(call.Pos, fmt.Sprintf("%s() parameter %q specified multiple times", call.Name, na.Name))
					continue
				}
				seen[idx] = true
				expected := varType.funcParams[idx]
				if expected.ft != typeUnknown && argTypes[i].ft != typeUnknown && !c.typeCompatible(expected, argTypes[i]) {
					c.addError(call.Pos, fmt.Sprintf("%s() parameter %q must be %s, got %s",
						call.Name, na.Name, expected.displayName(), argTypes[i].displayName()))
				}
			}
		} else {
			// Function-typed parameter (fn(...) annotation): no param names in the
			// type, so type-check positionally.
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

	// Check user-defined functions (type-based overload resolution).
	// Pass nArgs=-1 to skip arity pre-filtering: all calls use named args, so
	// findOverload selects the right overload regardless of provided arg count.
	// Arity filtering here would incorrectly exclude overloads when fewer args
	// are provided than the overload requires (e.g. Frustum(r1:..., h:...)
	// with only 2 args should still reach the 3-required r1/r2/h overload).
	userCandidates, _ := parser.CollectCandidates(
		c.prog.Sources[c.currentSrcKey].Functions(), call.Name, -1, true)
	if t, ok := c.resolveCandidates(call.Name, call.Pos, userCandidates, call.Args, argTypes); ok {
		return t
	}

	// Check stdlib functions (type-based overload resolution)
	stdCandidates, _ := parser.CollectCandidates(c.stdFuncs, call.Name, -1, false)
	if t, ok := c.resolveCandidates(call.Name, call.Pos, stdCandidates, call.Args, argTypes); ok {
		return t
	}

	c.addError(call.Pos, fmt.Sprintf("unknown function %q", call.Name))
	return unknown()
}

// resolveCandidates dispatches a call against a set of overload candidates,
// shared by every name-based call site (free functions, stdlib, struct methods).
// It returns (type, true) whenever there was at least one candidate — a unique
// match, the best overload, or a reported overload failure — and (unknown,
// false) only when there were no candidates, so the caller can try the next
// source. The fallback-for-arity path that once followed these blocks was dead:
// the candidate lists here are collected with nArgs=-1, so a non-empty fallback
// implies non-empty candidates, which these branches have already handled.
func (c *checker) resolveCandidates(name string, pos parser.Pos, cands []*parser.Function, args []parser.Expr, argTypes []typeInfo) (typeInfo, bool) {
	switch {
	case len(cands) == 1:
		return c.checkFuncArgs(name, pos, cands[0], args, argTypes), true
	case len(cands) > 1:
		if fn := c.findOverload(cands, args, argTypes); fn != nil {
			return c.checkFuncArgs(name, pos, fn, args, argTypes), true
		}
		c.reportOverloadFailure(name, pos, cands, args, argTypes)
		return unknown(), true
	}
	return unknown(), false
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
		// Number mixed with Length/Angle is valid: the evaluator coerces the bare
		// number to the dimensional type (mathMinMax → coerceNumericArgs), so the
		// result carries the unit. Report only genuinely incompatible mixes.
		if argTypes[0].ft == typeUnknown || argTypes[1].ft == typeUnknown {
			return argTypes[0]
		}
		if argTypes[0].ft == argTypes[1].ft {
			return argTypes[0]
		}
		if promoted, ok := promoteVarGroupType(argTypes[0], argTypes[1]); ok {
			return promoted // e.g. _min(0, 5 mm) is a Length
		}
		c.addError(call.Pos, fmt.Sprintf("%s() arguments must have the same type, got %s and %s", call.Name, argTypes[0].displayName(), argTypes[1].displayName()))
		return argTypes[0]
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

	// The name matched no signature and no special case. If it isn't a registered
	// builtin at all, it's a typo or a stale name the evaluator would reject with
	// "unknown builtin" — flag it here so it fails at compile time. Registered but
	// unsigned builtins fall through as unknown(), bounded by the caller's type.
	if !KnownBuiltins[call.Name] {
		c.addError(call.Pos, fmt.Sprintf("unknown builtin %q", call.Name))
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
		if !filled[i] && p.IsRequired() {
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
		if p.IsRequired() {
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
			expected := c.resolveParamType(fn.Params[i].Type)
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

	// Generic-group consistency check: params declared together as `a, b Any`
	// share a type slot via Param.GroupID and must resolve to the same
	// concrete type. Singletons (GroupID == 0) get their own slot — two
	// independent `Any` params can take different types.
	if len(argTypes) > 0 && len(fn.Params) > 0 {
		type varGroup struct {
			typeStr string // "Any" or "[]Any"
			indices []int  // param indices in this group
		}
		var groups []varGroup
		groupByID := map[int]int{} // Param.GroupID → index into groups (shared decls only)
		for i, p := range fn.Params {
			pType := p.Type
			isVar := pType == "Any" || pType == "[]Any"
			if !isVar {
				continue
			}
			if p.GroupID != 0 {
				if existing, ok := groupByID[p.GroupID]; ok {
					groups[existing].indices = append(groups[existing].indices, i)
					continue
				}
				groupByID[p.GroupID] = len(groups)
			}
			groups = append(groups, varGroup{typeStr: pType, indices: []int{i}})
		}

		for _, grp := range groups {
			isArrayGroup := grp.typeStr == "[]Any"
			var groupType typeInfo
			for _, idx := range grp.indices {
				if idx >= len(argTypes) {
					continue
				}
				argT := argTypes[idx]
				if argT.ft == typeUnknown {
					continue
				}
				// For []var / []Any, extract the element type from the array arg
				concreteT := argT
				if isArrayGroup && argT.ft == typeArray && argT.elem != nil {
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
		}
	}

	// A declared `Any` (or `[]Any`) return stays opaque at the call site —
	// coercion at the consumer's boundary handles the concrete type. We
	// deliberately do not specialize from the first generic group's arg type:
	// that's wrong for functions that index or reshape (e.g.
	// `fn first(v Any) Any { return v[0] }` returns the element type, not v's
	// type).
	switch fn.ReturnType {
	case "Any":
		return unknown()
	case "[]Any":
		return arrayOf(unknown())
	}
	return c.resolveReturnType(fn)
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
func (c *checker) resolveParamType(typeName string) typeInfo {
	// Function types must use the checker-aware path so user-defined struct
	// names inside (Vec3, Mesh, etc.) resolve to typeStruct rather than
	// typeUnknown — the stateless typeFromNameStr can't see structDecls.
	if strings.HasPrefix(typeName, "fn(") {
		return c.resolveFuncTypeStr("", typeName)
	}
	// Everything else shares the return-type resolver, which also peels the
	// postfix `?` for optional params (e.g. `gap Length?`) and resolves array
	// element / struct names.
	return c.resolveType("", typeName)
}

// resolveReturnType resolves a function's return type to typeInfo.
func (c *checker) resolveReturnType(fn *parser.Function) typeInfo {
	if fn.ReturnType == "" {
		// Use the inferred return type keyed by (receiver, name) so an unannotated
		// method resolves to its own slot, not a same-named free function's.
		if inferred, ok := c.inferredReturns[fnKey{fn.ReceiverType, fn.Name}]; ok {
			return inferred
		}
		// Not yet inferred — e.g. this call is in a global initializer, checked
		// before the inferReturnTypes pass. Infer on demand (recursion-guarded) so
		// the global gets a real type instead of unknown.
		return c.inferReturnType(fn)
	}
	return c.resolveType("", fn.ReturnType)
}

// checkMethodCall checks a method call and returns its inferred return type.
func (c *checker) checkMethodCall(mc *parser.MethodCallExpr, env *typeEnv) typeInfo {
	recvType := c.inferExpr(mc.Receiver, env)

	// Infer arg types
	argTypes := make([]typeInfo, len(mc.Args))
	for i, arg := range mc.Args {
		argTypes[i] = c.inferExpr(arg, env)
	}

	// Method calls require named arguments, same as free-function calls.
	if !c.requireNamedArgs(mc.Method, mc.Pos, mc.Args) {
		return unknown()
	}

	if recvType.ft == typeUnknown {
		return unknown()
	}

	// Optional chaining: `opt?.Method(args)`. Unwrap the receiver to its
	// inner T, dispatch the method on T, then wrap the result in `?`.
	// Short-circuit semantics at runtime — if opt is None, the whole
	// chain is None and the call never executes.
	if mc.Optional {
		if recvType.ft != typeOptional {
			c.addError(mc.Pos, fmt.Sprintf("?. operator requires an Optional receiver, got %s", recvType.displayName()))
			return unknown()
		}
		ret := c.checkMethodOnRecvType(mc, env, optionalInnerOr(recvType), argTypes)
		if ret.ft == typeUnknown {
			return unknown()
		}
		return optionalOf(ret)
	}

	return c.checkMethodOnRecvType(mc, env, recvType, argTypes)
}

// checkMethodOnRecvType returns the type of `recvType.Method(args)`. The
// receiver may be a user value or, via optional-chaining, the unwrapped
// inner type of a T?.
func (c *checker) checkMethodOnRecvType(mc *parser.MethodCallExpr, env *typeEnv, recvType typeInfo, argTypes []typeInfo) typeInfo {
	// Optional has no methods. The closed Optional API is `??` for fallback,
	// `== nil` / `!= nil` for presence checks, and `if var x = opt { ... }`
	// to bind the inner value.
	if recvType.ft == typeOptional {
		c.addError(mc.Pos, fmt.Sprintf("%s has no methods; use ?? for fallback, == nil / != nil for presence, or `if var x = opt { ... }` to bind the inner value",
			recvType.displayName()))
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
					c.reportOverloadFailure(mc.Method, mc.Pos, libCandidates, mc.Args, argTypes)
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
		if t, ok := c.resolveCandidates(mc.Method, mc.Pos, structMethodCandidates, mc.Args, argTypes); ok {
			return t
		}
		// Check stdlib/library methods for this struct type (type-based overload resolution)
		if methods, ok := c.stdMethods[structName]; ok {
			var stdMethodCandidates []*parser.Function
			for _, fn := range methods {
				if fn.Name == mc.Method && fn.ArgsInRange(len(argTypes)) {
					stdMethodCandidates = append(stdMethodCandidates, fn)
				}
			}
			if t, ok := c.resolveCandidates(mc.Method, mc.Pos, stdMethodCandidates, mc.Args, argTypes); ok {
				return t
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
		if t, ok := c.resolveCandidates(mc.Method, mc.Pos, stdMethodCandidates, mc.Args, argTypes); ok {
			return t
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

// tryConstUnitValue extracts a constant unit value from a literal (possibly
// negated) UnitExpr — degrees when isAngle is true, mm otherwise.
func tryConstUnitValue(expr parser.Expr, isAngle bool) (float64, bool) {
	switch e := expr.(type) {
	case *parser.UnitExpr:
		if e.IsAngle == isAngle {
			if v, ok := tryConstFloat(e.Expr); ok {
				return v * e.Factor, true
			}
		}
	case *parser.UnaryExpr:
		if e.Op == "-" {
			if v, ok := tryConstUnitValue(e.Operand, isAngle); ok {
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

// outOfRange reports whether val falls outside [lo:hi] — inclusive of hi
// normally, exclusive of hi when exclusive is true.
func outOfRange(val, lo, hi float64, exclusive bool) bool {
	if exclusive {
		return val < lo || val >= hi
	}
	return val < lo || val > hi
}

// constInSet reports whether val equals a constant element of elems, using
// extract to pull the constant out of each element expression.
func constInSet[T comparable](val T, elems []parser.Expr, extract func(parser.Expr) (T, bool)) bool {
	for _, elem := range elems {
		if v, ok := extract(elem); ok && v == val {
			return true
		}
	}
	return false
}

// checkConstrainedRangeTypes validates that a ConstrainedRange's bounds (start,
// end, and optional step) are Numbers and that its unit matches targetType.
// msgPrefix is prepended to each error — empty for a variable, `"F() parameter
// \"x\": "` for a parameter — and subject names the constrained thing in the
// unit message ("variable" / "parameter"). Shared by the variable and parameter
// constraint checkers.
func (c *checker) checkConstrainedRangeTypes(pos parser.Pos, con *parser.ConstrainedRange, targetType typeInfo, env *typeEnv, msgPrefix, subject string) {
	checkNum := func(e parser.Expr, which string) {
		if e == nil {
			return
		}
		if t := c.inferExpr(e, env); t.ft != typeUnknown && t.ft != typeNumber {
			c.addError(pos, fmt.Sprintf("%sconstraint range %s must be a Number, got %s", msgPrefix, which, t.displayName()))
		}
	}
	checkNum(con.Range.Start, "start")
	checkNum(con.Range.End, "end")
	checkNum(con.Range.Step, "step")

	if _, isAngle := parser.AngleFactors[con.Unit]; isAngle {
		if targetType.ft != typeUnknown && targetType.ft != typeAngle {
			c.addError(pos, fmt.Sprintf("%sconstraint unit %q is an angle unit, but %s is %s", msgPrefix, con.Unit, subject, targetType.displayName()))
		}
	} else if _, isUnit := parser.UnitFactors[con.Unit]; isUnit {
		if targetType.ft != typeUnknown && targetType.ft != typeLength {
			c.addError(pos, fmt.Sprintf("%sconstraint unit %q is a length unit, but %s is %s", msgPrefix, con.Unit, subject, targetType.displayName()))
		}
	}
}

// checkRangeConstraintBoundTypes validates that a range constraint's bounds
// match targetType: Length or Number when the target is a Length, Number only
// otherwise. msgPrefix follows the same convention as
// checkConstrainedRangeTypes. Shared by the variable and parameter constraint
// checkers.
func (c *checker) checkRangeConstraintBoundTypes(pos parser.Pos, con *parser.RangeExpr, targetType typeInfo, env *typeEnv, msgPrefix string) {
	st := c.inferExpr(con.Start, env)
	et := c.inferExpr(con.End, env)
	if targetType.ft == typeLength {
		if st.ft != typeUnknown && st.ft != typeLength && st.ft != typeNumber {
			c.addError(pos, fmt.Sprintf("%sconstraint range start must be Length or Number, got %s", msgPrefix, st.displayName()))
		}
		if et.ft != typeUnknown && et.ft != typeLength && et.ft != typeNumber {
			c.addError(pos, fmt.Sprintf("%sconstraint range end must be Length or Number, got %s", msgPrefix, et.displayName()))
		}
	} else {
		if st.ft != typeUnknown && st.ft != typeNumber {
			c.addError(pos, fmt.Sprintf("%sconstraint range start must be a Number, got %s", msgPrefix, st.displayName()))
		}
		if et.ft != typeUnknown && et.ft != typeNumber {
			c.addError(pos, fmt.Sprintf("%sconstraint range end must be a Number, got %s", msgPrefix, et.displayName()))
		}
	}
}

// checkEnumConstraintTypes validates that every enum constraint element's type
// matches targetType. msgPrefix follows the same convention as
// checkConstrainedRangeTypes. Shared by the variable and parameter constraint
// checkers.
func (c *checker) checkEnumConstraintTypes(pos parser.Pos, con *parser.ArrayLitExpr, targetType typeInfo, env *typeEnv, msgPrefix string) {
	for i, elem := range con.Elems {
		elemType := c.inferExpr(elem, env)
		if elemType.ft != typeUnknown && targetType.ft != typeUnknown && !c.typeCompatible(targetType, elemType) {
			c.addError(pos, fmt.Sprintf("%sconstraint enum element %d has type %s, expected %s",
				msgPrefix, i+1, elemType.displayName(), targetType.displayName()))
		}
	}
}

// checkConstraint validates that a variable's constraint is compatible with its type.
func (c *checker) checkConstraint(g *parser.VarStmt, varType typeInfo, env *typeEnv) {
	switch con := g.Constraint.(type) {
	case *parser.ConstrainedRange:
		// Unit range constraint — validate bounds are numeric and unit matches var type
		c.checkConstrainedRangeTypes(g.Pos, con, varType, env, "", "variable")
		// Value range check for constant expressions
		factor, isAngle := parser.AngleFactors[con.Unit]
		if !isAngle {
			var isLength bool
			if factor, isLength = parser.UnitFactors[con.Unit]; !isLength {
				return
			}
		}
		startN, ok1 := tryConstFloat(con.Range.Start)
		endN, ok2 := tryConstFloat(con.Range.End)
		val, ok3 := tryConstUnitValue(g.Value, isAngle)
		if ok1 && ok2 && ok3 && outOfRange(val, startN*factor, endN*factor, con.Range.IsExclusive()) {
			boundsFmt := "[%g:%g]"
			if con.Range.IsExclusive() {
				boundsFmt = "[%g:<%g]"
			}
			c.addError(g.Pos, fmt.Sprintf("value %.4g %s is out of range "+boundsFmt+" %s", val/factor, con.Unit, startN, endN, con.Unit))
		}

	case *parser.RangeExpr:
		// Range bounds should match var type or be Number
		c.checkRangeConstraintBoundTypes(g.Pos, con, varType, env, "")
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
		// Non-empty and value range checks — try length bounds first, then plain number
		if startMM, ok1 := tryConstUnitValue(con.Start, false); ok1 {
			if endMM, ok2 := tryConstUnitValue(con.End, false); ok2 {
				if startMM > endMM {
					c.addError(g.Pos, "constraint range is empty: start > end")
				}
				if valMM, ok3 := tryConstUnitValue(g.Value, false); ok3 && outOfRange(valMM, startMM, endMM, con.IsExclusive()) {
					c.addError(g.Pos, "value is out of range")
				}
			}
		} else if startN, ok1 := tryConstFloat(con.Start); ok1 {
			if endN, ok2 := tryConstFloat(con.End); ok2 {
				if valN, ok3 := tryConstFloat(g.Value); ok3 && outOfRange(valN, startN, endN, con.IsExclusive()) {
					boundsFmt := "[%g:%g]"
					if con.IsExclusive() {
						boundsFmt = "[%g:<%g]"
					}
					c.addError(g.Pos, fmt.Sprintf("value %g is out of range "+boundsFmt, valN, startN, endN))
				}
			}
		}

	case *parser.ArrayLitExpr:
		if len(con.Elems) == 0 {
			return // free-form, no validation needed
		}
		// Enum — check all elements are the same type and match the var type
		c.checkEnumConstraintTypes(g.Pos, con, varType, env, "")
		// Value membership check for string enums
		if valStr, ok := tryConstString(g.Value); ok && !constInSet(valStr, con.Elems, tryConstString) {
			c.addError(g.Pos, fmt.Sprintf("value %q is not in the allowed set", valStr))
		}
		// Value membership check for number enums
		if valN, ok := tryConstFloat(g.Value); ok && !constInSet(valN, con.Elems, tryConstFloat) {
			c.addError(g.Pos, fmt.Sprintf("value %g is not in the allowed set", valN))
		}
	}
}

// checkParamConstraint validates a function parameter's constraint expression.
// This is a lightweight version of checkConstraint for Param (not VarStmt):
// only the type checks apply, since a parameter has no constant value to
// range- or membership-check.
func (c *checker) checkParamConstraint(fn *parser.Function, p *parser.Param, paramType typeInfo, env *typeEnv) {
	msgPrefix := fmt.Sprintf("%s() parameter %q: ", fn.Name, p.Name)
	switch con := p.Constraint.(type) {
	case *parser.ConstrainedRange:
		// Validate bounds are numeric and the unit matches the parameter type
		// (shared with the variable path), so e.g. an angle unit on a Length
		// parameter is caught at check time rather than slipping through.
		c.checkConstrainedRangeTypes(fn.Pos, con, paramType, env, msgPrefix, "parameter")
	case *parser.RangeExpr:
		c.checkRangeConstraintBoundTypes(fn.Pos, con, paramType, env, msgPrefix)
	case *parser.ArrayLitExpr:
		c.checkEnumConstraintTypes(fn.Pos, con, paramType, env, msgPrefix)
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
		// whose params don't include a given NamedArg name. Every arg is a
		// NamedArg: all call paths are gated by requireNamedArgs.
		paramIdx := make(map[string]int, len(fn.Params))
		for i, p := range fn.Params {
			paramIdx[p.Name] = i
		}
		reordered := make([]typeInfo, len(fn.Params))
		filled := make([]bool, len(fn.Params))
		invalid := false
		for i, a := range callArgs {
			na := a.(*parser.NamedArg)
			idx, exists := paramIdx[na.Name]
			if !exists || filled[idx] {
				invalid = true
				break
			}
			reordered[idx] = argTypes[i]
			filled[idx] = true
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
			expected := c.resolveParamType(p.Type)
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

// reportOverloadFailure is called when no overload matched. If any named arg
// names don't exist in any candidate, it reports specific "no parameter named"
// errors. Otherwise it falls through to the generic overload error message.
func (c *checker) reportOverloadFailure(name string, pos parser.Pos, candidates []*parser.Function, callArgs []parser.Expr, argTypes []typeInfo) {
	allParamNames := make(map[string]bool)
	for _, fn := range candidates {
		for _, p := range fn.Params {
			allParamNames[p.Name] = true
		}
	}
	reported := false
	for _, a := range callArgs {
		if na, ok := a.(*parser.NamedArg); ok && !allParamNames[na.Name] {
			c.addError(pos, fmt.Sprintf("%s() has no parameter named %q", name, na.Name))
			reported = true
		}
	}
	if reported {
		return
	}
	c.addOverloadError(name, pos, candidates, callArgs, argTypes)
}

// addOverloadError reports a descriptive error when no overload matches,
// listing the actual argument types (with named-arg names when present)
// and available candidate signatures.
func (c *checker) addOverloadError(name string, pos parser.Pos, candidates []*parser.Function, callArgs []parser.Expr, argTypes []typeInfo) {
	argLabels := make([]string, len(argTypes))
	for i, t := range argTypes {
		if i < len(callArgs) {
			if na, ok := callArgs[i].(*parser.NamedArg); ok {
				argLabels[i] = na.Name + ": " + t.displayName()
				continue
			}
		}
		argLabels[i] = t.displayName()
	}
	msg := fmt.Sprintf("no matching overload for %s(%s); candidates are:", name, strings.Join(argLabels, ", "))
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
