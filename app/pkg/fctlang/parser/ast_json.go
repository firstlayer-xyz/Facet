package parser

import "encoding/json"

// MarshalJSON serializes a Pos to JSON with lowercase keys.
func (p Pos) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Line int `json:"line"`
		Col  int `json:"col"`
	}{Line: p.Line, Col: p.Col})
}

// MarshalJSON serializes a Source to JSON with type-tagged AST nodes.
func (p *Source) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Globals     []*VarStmt    `json:"globals"`
		Functions   []*Function   `json:"functions"`
		StructDecls []*StructDecl `json:"structDecls"`
	}{
		Globals:     p.Globals,
		Functions:   p.Functions,
		StructDecls: p.StructDecls,
	})
}

// MarshalJSON serializes a VarStmt to JSON.
func (v *VarStmt) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":  "VarStmt",
		"name":  v.Name,
		"value": marshalExpr(v.Value),
		"pos":   v.Pos,
	}
	if v.IsConst {
		m["isConst"] = true
	}
	if v.Constraint != nil {
		m["constraint"] = marshalExpr(v.Constraint)
	}
	return json.Marshal(m)
}

// MarshalJSON serializes a Function to JSON.
func (f *Function) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         string        `json:"type"`
		Name         string        `json:"name"`
		ReceiverType string        `json:"receiverType,omitempty"`
		IsOperator   bool          `json:"isOperator,omitempty"`
		ReturnType   string        `json:"returnType,omitempty"`
		Params       []*Param      `json:"params"`
		Body         []interface{} `json:"body"`
		Pos          Pos           `json:"pos"`
		Doc          string        `json:"doc,omitempty"`
	}{
		Type:         "Function",
		Name:         f.Name,
		ReceiverType: f.ReceiverType,
		IsOperator:   f.IsOperator,
		ReturnType:   f.ReturnType,
		Params:       f.Params,
		Body:         marshalStmts(f.Body),
		Pos:          f.Pos,
		Doc:          DocComment(f.Comments),
	})
}

// MarshalJSON serializes a Param to JSON.
func (p *Param) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name       string      `json:"name"`
		Type       string      `json:"type,omitempty"`
		Default    interface{} `json:"default,omitempty"`
		Constraint interface{} `json:"constraint,omitempty"`
	}{
		Name:       p.Name,
		Type:       p.Type,
		Default:    marshalExpr(p.Default),
		Constraint: marshalExpr(p.Constraint),
	})
}

// MarshalJSON serializes a StructDecl to JSON.
func (s *StructDecl) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string         `json:"type"`
		Name   string         `json:"name"`
		Fields []*StructField `json:"fields"`
		Pos    Pos            `json:"pos"`
		Doc    string         `json:"doc,omitempty"`
	}{
		Type:   "StructDecl",
		Name:   s.Name,
		Fields: s.Fields,
		Pos:    s.Pos,
		Doc:    DocComment(s.Comments),
	})
}

// MarshalJSON serializes a StructField to JSON.
func (f *StructField) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name    string      `json:"name"`
		Type    string      `json:"type"`
		Default interface{} `json:"default,omitempty"`
	}{
		Name:    f.Name,
		Type:    f.Type,
		Default: marshalExpr(f.Default),
	})
}

func marshalStmts(stmts []Stmt) []interface{} {
	out := make([]interface{}, len(stmts))
	for i, s := range stmts {
		out[i] = marshalStmt(s)
	}
	return out
}

func marshalStmt(s Stmt) interface{} {
	if s == nil {
		return nil
	}
	switch s := s.(type) {
	case *ReturnStmt:
		return map[string]interface{}{
			"type":  "ReturnStmt",
			"value": marshalExpr(s.Value),
			"pos":   s.Pos,
		}
	case *VarStmt:
		m := map[string]interface{}{
			"type":  "VarStmt",
			"name":  s.Name,
			"value": marshalExpr(s.Value),
			"pos":   s.Pos,
		}
		if s.IsConst {
			m["isConst"] = true
		}
		if s.Constraint != nil {
			m["constraint"] = marshalExpr(s.Constraint)
		}
		return m
	case *YieldStmt:
		return map[string]interface{}{
			"type":  "YieldStmt",
			"value": marshalExpr(s.Value),
			"pos":   s.Pos,
		}
	case *AssignStmt:
		return map[string]interface{}{
			"type":  "AssignStmt",
			"name":  s.Name,
			"value": marshalExpr(s.Value),
			"pos":   s.Pos,
		}
	case *FieldAssignStmt:
		return map[string]interface{}{
			"type":     "FieldAssignStmt",
			"receiver": marshalExpr(s.Receiver),
			"field":    s.Field,
			"value":    marshalExpr(s.Value),
			"pos":      s.Pos,
		}
	case *ExprStmt:
		return map[string]interface{}{
			"type": "ExprStmt",
			"expr": marshalExpr(s.Expr),
		}
	case *IfStmt:
		m := map[string]interface{}{
			"type": "IfStmt",
			"cond": marshalExpr(s.Cond),
			"then": marshalStmts(s.Then),
			"pos":  s.Pos,
		}
		if len(s.ElseIfs) > 0 {
			elseIfs := make([]interface{}, len(s.ElseIfs))
			for i, ei := range s.ElseIfs {
				elseIfs[i] = map[string]interface{}{
					"cond": marshalExpr(ei.Cond),
					"body": marshalStmts(ei.Body),
					"pos":  ei.Pos,
				}
			}
			m["elseIfs"] = elseIfs
		}
		if len(s.Else) > 0 {
			m["else"] = marshalStmts(s.Else)
		}
		return m
	case *AssertStmt:
		m := map[string]interface{}{
			"type": "AssertStmt",
			"pos":  s.Pos,
		}
		if s.Cond != nil {
			m["cond"] = marshalExpr(s.Cond)
		}
		if s.Message != nil {
			m["message"] = marshalExpr(s.Message)
		}
		if s.Value != nil {
			m["value"] = marshalExpr(s.Value)
		}
		if s.Constraint != nil {
			m["constraint"] = marshalExpr(s.Constraint)
		}
		return m
	default:
		return map[string]interface{}{"type": "Unknown"}
	}
}

func marshalExpr(e Expr) interface{} {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *NumberLit:
		return map[string]interface{}{
			"type":  "NumberLit",
			"value": e.Value,
		}
	case *BoolLit:
		return map[string]interface{}{
			"type":  "BoolLit",
			"value": e.Value,
		}
	case *StringLit:
		return map[string]interface{}{
			"type":  "StringLit",
			"value": e.Value,
		}
	case *IdentExpr:
		return map[string]interface{}{
			"type": "IdentExpr",
			"name": e.Name,
			"pos":  e.Pos,
		}
	case *UnaryExpr:
		return map[string]interface{}{
			"type":    "UnaryExpr",
			"op":      e.Op,
			"operand": marshalExpr(e.Operand),
			"pos":     e.Pos,
		}
	case *BinaryExpr:
		return map[string]interface{}{
			"type":  "BinaryExpr",
			"op":    e.Op,
			"left":  marshalExpr(e.Left),
			"right": marshalExpr(e.Right),
			"pos":   e.Pos,
		}
	case *CallExpr:
		return map[string]interface{}{
			"type": "CallExpr",
			"name": e.Name,
			"args": marshalExprs(e.Args),
			"pos":  e.Pos,
		}
	case *NamedArg:
		return map[string]interface{}{
			"type":  "NamedArg",
			"name":  e.Name,
			"value": marshalExpr(e.Value),
			"pos":   e.Pos,
		}
	case *MethodCallExpr:
		return map[string]interface{}{
			"type":     "MethodCallExpr",
			"receiver": marshalExpr(e.Receiver),
			"method":   e.Method,
			"args":     marshalExprs(e.Args),
			"pos":      e.Pos,
		}
	case *UnitExpr:
		return map[string]interface{}{
			"type":    "UnitExpr",
			"expr":    marshalExpr(e.Expr),
			"unit":    e.Unit,
			"factor":  e.Factor,
			"isAngle": e.IsAngle,
			"pos":     e.Pos,
		}
	case *FieldAccessExpr:
		return map[string]interface{}{
			"type":     "FieldAccessExpr",
			"receiver": marshalExpr(e.Receiver),
			"field":    e.Field,
			"pos":      e.Pos,
		}
	case *IndexExpr:
		return map[string]interface{}{
			"type":     "IndexExpr",
			"receiver": marshalExpr(e.Receiver),
			"index":    marshalExpr(e.Index),
			"pos":      e.Pos,
		}
	case *ArrayLitExpr:
		m := map[string]interface{}{
			"type":  "ArrayLitExpr",
			"elems": marshalExprs(e.Elems),
			"pos":   e.Pos,
		}
		if e.TypeName != "" {
			m["typeName"] = e.TypeName
		}
		return m
	case *RangeExpr:
		m := map[string]interface{}{
			"type":      "RangeExpr",
			"start":     marshalExpr(e.Start),
			"end":       marshalExpr(e.End),
			"exclusive": e.Exclusive,
			"pos":       e.Pos,
		}
		if e.Step != nil {
			m["step"] = marshalExpr(e.Step)
		}
		return m
	case *ConstrainedRange:
		return map[string]interface{}{
			"type":  "ConstrainedRange",
			"range": marshalExpr(e.Range),
			"unit":  e.Unit,
		}
	case *StructLitExpr:
		fields := make([]interface{}, len(e.Fields))
		for i, f := range e.Fields {
			fields[i] = map[string]interface{}{
				"name":  f.Name,
				"value": marshalExpr(f.Value),
			}
		}
		return map[string]interface{}{
			"type":     "StructLitExpr",
			"typeName": e.TypeName,
			"fields":   fields,
			"pos":      e.Pos,
		}
	case *ForYieldExpr:
		clauses := make([]interface{}, len(e.Clauses))
		for i, c := range e.Clauses {
			cl := map[string]interface{}{
				"var":  c.Var,
				"iter": marshalExpr(c.Iter),
				"pos":  c.Pos,
			}
			if c.Index != "" {
				cl["index"] = c.Index
			}
			clauses[i] = cl
		}
		return map[string]interface{}{
			"type":    "ForYieldExpr",
			"clauses": clauses,
			"body":    marshalStmts(e.Body),
			"pos":     e.Pos,
		}
	case *FoldExpr:
		return map[string]interface{}{
			"type":    "FoldExpr",
			"accVar":  e.AccVar,
			"elemVar": e.ElemVar,
			"iter":    marshalExpr(e.Iter),
			"body":    marshalStmts(e.Body),
			"pos":     e.Pos,
		}
	case *LibExpr:
		return map[string]interface{}{
			"type": "LibExpr",
			"path": e.Path,
			"pos":  e.Pos,
		}
	case *LambdaExpr:
		return map[string]interface{}{
			"type":       "LambdaExpr",
			"params":     e.Params,
			"returnType": e.ReturnType,
			"body":       marshalStmts(e.Body),
			"pos":        e.Pos,
		}
	default:
		return map[string]interface{}{"type": "Unknown"}
	}
}

func marshalExprs(exprs []Expr) []interface{} {
	out := make([]interface{}, len(exprs))
	for i, e := range exprs {
		out[i] = marshalExpr(e)
	}
	return out
}
