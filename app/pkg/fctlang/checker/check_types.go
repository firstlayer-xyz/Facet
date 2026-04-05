package checker

import "strings"

// facetType represents the type of a facet value during static analysis.
type facetType int

const (
	typeUnknown facetType = iota
	typeSolid
	typeSketch
	typeLength
	typeAngle
	typeNumber
	typeVar
	typeArray
	typeBool
	typeString
	typeLibrary
	typeStruct
	typeFunc
)

// typeInfo wraps a facetType with optional element type info (for arrays),
// struct name (for struct types), and function signature (for function types).
type typeInfo struct {
	ft         facetType
	elem       *typeInfo  // non-nil for typed arrays
	structName string     // for struct types
	funcParams []typeInfo // for function types: parameter types
	funcReturn *typeInfo  // for function types: return type (nil = void)
}

// simple creates a typeInfo for a basic type (non-array, non-struct).
func simple(ft facetType) typeInfo { return typeInfo{ft: ft} }

// arrayOf creates a typed array typeInfo.
func arrayOf(elem typeInfo) typeInfo { return typeInfo{ft: typeArray, elem: &elem} }

// structTI creates a struct typeInfo with the given name.
func structTI(name string) typeInfo { return typeInfo{ft: typeStruct, structName: name} }

// funcTI creates a function typeInfo with parameter and return types.
func funcTI(params []typeInfo, ret *typeInfo) typeInfo {
	return typeInfo{ft: typeFunc, funcParams: params, funcReturn: ret}
}

// unknown returns a typeInfo for typeUnknown.
func unknown() typeInfo { return typeInfo{ft: typeUnknown} }

// displayName returns the display name for a typeInfo.
func (ti typeInfo) displayName() string {
	if ti.ft == typeArray && ti.elem != nil {
		return "[]" + ti.elem.displayName()
	}
	if ti.ft == typeStruct && ti.structName != "" {
		return ti.structName
	}
	if ti.ft == typeFunc {
		var params []string
		for _, p := range ti.funcParams {
			params = append(params, p.displayName())
		}
		s := "fn(" + strings.Join(params, ", ") + ")"
		if ti.funcReturn != nil {
			s += " " + ti.funcReturn.displayName()
		}
		return s
	}
	return typeDisplayName(ti.ft)
}

// typeInfoEqual returns true if two typeInfos represent the same type,
// including element types for arrays and struct names for structs.
func (c *checker) typeInfoEqual(a, b typeInfo) bool {
	if a.ft != b.ft {
		return false
	}
	if a.ft == typeArray {
		if a.elem == nil && b.elem == nil {
			return true
		}
		if a.elem == nil || b.elem == nil {
			return false
		}
		return c.typeInfoEqual(*a.elem, *b.elem)
	}
	if a.ft == typeStruct {
		// For equality, require both struct names to match.
		// Empty struct names (anonymous) only match other anonymous structs.
		return a.structName == b.structName
	}
	if a.ft == typeFunc {
		if len(a.funcParams) != len(b.funcParams) {
			return false
		}
		for i := range a.funcParams {
			if !c.typeInfoEqual(a.funcParams[i], b.funcParams[i]) {
				return false
			}
		}
		if a.funcReturn == nil && b.funcReturn == nil {
			return true
		}
		if a.funcReturn == nil || b.funcReturn == nil {
			return false
		}
		return c.typeInfoEqual(*a.funcReturn, *b.funcReturn)
	}
	return true
}

// typeFromName converts an AST type string to a facetType.
func typeFromName(name string) facetType {
	switch name {
	case "Solid":
		return typeSolid
	case "Sketch":
		return typeSketch
	case "Length":
		return typeLength
	case "Angle":
		return typeAngle
	case "Number":
		return typeNumber
	case "var":
		return typeVar
	case "Bool":
		return typeBool
	case "String":
		return typeString
	case "Library":
		return typeLibrary
	default:
		return typeUnknown
	}
}

// typeFromNameStr parses a type string (possibly with [] prefix) into a typeInfo.
// e.g. "[]Vec3" → arrayOf(structTI("Vec3")), "Solid" → simple(typeSolid).
// For struct element types (e.g. "[]MyStruct"), returns arrayOf(structTI("MyStruct")).
func typeFromNameStr(name string) typeInfo {
	if strings.HasPrefix(name, "[]") {
		elemStr := name[2:]
		elem := typeFromNameStr(elemStr)
		if elem.ft == typeUnknown && elemStr != "" {
			// Assume unknown element type is a struct name
			return arrayOf(structTI(elemStr))
		}
		return arrayOf(elem)
	}
	if strings.HasPrefix(name, "fn(") {
		return parseFuncTypeInfo(name)
	}
	ft := typeFromName(name)
	if ft != typeUnknown {
		return simple(ft)
	}
	// Could be a struct name — return unknown ft, caller will resolve
	return unknown()
}

// parseFuncTypeInfo parses a function type string like "fn(Solid,Length) Solid"
// into a structured typeInfo.
func parseFuncTypeInfo(s string) typeInfo {
	// Strip "fn(" prefix
	s = s[3:]
	// Find closing paren
	parenDepth := 1
	i := 0
	for i < len(s) && parenDepth > 0 {
		if s[i] == '(' {
			parenDepth++
		} else if s[i] == ')' {
			parenDepth--
		}
		if parenDepth > 0 {
			i++
		}
	}
	paramStr := s[:i]
	rest := strings.TrimSpace(s[i+1:]) // after ")"

	// Parse param types
	var params []typeInfo
	if paramStr != "" {
		// Split by comma, respecting nested fn() types
		paramParts := splitFuncTypeParams(paramStr)
		for _, p := range paramParts {
			params = append(params, typeFromNameStr(strings.TrimSpace(p)))
		}
	}

	// Parse return type
	var ret *typeInfo
	if rest != "" {
		r := typeFromNameStr(rest)
		ret = &r
	}

	return funcTI(params, ret)
}

// splitFuncTypeParams splits a comma-separated param string, respecting nested fn() types.
func splitFuncTypeParams(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// typeDisplayName returns the display name for a facetType.
func typeDisplayName(t facetType) string {
	switch t {
	case typeSolid:
		return "Solid"
	case typeSketch:
		return "Sketch"
	case typeLength:
		return "Length"
	case typeAngle:
		return "Angle"
	case typeNumber:
		return "Number"
	case typeVar:
		return "var"
	case typeArray:
		return "array"
	case typeBool:
		return "Bool"
	case typeString:
		return "String"
	case typeLibrary:
		return "Library"
	case typeStruct:
		return "type"
	case typeFunc:
		return "fn"
	default:
		return "unknown"
	}
}

// typeEnv is a lexically-scoped type environment.
type typeEnv struct {
	types  map[string]typeInfo
	consts map[string]bool // tracks const bindings
	parent *typeEnv
}

func (c *checker) newEnv() *typeEnv {
	return &typeEnv{
		types:  make(map[string]typeInfo),
		consts: make(map[string]bool),
	}
}

func (e *typeEnv) child() *typeEnv {
	return &typeEnv{
		types:  make(map[string]typeInfo),
		consts: make(map[string]bool),
		parent: e,
	}
}

func (e *typeEnv) set(name string, t typeInfo) {
	e.types[name] = t
}

func (e *typeEnv) setConst(name string) {
	e.consts[name] = true
}

func (e *typeEnv) isConst(name string) bool {
	if e.consts[name] {
		return true
	}
	if e.parent != nil {
		return e.parent.isConst(name)
	}
	return false
}

func (e *typeEnv) lookup(name string) (typeInfo, bool) {
	if t, ok := e.types[name]; ok {
		return t, true
	}
	if e.parent != nil {
		return e.parent.lookup(name)
	}
	return unknown(), false
}


// builtinSig describes the parameter types and return type for a builtin function.
type builtinSig struct {
	params []typeInfo
	ret    typeInfo
}

var builtinSigs = map[string]builtinSig{
	"_cube":   {params: []typeInfo{simple(typeLength), simple(typeLength), simple(typeLength)}, ret: simple(typeSolid)},
	"_square": {params: []typeInfo{simple(typeLength), simple(typeLength)}, ret: simple(typeSketch)},
	"_polygon":  {params: []typeInfo{arrayOf(structTI("Vec2"))}, ret: simple(typeSketch)},
	"_hull":         {params: []typeInfo{arrayOf(simple(typeVar))}, ret: unknown()},
	"_union":        {params: []typeInfo{arrayOf(simple(typeVar))}, ret: unknown()},
	"_difference":   {params: []typeInfo{arrayOf(simple(typeVar))}, ret: unknown()},
	"_intersection": {params: []typeInfo{arrayOf(simple(typeVar))}, ret: unknown()},
	"_sin":          {params: []typeInfo{simple(typeAngle)}, ret: simple(typeNumber)},
	"_cos":          {params: []typeInfo{simple(typeAngle)}, ret: simple(typeNumber)},
	"_tan":          {params: []typeInfo{simple(typeAngle)}, ret: simple(typeNumber)},
	"_asin":         {params: []typeInfo{simple(typeNumber)}, ret: simple(typeAngle)},
	"_acos":         {params: []typeInfo{simple(typeNumber)}, ret: simple(typeAngle)},
	"_atan2":        {params: []typeInfo{simple(typeNumber), simple(typeNumber)}, ret: simple(typeAngle)},
	"_sqrt":         {params: []typeInfo{simple(typeNumber)}, ret: simple(typeNumber)},
	"_pow":          {params: []typeInfo{simple(typeNumber), simple(typeNumber)}, ret: simple(typeNumber)},
	"_floor":        {params: []typeInfo{simple(typeNumber)}, ret: simple(typeNumber)},
	"_ceil":         {params: []typeInfo{simple(typeNumber)}, ret: simple(typeNumber)},
	"_round":        {params: []typeInfo{simple(typeNumber)}, ret: simple(typeNumber)},
	"_loft":                 {params: []typeInfo{arrayOf(simple(typeSketch)), arrayOf(simple(typeLength))}, ret: simple(typeSolid)},
	"_load_mesh":            {params: []typeInfo{simple(typeString)}, ret: simple(typeSolid)},
	"_solid_from_mesh": {params: []typeInfo{arrayOf(structTI("Vec3")), arrayOf(structTI("Face"))}, ret: simple(typeSolid)},
	"_utc_date":             {params: []typeInfo{}, ret: simple(typeString)},
	"_utc_time":             {params: []typeInfo{}, ret: simple(typeString)},
}
