package checker

import (
	"facet/app/pkg/fctlang/loader"
	"facet/app/pkg/fctlang/parser"
	"fmt"
	"path/filepath"
	"strings"
)

// Result holds all outputs from a single static analysis pass.
type Result struct {
	Prog                loader.Program
	Errors              []parser.SourceError
	VarTypes            VarTypeMap
	InferredReturnTypes map[string]string  // fn name → inferred return type display name
	Declarations        *DeclResult
}

// DeclLocation is the source position of a declaration.
type DeclLocation struct {
	Line       int    `json:"line"`
	Col        int    `json:"col"`
	File       string `json:"file,omitempty"`       // empty = current file; set for library declarations
	Kind       string `json:"kind,omitempty"`       // "fn", "type", "var" — helps frontend disambiguate
	ReturnType string `json:"returnType,omitempty"` // declared return type name (e.g. "Solid", "Length")
}

// DeclResult bundles declaration locations for Go to Definition support.
type DeclResult struct {
	Decls map[string]DeclLocation `json:"decls"`
}

// VarTypeMap maps source key → (var name → type display name).
type VarTypeMap map[string]map[string]string

// Check performs static type checking, type inference, and declaration extraction
// on a parsed program. Libraries must be resolved before calling Check.
// Returns a Result with all analysis outputs.
func Check(prog loader.Program) *Result {
	c := initChecker(prog)
	globalEnv := c.registerLibraries(prog)
	c.checkGlobals(prog, globalEnv)
	c.checkStructDefaults(prog, globalEnv)
	c.checkDuplicateFunctions(prog)
	c.inferReturnTypes(prog, globalEnv)
	c.validateFunctions(prog)

	inferredRets := make(map[string]string, len(c.inferredReturns))
	for name, ti := range c.inferredReturns {
		inferredRets[name] = ti.displayName()
	}
	return &Result{
		Prog:                prog,
		Errors:              c.errors,
		VarTypes:            c.varTypes,
		InferredReturnTypes: inferredRets,
		Declarations:        buildDeclarations(prog),
	}
}

// registerLibraries registers library struct declarations and methods,
// and builds the global type environment.
func (c *checker) registerLibraries(prog loader.Program) *typeEnv {
	for _, src := range prog.Sources {
		for _, g := range src.Globals {
			if le, ok := g.Value.(*parser.LibExpr); ok {
				rl := prog.Sources[prog.Resolve(le.Path)]
				if rl != nil {
					c.libVarToPath[g.Name] = le.Path
					for _, sd := range rl.StructDecls {
						c.structDecls[g.Name+"."+sd.Name] = sd
					}
					for _, fn := range rl.Functions {
						if fn.ReceiverType != "" {
							qualified := g.Name + "." + fn.ReceiverType
							c.stdMethods[qualified] = append(c.stdMethods[qualified], fn)
						}
					}
				} else {
					c.addError(g.Pos, fmt.Sprintf("library %q not resolved", le.Path))
				}
			}
		}
	}
	return c.seedGlobalEnv()
}

// checkGlobals validates global variables across all sources.
func (c *checker) checkGlobals(prog loader.Program, globalEnv *typeEnv) {
	for srcKey, src := range prog.Sources {
		if srcKey == loader.StdlibPath {
			continue
		}
		c.currentSrcKey = srcKey
		for _, g := range src.Globals {
			t := c.inferExpr(g.Value, globalEnv)
			globalEnv.set(g.Name, t)
			if g.IsConst {
				globalEnv.setConst(g.Name)
			}
			c.recordVarType(g.Name, globalEnv)
			if g.Constraint != nil {
				c.checkConstraint(g, t, globalEnv)
			}
			if t.ft == typeLibrary {
				if le, ok := g.Value.(*parser.LibExpr); ok {
					if ns := libPathToNamespace(le.Path); ns != "" {
						c.srcVarTypes()[g.Name] = "Library:" + ns
					}
				}
			}
		}
	}
}

// checkStructDefaults validates struct field default values against declared types.
func (c *checker) checkStructDefaults(prog loader.Program, globalEnv *typeEnv) {
	for srcKey, src := range prog.Sources {
		c.currentSrcKey = srcKey
		for _, sd := range src.StructDecls {
			for _, f := range sd.Fields {
				if f.Default == nil {
					continue
				}
				defType := c.inferExpr(f.Default, globalEnv)
				expectedType := c.resolveTypeStr(sd.Name, f.Type)
				if defType.ft != typeUnknown && expectedType.ft != typeUnknown && !c.typeCompatible(expectedType, defType) {
					c.addError(sd.Pos, fmt.Sprintf("field %q of %s: default value is %s, expected %s",
						f.Name, sd.Name, defType.displayName(), f.Type))
				}
			}
		}
	}
}

// checkDuplicateFunctions detects ambiguous function definitions in every source.
func (c *checker) checkDuplicateFunctions(prog loader.Program) {
	type funcSigKey struct {
		name       string
		receiver   string
		paramTypes string
	}
	for srcKey, src := range prog.Sources {
		c.currentSrcKey = srcKey
		funcSigSeen := map[funcSigKey]*parser.Function{}
		for _, fn := range src.Functions {
			required := 0
			for _, p := range fn.Params {
				if p.Default == nil {
					required++
				}
			}
			total := len(fn.Params)
			reported := false
			for arity := required; arity <= total && !reported; arity++ {
				var pts []string
				for i := 0; i < arity; i++ {
					pts = append(pts, fn.Params[i].Type)
				}
				key := funcSigKey{name: fn.Name, receiver: fn.ReceiverType, paramTypes: strings.Join(pts, ",")}
				if prev := funcSigSeen[key]; prev != nil && prev != fn {
					if fn.ReceiverType != "" {
						c.addError(fn.Pos, fmt.Sprintf("method %s.%s() has ambiguous signature (conflicts with definition at line %d)", fn.ReceiverType, fn.Name, prev.Pos.Line))
					} else {
						c.addError(fn.Pos, fmt.Sprintf("function %s() has ambiguous signature (conflicts with definition at line %d)", fn.Name, prev.Pos.Line))
					}
					reported = true
				}
				if funcSigSeen[key] == nil {
					funcSigSeen[key] = fn
				}
			}
		}
	}
}

// inferReturnTypes infers return types for unannotated functions (Pass 1).
func (c *checker) inferReturnTypes(prog loader.Program, globalEnv *typeEnv) {
	for srcKey, src := range prog.Sources {
		c.currentSrcKey = srcKey
		nameCounts := map[string]int{}
		for _, fn := range src.Functions {
			if fn.ReceiverType == "" {
				nameCounts[fn.Name]++
			}
		}
		for _, fn := range src.Functions {
			if fn.ReturnType != "" || fn.ReceiverType != "" {
				continue
			}
			if nameCounts[fn.Name] > 1 {
				continue
			}
			env := globalEnv.child()
			for _, p := range fn.Params {
				pt := c.resolveParamType(fn, p.Type)
				env.set(p.Name, pt)
			}
			retType := c.checkStmts(fn.Body, env)
			if retType.ft != typeUnknown {
				c.inferredReturns[fn.Name] = retType
				if retType.ft == typeStruct {
					for _, stmt := range fn.Body {
						if rs, ok := stmt.(*parser.ReturnStmt); ok {
							if sn := c.resolveStructName(rs.Value, env); sn != "" {
								c.inferredReturnStructs[fn.Name] = sn
								break
							}
						}
					}
				}
			}
		}
	}
}

// validateFunctions performs full validation on all functions (Pass 2).
func (c *checker) validateFunctions(prog loader.Program) {
	for srcKey, src := range prog.Sources {
		c.currentSrcKey = srcKey
		srcGlobalEnv := c.seedGlobalEnv()
		for _, g := range src.Globals {
			t := c.inferExpr(g.Value, srcGlobalEnv)
			srcGlobalEnv.set(g.Name, t)
			if g.IsConst {
				srcGlobalEnv.setConst(g.Name)
			}
			c.recordVarType(g.Name, srcGlobalEnv)
		}
		savedStructDecls := c.structDecls
		srcDecls := make(map[string]*parser.StructDecl)
		if stdSrc := prog.Std(); stdSrc != nil {
			for _, sd := range stdSrc.StructDecls {
				srcDecls[sd.Name] = sd
			}
		}
		srcStructs := make(map[string]bool, len(src.StructDecls))
		for _, sd := range src.StructDecls {
			srcDecls[sd.Name] = sd
			srcStructs[sd.Name] = true
		}
		for k, v := range savedStructDecls {
			if strings.Contains(k, ".") {
				srcDecls[k] = v
			}
		}
		c.structDecls = srcDecls
		for _, fn := range src.Functions {
			c.validateFunction(fn, src, prog, srcGlobalEnv, srcStructs)
		}
		c.structDecls = savedStructDecls
	}
}

// validateFunction checks a single function definition.
func (c *checker) validateFunction(fn *parser.Function, src *parser.Source, prog loader.Program, srcGlobalEnv *typeEnv, srcStructs map[string]bool) {
	env := srcGlobalEnv.child()
	for _, p := range fn.Params {
		bareType := strings.TrimPrefix(p.Type, "[]")
		if bareType == "Array" {
			c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: bare Array type is not allowed; use a typed array (e.g., []Solid) or []var for generic arrays", fn.Name, p.Name))
		}
		pt := c.resolveParamType(fn, p.Type)
		if pt.ft == typeUnknown && p.Type != "" && bareType != "Array" {
			c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q has unknown type %q", fn.Name, p.Name, p.Type))
		}
		env.set(p.Name, pt)
		if p.Default != nil && pt.ft != typeUnknown {
			defType := c.inferExpr(p.Default, env)
			if defType.ft != typeUnknown && !c.typeCompatible(pt, defType) {
				c.addError(fn.Pos, fmt.Sprintf("%s() parameter %q: default value is %s, expected %s",
					fn.Name, p.Name, defType.displayName(), pt.displayName()))
			}
		}
		if p.Constraint != nil {
			c.checkParamConstraint(fn, p, pt, env)
		}
	}
	if fn.ReceiverType != "" {
		selfType := typeFromNameStr(fn.ReceiverType)
		if selfType.ft == typeUnknown {
			if _, ok := c.structDecls[fn.ReceiverType]; ok {
				selfType = structTI(fn.ReceiverType)
			} else {
				c.addError(fn.Pos, fmt.Sprintf("method receiver type %q is not defined", fn.ReceiverType))
			}
		}
		isBuiltin := typeFromName(fn.ReceiverType) != typeUnknown
		if !srcStructs[fn.ReceiverType] && !(isBuiltin && src == prog.Std()) {
			c.addError(fn.Pos, fmt.Sprintf("cannot define method on type %q: methods can only be defined on types declared in the same source", fn.ReceiverType))
		}
		env.set("self", selfType)
		if selfType.ft == typeStruct && selfType.structName != "" {
			c.srcVarTypes()["self"] = selfType.structName
		} else {
			dn := selfType.displayName()
			if dn != "unknown" {
				c.srcVarTypes()["self"] = dn
			}
		}
	}
	// Top-down coercion: if return type is a typed array (e.g. []Point), set the element
	// type as context so untyped array literals with anonymous structs can be coerced.
	if strings.HasPrefix(fn.ReturnType, "[]") {
		elemTypeName := fn.ReturnType[2:]
		elemType := typeFromNameStr(elemTypeName)
		if elemType.ft == typeUnknown {
			if _, ok := c.structDecls[elemTypeName]; ok {
				elemType = structTI(elemTypeName)
			}
		}
		if elemType.ft != typeUnknown {
			c.returnElemType = elemType
		}
	}
	retType := c.checkStmts(fn.Body, env)
	if fn.ReturnType == "" && retType.ft != typeUnknown {
		if _, exists := c.inferredReturns[fn.Name]; !exists {
			c.inferredReturns[fn.Name] = retType
		}
	}
	if fn.ReturnType != "" && retType.ft != typeUnknown {
		expected := c.resolveReturnType(fn)
		if expected.ft == typeUnknown {
			c.addError(fn.Pos, fmt.Sprintf("%s() has unknown return type %q", fn.Name, fn.ReturnType))
		}
		// Top-down coercion: if the inferred return type is unknown (e.g. array of anonymous structs)
		// and the declared return is a typed array, accept the coercion — no error.
		if expected.ft != typeUnknown && retType.ft == typeUnknown {
			// Coerced — no error
		} else if expected.ft != typeUnknown && !c.typeCompatible(expected, retType) {
			c.addError(fn.Pos, fmt.Sprintf("%s() declared return type %s, but body returns %s",
				fn.Name, fn.ReturnType, retType.displayName()))
		}
	}
	if !hasYieldAtTopLevel(fn.Body) {
		hasReturn := fn.ReturnType != "" || containsReturn(fn.Body)
		if hasReturn && !returnsOnAllPaths(fn.Body) {
			c.addError(fn.Pos, fmt.Sprintf("%s() does not return on all code paths", fn.Name))
		}
	}
	if !hasYieldAtTopLevel(fn.Body) {
		retTypes := c.collectReturnTypes(fn.Body, env)
		if len(retTypes) > 1 {
			first := retTypes[0]
			for _, rt := range retTypes[1:] {
				if rt.ft != typeUnknown && first.ft != typeUnknown && !c.typeCompatible(first, rt) && !c.typeCompatible(rt, first) {
					c.addError(fn.Pos, fmt.Sprintf("%s() has inconsistent return types: %s and %s",
						fn.Name, first.displayName(), rt.displayName()))
					break
				}
			}
		}
	}
	c.returnElemType = typeInfo{} // clear after all checks for this function
}

// ---------------------------------------------------------------------------
// Declarations
// ---------------------------------------------------------------------------

func buildDeclarations(prog loader.Program) *DeclResult {
	result := &DeclResult{
		Decls: make(map[string]DeclLocation),
	}

	addDecl := func(key string, loc DeclLocation) {
		if existing, exists := result.Decls[key]; exists {
			if loc.Kind == "type" && existing.Kind != "type" {
				result.Decls["struct:"+key] = loc
				return
			}
			if loc.Kind != "type" && existing.Kind == "type" {
				result.Decls["struct:"+key] = existing
				result.Decls[key] = loc
				return
			}
		}
		result.Decls[key] = loc
	}

	// Declarations from all files in the program.
	for srcKey, src := range prog.Sources {
		// File tag: source key for all files (frontend identifies the active file)
		file := srcKey
		for _, fn := range src.Functions {
			key := fn.Name
			if fn.ReceiverType != "" {
				key = fn.ReceiverType + "." + fn.Name
			}
			loc := DeclLocation{Line: fn.Pos.Line, Col: fn.Pos.Col, File: file, Kind: "fn", ReturnType: fn.ReturnType}
			if _, exists := result.Decls[key]; !exists {
				addDecl(key, loc)
			}
		}
		for _, sd := range src.StructDecls {
			loc := DeclLocation{Line: sd.Pos.Line, Col: sd.Pos.Col, File: file, Kind: "type"}
			addDecl(sd.Name, loc)
		}
		for _, v := range src.Globals {
			kind := "var"
			if v.IsConst {
				kind = "const"
			}
			loc := DeclLocation{Line: v.Pos.Line, Col: v.Pos.Col, File: file, Kind: kind}
			if _, exists := result.Decls[v.Name]; !exists {
				addDecl(v.Name, loc)
			}
		}
		// Qualified library declarations (e.g. T.FunctionName, T.StructName)
		for _, g := range src.Globals {
			le, ok := g.Value.(*parser.LibExpr)
			if !ok {
				continue
			}
			libSrc := prog.Sources[prog.Resolve(le.Path)]
			if libSrc == nil {
				continue
			}
			for _, fn := range libSrc.Functions {
				loc := DeclLocation{Line: fn.Pos.Line, Col: fn.Pos.Col, File: le.Path, Kind: "fn", ReturnType: fn.ReturnType}
				if fn.ReceiverType != "" {
					result.Decls[g.Name+"."+fn.ReceiverType+"."+fn.Name] = loc
					if _, exists := result.Decls[fn.ReceiverType+"."+fn.Name]; !exists {
						result.Decls[fn.ReceiverType+"."+fn.Name] = loc
					}
				} else {
					result.Decls[g.Name+"."+fn.Name] = loc
				}
			}
			for _, sd := range libSrc.StructDecls {
				loc := DeclLocation{Line: sd.Pos.Line, Col: sd.Pos.Col, File: le.Path, Kind: "type"}
				qualKey := g.Name + "." + sd.Name
				if existing, exists := result.Decls[qualKey]; exists && existing.Kind == "fn" {
					result.Decls["struct:"+qualKey] = loc
				} else {
					result.Decls[qualKey] = loc
				}
			}
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Internal: checker struct and helpers
// ---------------------------------------------------------------------------

// opMapKey identifies an operator + operand type combination for dispatch.
type opMapKey struct {
	op        string
	leftType  string
	rightType string
}

// checker holds the state for a type-checking pass.
type checker struct {
	prog                  loader.Program
	stdFuncs              []*parser.Function
	stdMethods            map[string][]*parser.Function
	structDecls           map[string]*parser.StructDecl
	opMap                 map[opMapKey]typeInfo
	errors                []parser.SourceError
	varTypes              VarTypeMap
	currentSrcKey         string
	libVarToPath          map[string]string // lib variable name → lib path in prog
	userStructs           map[string]bool
	inferredReturns       map[string]typeInfo
	inferredReturnStructs map[string]string
	// returnElemType is set when checking a function with a declared array return type.
	// It provides top-down coercion context for untyped array literals in return position.
	returnElemType typeInfo
}

func initChecker(prog loader.Program) *checker {
	c := &checker{
		prog:                  prog,
		stdMethods:            make(map[string][]*parser.Function),
		structDecls:           make(map[string]*parser.StructDecl),
		opMap:                 make(map[opMapKey]typeInfo),
		varTypes:              make(VarTypeMap),
		libVarToPath:          make(map[string]string),
		inferredReturns:       make(map[string]typeInfo),
		inferredReturnStructs: make(map[string]string),
		userStructs:           make(map[string]bool),
	}
	if stdSrc := prog.Std(); stdSrc != nil {
		for _, fn := range stdSrc.Functions {
			if fn.IsOperator {
				continue
			}
			if fn.ReceiverType != "" {
				c.stdMethods[fn.ReceiverType] = append(c.stdMethods[fn.ReceiverType], fn)
			} else {
				c.stdFuncs = append(c.stdFuncs, fn)
			}
		}
		for _, sd := range stdSrc.StructDecls {
			c.structDecls[sd.Name] = sd
		}
	}
	for _, src := range prog.Sources {
		for _, sd := range src.StructDecls {
			c.structDecls[sd.Name] = sd
			c.userStructs[sd.Name] = true
		}
		c.registerOpFuncs(src.Functions)
	}
	return c
}

func (c *checker) registerOpFuncs(fns []*parser.Function) {
	for _, fn := range fns {
		if !fn.IsOperator {
			continue
		}
		retType := c.resolveOpReturnType(fn)
		switch len(fn.Params) {
		case 1:
			leftType := c.resolveOpParamDisplayName(fn.Params[0].Type)
			key := opMapKey{op: fn.Name, leftType: leftType}
			c.opMap[key] = retType
		case 2:
			leftType := c.resolveOpParamDisplayName(fn.Params[0].Type)
			rightType := c.resolveOpParamDisplayName(fn.Params[1].Type)
			key := opMapKey{op: fn.Name, leftType: leftType, rightType: rightType}
			c.opMap[key] = retType
		default:
			c.addError(fn.Pos, fmt.Sprintf("operator function %q must have 1 or 2 parameters, got %d", fn.Name, len(fn.Params)))
		}
	}
}

func (c *checker) resolveOpParamDisplayName(typeName string) string {
	ti := typeFromNameStr(typeName)
	if ti.ft != typeUnknown {
		return ti.displayName()
	}
	if _, ok := c.structDecls[typeName]; ok {
		return typeName
	}
	return typeName
}

func (c *checker) resolveOpReturnType(fn *parser.Function) typeInfo {
	if fn.ReturnType == "" {
		return unknown()
	}
	ti := typeFromNameStr(fn.ReturnType)
	if ti.ft != typeUnknown {
		return ti
	}
	if _, ok := c.structDecls[fn.ReturnType]; ok {
		return structTI(fn.ReturnType)
	}
	return unknown()
}

func (c *checker) seedGlobalEnv() *typeEnv {
	env := c.newEnv()
	if stdSrc := c.prog.Std(); stdSrc != nil {
		for _, g := range stdSrc.Globals {
			t := c.inferExpr(g.Value, env)
			env.set(g.Name, t)
		}
	}
	return env
}

func (c *checker) srcVarTypes() map[string]string {
	m := c.varTypes[c.currentSrcKey]
	if m == nil {
		m = make(map[string]string)
		c.varTypes[c.currentSrcKey] = m
	}
	return m
}

func (c *checker) recordVarType(name string, env *typeEnv) {
	t, ok := env.lookup(name)
	if !ok {
		return
	}
	dn := t.displayName()
	if dn != "unknown" {
		if t.ft == typeStruct && t.structName != "" {
			c.srcVarTypes()[name] = bareStructName(t.structName)
		} else {
			c.srcVarTypes()[name] = dn
		}
	}
}

func (c *checker) errorFile() string {
	return c.currentSrcKey
}

func (c *checker) addError(pos parser.Pos, msg string) {
	c.errors = append(c.errors, parser.SourceError{File: c.errorFile(), Line: pos.Line, Col: pos.Col, Message: msg})
}

func (c *checker) addErrorSpan(pos parser.Pos, endCol int, msg string) {
	c.errors = append(c.errors, parser.SourceError{File: c.errorFile(), Line: pos.Line, Col: pos.Col, EndCol: endCol, Message: msg})
}

func bareStructName(name string) string {
	if i := strings.IndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (c *checker) qualifyStructType(parentQualified, typeName string) string {
	baseName := typeName
	if strings.HasPrefix(baseName, "[]") {
		baseName = baseName[2:]
	}
	if _, ok := c.structDecls[baseName]; ok {
		return baseName
	}
	if dotIdx := strings.IndexByte(parentQualified, '.'); dotIdx >= 0 {
		qualified := parentQualified[:dotIdx+1] + baseName
		if _, ok := c.structDecls[qualified]; ok {
			return qualified
		}
	}
	return ""
}

func libPathToNamespace(rawPath string) string {
	lp, err := loader.ParseLibPath(rawPath)
	if err != nil {
		return rawPath
	}
	if lp.IsLocal {
		return rawPath
	}
	ns := filepath.Join(lp.Host, lp.User, lp.Repo, lp.Ref)
	if lp.SubPath != "" {
		ns = filepath.Join(ns, lp.SubPath)
	}
	return ns
}
