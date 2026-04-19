package checker

import (
	"facet/app/pkg/fctlang/parser"
	"fmt"
)

// typeCompatible checks if actualType is compatible with expectedType.
// Number→Length and Number→Angle coercions are allowed. Array compatibility checks element types.
func (c *checker) typeCompatible(expected, actual typeInfo) bool {
	// typeVar is compatible with any type (generic type variable)
	if expected.ft == typeVar || actual.ft == typeVar {
		return true
	}
	if expected.ft == actual.ft {
		// Same kind — check deeper for arrays and structs
		if expected.ft == typeArray {
			// Both arrays: if expected has element type, check compatibility
			if expected.elem != nil && actual.elem != nil {
				return c.typeCompatible(*expected.elem, *actual.elem)
			}
			// Unknown element type is compatible with anything
			return true
		}
		if expected.ft == typeStruct {
			// Both structs: names must match (or one is anonymous)
			if expected.structName == "" || actual.structName == "" {
				return true
			}
			return expected.structName == actual.structName
		}
		if expected.ft == typeFunc {
			// Function types: check param count and compatibility
			if len(expected.funcParams) != len(actual.funcParams) {
				return false
			}
			for i := range expected.funcParams {
				// Contravariant: expected param must be compatible with actual param
				if !c.typeCompatible(expected.funcParams[i], actual.funcParams[i]) &&
					!c.typeCompatible(actual.funcParams[i], expected.funcParams[i]) {
					return false
				}
			}
			// Covariant return
			if expected.funcReturn != nil && actual.funcReturn != nil {
				return c.typeCompatible(*expected.funcReturn, *actual.funcReturn)
			}
			return true
		}
		return true
	}
	// Numeric coercion: Number → Length and Number → Angle (bare numbers auto-cast)
	if (expected.ft == typeLength || expected.ft == typeAngle) && actual.ft == typeNumber {
		return true
	}
	return false
}

// returnsOnAllPaths returns true if every execution path through the
// statement list ends with a return statement.
func returnsOnAllPaths(stmts []parser.Stmt) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *parser.ReturnStmt:
			return true
		case *parser.IfStmt:
			if ifReturnsOnAllPaths(s) {
				return true
			}
		}
	}
	return false
}

// ifReturnsOnAllPaths checks whether an if statement returns on all branches.
func ifReturnsOnAllPaths(s *parser.IfStmt) bool {
	if s.Else == nil {
		return false // no else → not all paths covered
	}
	if !returnsOnAllPaths(s.Then) {
		return false
	}
	for _, eif := range s.ElseIfs {
		if !returnsOnAllPaths(eif.Body) {
			return false
		}
	}
	return returnsOnAllPaths(s.Else)
}

// hasYieldAtTopLevel returns true if the statement list contains a YieldStmt
// at the top level (indicating a for-yield style function body).
func hasYieldAtTopLevel(stmts []parser.Stmt) bool {
	for _, stmt := range stmts {
		if _, ok := stmt.(*parser.YieldStmt); ok {
			return true
		}
	}
	return false
}

// containsReturn returns true if the statement list contains any ReturnStmt.
func containsReturn(stmts []parser.Stmt) bool {
	for _, stmt := range stmts {
		if _, ok := stmt.(*parser.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// collectReturnTypes gathers the inferred types of all return statements,
// recursing into if/else-if/else branches to catch inconsistent return types.
// Each branch gets a child scope so variables declared inside are visible.
func (c *checker) collectReturnTypes(stmts []parser.Stmt, env *typeEnv) []typeInfo {
	var types []typeInfo
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *parser.ReturnStmt:
			if s.Value != nil {
				t := c.inferExpr(s.Value, env)
				types = append(types, t)
			}
		case *parser.VarStmt:
			t := c.inferExpr(s.Value, env)
			env.set(s.Name, t)
		case *parser.AssignStmt:
			c.inferExpr(s.Value, env)
		case *parser.IfStmt:
			branchEnv := env.child()
			types = append(types, c.collectReturnTypes(s.Then, branchEnv)...)
			for _, eif := range s.ElseIfs {
				eifEnv := env.child()
				types = append(types, c.collectReturnTypes(eif.Body, eifEnv)...)
			}
			if s.Else != nil {
				elseEnv := env.child()
				types = append(types, c.collectReturnTypes(s.Else, elseEnv)...)
			}
		}
	}
	return types
}

// stmtPos returns the source position of a statement node.
func stmtPos(s parser.Stmt) parser.Pos {
	switch s := s.(type) {
	case *parser.ReturnStmt:
		return s.Pos
	case *parser.YieldStmt:
		return s.Pos
	case *parser.VarStmt:
		return s.Pos
	case *parser.AssignStmt:
		return s.Pos
	case *parser.FieldAssignStmt:
		return s.Pos
	case *parser.IfStmt:
		return s.Pos
	case *parser.AssertStmt:
		return s.Pos
	case *parser.ExprStmt:
		return s.Pos
	default:
		return parser.Pos{}
	}
}

// checkStmts walks a statement list and returns the type of the return statement.
func (c *checker) checkStmts(stmts []parser.Stmt, env *typeEnv) typeInfo {
	var retType typeInfo
	returned := false
	for _, stmt := range stmts {
		if returned {
			c.addError(stmtPos(stmt), "unreachable code after return")
			break
		}
		switch s := stmt.(type) {
		case *parser.ReturnStmt:
			if s.Value != nil {
				retType = c.inferExpr(s.Value, env)
			}
			returned = true
		case *parser.YieldStmt:
			c.inferExpr(s.Value, env)
		case *parser.VarStmt:
			if _, exists := env.types[s.Name]; exists {
				c.addError(s.Pos, fmt.Sprintf("variable %q already defined in this scope", s.Name))
			} else if env.parent != nil {
				if _, shadowed := env.parent.lookup(s.Name); shadowed {
					c.addError(s.Pos, fmt.Sprintf("variable %q shadows outer variable", s.Name))
				}
			}
			if _, isType := c.structDecls[s.Name]; isType {
				c.addError(s.Pos, fmt.Sprintf("variable %q shadows type %q", s.Name, s.Name))
			}
			t := c.inferExpr(s.Value, env)
			varKind := "var"
			if s.IsConst {
				varKind = "const"
			}
			env.bind(s.Name, t, s.Pos, varKind)
			if s.IsConst {
				env.setConst(s.Name)
			}
			c.recordVarType(s.Name, env)
		case *parser.AssignStmt:
			existing, ok := env.lookup(s.Name)
			if !ok {
				c.addError(s.Pos, fmt.Sprintf("cannot assign to undefined variable %q", s.Name))
			} else if env.isConst(s.Name) {
				c.addError(s.Pos, fmt.Sprintf("cannot reassign const %q", s.Name))
			} else {
				newType := c.inferExpr(s.Value, env)
				if newType.ft != typeUnknown && existing.ft != typeUnknown && !c.typeCompatible(existing, newType) {
					c.addError(s.Pos, fmt.Sprintf("cannot assign %s to variable %q of type %s", newType.displayName(), s.Name, existing.displayName()))
				}
				// Update struct name if reassigned
				if newType.ft == typeStruct {
					if sn := c.resolveStructName(s.Value, env); sn != "" {
						env.set(s.Name, structTI(sn))
					}
				}
				c.recordVarType(s.Name, env)
			}
		case *parser.FieldAssignStmt:
			if ident, ok := s.Receiver.(*parser.IdentExpr); ok && env.isConst(ident.Name) {
				c.addError(s.Pos, fmt.Sprintf("cannot mutate field on const %q", ident.Name))
			}
			recvType := c.inferExpr(s.Receiver, env)
			if recvType.ft == typeUnknown {
				break
			}
			if recvType.ft != typeStruct {
				c.addError(s.Pos, fmt.Sprintf("cannot assign field %q on %s", s.Field, recvType.displayName()))
				break
			}
			structName := c.resolveStructName(s.Receiver, env)
			if structName == "" {
				break
			}
			decl, ok := c.structDecls[structName]
			if !ok {
				break
			}
			var fieldType typeInfo
			found := false
			for _, f := range decl.Fields {
				if f.Name == s.Field {
					fieldType = c.resolveFieldType(structName, f.Type)
					found = true
					break
				}
			}
			if !found {
				c.addError(s.Pos, fmt.Sprintf("struct %s has no field %q", bareStructName(structName), s.Field))
				break
			}
			valType := c.inferExpr(s.Value, env)
			if valType.ft != typeUnknown && fieldType.ft != typeUnknown && !c.typeCompatible(fieldType, valType) {
				c.addError(s.Pos, fmt.Sprintf("cannot assign %s to field %q of type %s", valType.displayName(), s.Field, fieldType.displayName()))
			}
		case *parser.AssertStmt:
			if s.Constraint != nil {
				// "assert EXPR where CONSTRAINT" form
				c.inferExpr(s.Value, env)
			} else {
				// "assert COND [, MSG]" form
				condType := c.inferExpr(s.Cond, env)
				if condType.ft != typeUnknown && condType.ft != typeBool {
					c.addError(s.Pos, fmt.Sprintf("assert condition must be a Bool, got %s", condType.displayName()))
				}
				if s.Message != nil {
					msgType := c.inferExpr(s.Message, env)
					if msgType.ft != typeUnknown && msgType.ft != typeString {
						c.addError(s.Pos, fmt.Sprintf("assert message must be a String, got %s", msgType.displayName()))
					}
				}
			}
		case *parser.IfStmt:
			c.checkIfStmt(s, env)
		case *parser.ExprStmt:
			c.inferExpr(s.Expr, env)
		}
	}
	return retType
}

// checkIfStmt validates an if statement's condition and branches.
func (c *checker) checkIfStmt(s *parser.IfStmt, env *typeEnv) {
	condType := c.inferExpr(s.Cond, env)
	if condType.ft != typeUnknown && condType.ft != typeBool {
		c.addError(s.Pos, fmt.Sprintf("if condition must be a Bool, got %s", condType.displayName()))
	}
	c.checkStmts(s.Then, env.child())
	for _, eif := range s.ElseIfs {
		eifCond := c.inferExpr(eif.Cond, env)
		if eifCond.ft != typeUnknown && eifCond.ft != typeBool {
			c.addError(eif.Pos, fmt.Sprintf("else-if condition must be a Bool, got %s", eifCond.displayName()))
		}
		c.checkStmts(eif.Body, env.child())
	}
	if s.Else != nil {
		c.checkStmts(s.Else, env.child())
	}
}

// resolveFieldType resolves a struct field type string to a typeInfo.
func (c *checker) resolveFieldType(parentStruct, fieldType string) typeInfo {
	ti := typeFromNameStr(fieldType)
	if ti.ft != typeUnknown {
		return ti
	}
	// Try as struct type
	if qualified := c.qualifyStructType(parentStruct, fieldType); qualified != "" {
		return structTI(qualified)
	}
	return unknown()
}
