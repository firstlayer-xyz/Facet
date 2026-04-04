// Package formatter formats Facet source code from its AST representation.
package formatter

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"facet/app/pkg/fctlang/parser"
)

// Options controls formatter behavior.
type Options struct {
	MaxLineLength int // 0 = use default (80)
}

// Format formats a parsed Facet AST back to source code.
func Format(src *parser.Source) string {
	return formatWithOptions(src, Options{})
}

// formatWithOptions formats with explicit options.
func formatWithOptions(src *parser.Source, opts Options) string {
	if opts.MaxLineLength <= 0 {
		opts.MaxLineLength = 80
	}
	f := &formatter{indent: "    ", opts: opts}
	f.formatSource(src)
	return f.buf.String()
}

type formatter struct {
	buf    strings.Builder
	depth  int
	indent string
	opts   Options
}

func (f *formatter) write(s string) {
	f.buf.WriteString(s)
}

func (f *formatter) writeln(s string) {
	f.buf.WriteString(s)
	f.buf.WriteByte('\n')
}

func (f *formatter) writeIndent() {
	for i := 0; i < f.depth; i++ {
		f.buf.WriteString(f.indent)
	}
}

func (f *formatter) writeIndented(s string) {
	f.writeIndent()
	f.buf.WriteString(s)
}

func (f *formatter) writeIndentedLn(s string) {
	f.writeIndent()
	f.writeln(s)
}

// currentLineLen returns the length of the current (last) line in the buffer.
func (f *formatter) currentLineLen() int {
	s := f.buf.String()
	i := strings.LastIndexByte(s, '\n')
	if i < 0 {
		return len(s)
	}
	return len(s) - i - 1
}

// measureExpr returns the single-line width of an expression.
func (f *formatter) measureExpr(e parser.Expr) int {
	m := &formatter{indent: f.indent, depth: f.depth, opts: f.opts}
	m.formatExpr(e)
	return m.buf.Len()
}

// wouldExceed returns true if appending extra characters to the current line
// would exceed MaxLineLength.
func (f *formatter) wouldExceed(extra int) bool {
	return f.currentLineLen()+extra > f.opts.MaxLineLength
}

// measureMethodTail returns the single-line width of ".Method(args)".
func (f *formatter) measureMethodTail(e *parser.MethodCallExpr) int {
	m := &formatter{indent: f.indent, depth: f.depth, opts: Options{MaxLineLength: 1 << 30}}
	m.write("." + e.Method + "(")
	m.formatArgs(e.Args)
	m.write(")")
	return m.buf.Len()
}

func (f *formatter) formatSource(src *parser.Source) {
	// Declarations are already in source order — iterate directly.
	prevEndLine := 0
	for i, decl := range src.Declarations {
		pos := decl.DeclPos()
		if i > 0 {
			_, prevIsVar := src.Declarations[i-1].(*parser.VarStmt)
			_, curIsVar := decl.(*parser.VarStmt)
			if prevIsVar && curIsVar {
				// Between consecutive vars: blank line only if original had one
				if pos.Line > prevEndLine+1 {
					f.write("\n")
				}
			} else {
				// Always blank line before/after functions and structs
				f.write("\n")
			}
		}
		switch d := decl.(type) {
		case *parser.VarStmt:
			leading, trailing := splitComments(d.Comments)
			f.writeComments(leading)
			f.formatVarDeclTrailing(d, trailing)
		case *parser.Function:
			leading, trailing := splitComments(d.Comments)
			f.writeComments(leading)
			f.formatFunction(d, trailing)
		case *parser.StructDecl:
			leading, trailing := splitComments(d.Comments)
			f.writeComments(leading)
			f.formatStructDecl(d, trailing)
		}
		// Track end line for blank-line detection
		prevEndLine = pos.Line
		switch d := decl.(type) {
		case *parser.Function:
			prevEndLine += len(d.Body) + 1
		case *parser.StructDecl:
			prevEndLine += len(d.Fields) + 1
		}
	}

	// Trailing comments
	f.writeComments(src.TrailingComments)

	// Ensure single trailing newline
	s := f.buf.String()
	s = strings.TrimRight(s, "\n")
	if s != "" {
		s += "\n"
	} else {
		s = "\n"
	}
	f.buf.Reset()
	f.buf.WriteString(s)
}

func (f *formatter) writeComments(comments []parser.Comment) {
	for _, c := range comments {
		f.writeIndent()
		if c.IsDoc {
			f.writeln("# " + c.Text)
		} else {
			f.writeln("// " + c.Text)
		}
	}
}

// splitComments separates leading comments from trailing (end-of-line) comments.
func splitComments(comments []parser.Comment) (leading, trailing []parser.Comment) {
	for _, c := range comments {
		if c.IsTrailing {
			trailing = append(trailing, c)
		} else {
			leading = append(leading, c)
		}
	}
	return
}

// writeTrailingComment appends an inline comment at the end of the current line
// (before the newline). Called after writing a statement but before writeln.
func (f *formatter) writeTrailingComment(comments []parser.Comment) {
	for _, c := range comments {
		if c.IsDoc {
			f.write(" # " + c.Text)
		} else {
			f.write(" // " + c.Text)
		}
	}
}

func (f *formatter) formatVarDecl(v *parser.VarStmt) {
	f.formatVarDeclTrailing(v, nil)
}

func (f *formatter) formatVarDeclTrailing(v *parser.VarStmt, trailing []parser.Comment) {
	f.writeIndent()
	if v.IsConst {
		f.write("const ")
	} else {
		f.write("var ")
	}
	f.write(v.Name + " = ")
	f.formatExpr(v.Value)
	if v.Constraint != nil {
		f.write(" where ")
		f.formatExpr(v.Constraint)
	}
	f.writeTrailingComment(trailing)
	f.writeln("")
}

func (f *formatter) formatFunction(fn *parser.Function, trailing []parser.Comment) {
	f.writeIndent()
	f.write("fn ")
	if fn.ReceiverType != "" {
		f.write(fn.ReceiverType + ".")
	}
	f.write(fn.Name)

	// Use multi-line params if any param has a default or constraint.
	multiLine := false
	for _, p := range fn.Params {
		if p.Default != nil || p.Constraint != nil {
			multiLine = true
			break
		}
	}

	if multiLine && len(fn.Params) > 0 {
		f.writeln("(")
		f.depth++
		f.formatParamsMultiLine(fn.Params)
		f.depth--
		f.writeIndent()
		f.write(")")
	} else {
		f.write("(")
		f.formatParams(fn.Params)
		f.write(")")
	}
	if fn.ReturnType != "" {
		f.write(" " + fn.ReturnType)
	}
	f.write(" {")
	f.writeTrailingComment(trailing)
	f.writeln("")
	f.depth++
	f.formatStmts(fn.Body)
	f.depth--
	f.writeIndentedLn("}")
}

func (f *formatter) formatParamsMultiLine(params []*parser.Param) {
	for _, p := range params {
		f.writeIndent()
		f.write(p.Name)
		if p.Type != "" {
			f.write(" " + p.Type)
		}
		if p.Default != nil {
			f.write(" = ")
			f.formatExpr(p.Default)
		}
		if p.Constraint != nil {
			f.write(" where ")
			f.formatExpr(p.Constraint)
		}
		f.writeln(",")
	}
}

func (f *formatter) formatParams(params []*parser.Param) {
	// Facet param syntax: name Type [= default] [where constraint]
	// Consecutive same-type params without defaults/constraints share the trailing type.
	// Groups separated by commas between groups: "x, y Length, z Angle"

	type paramGroup struct {
		params []*parser.Param
		typ    string
	}
	var groups []paramGroup
	for _, p := range params {
		if len(groups) == 0 || groups[len(groups)-1].typ != p.Type || p.Default != nil || p.Constraint != nil {
			groups = append(groups, paramGroup{typ: p.Type})
		}
		groups[len(groups)-1].params = append(groups[len(groups)-1].params, p)
	}

	firstParam := true
	for _, g := range groups {
		for _, p := range g.params {
			if !firstParam {
				f.write(", ")
			}
			firstParam = false
			f.write(p.Name)
		}
		if g.typ != "" {
			f.write(" " + g.typ)
		}
		// Defaults/constraints only on single-param groups, written after type.
		if len(g.params) == 1 {
			p := g.params[0]
			if p.Default != nil {
				f.write(" = ")
				f.formatExpr(p.Default)
			}
			if p.Constraint != nil {
				f.write(" where ")
				f.formatExpr(p.Constraint)
			}
		}
	}
}

func (f *formatter) formatStructDecl(sd *parser.StructDecl, trailing []parser.Comment) {
	f.writeIndent()
	f.write("type " + sd.Name + " {")
	f.writeTrailingComment(trailing)
	f.writeln("")
	f.depth++
	// Find max field name length for type alignment
	maxNameLen := 0
	for _, field := range sd.Fields {
		if len(field.Name) > maxNameLen {
			maxNameLen = len(field.Name)
		}
	}
	for _, field := range sd.Fields {
		fLeading, fTrailing := splitComments(field.Comments)
		f.writeComments(fLeading)
		f.writeIndent()
		f.write(field.Name)
		padding := maxNameLen - len(field.Name) + 1
		for i := 0; i < padding; i++ {
			f.write(" ")
		}
		f.write(field.Type)
		if field.Default != nil {
			f.write(" = ")
			f.formatExpr(field.Default)
		}
		if field.Constraint != nil {
			f.write(" where ")
			f.formatExpr(field.Constraint)
		}
		f.writeTrailingComment(fTrailing)
		f.writeln("")
	}
	f.depth--
	f.writeIndentedLn("}")
}

func (f *formatter) formatStmts(stmts []parser.Stmt) {
	for _, s := range stmts {
		f.formatStmt(s)
	}
}

func (f *formatter) formatStmt(s parser.Stmt) {
	switch s := s.(type) {
	case *parser.ReturnStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.writeIndent()
		if s.Value != nil {
			f.write("return ")
			f.formatExpr(s.Value)
		} else {
			f.write("return")
		}
		f.writeTrailingComment(trailing)
		f.writeln("")
	case *parser.VarStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.formatVarDeclTrailing(s, trailing)
	case *parser.YieldStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.writeIndent()
		if s.Value == nil {
			f.writeTrailingComment(trailing)
			f.writeln("yield")
		} else {
			f.write("yield ")
			f.formatExpr(s.Value)
			f.writeTrailingComment(trailing)
			f.writeln("")
		}
	case *parser.AssignStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.writeIndent()
		f.write(s.Name + " = ")
		f.formatExpr(s.Value)
		f.writeTrailingComment(trailing)
		f.writeln("")
	case *parser.FieldAssignStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.writeIndent()
		f.formatExpr(s.Receiver)
		f.write("." + s.Field + " = ")
		f.formatExpr(s.Value)
		f.writeTrailingComment(trailing)
		f.writeln("")
	case *parser.ExprStmt:
		f.writeComments(s.Comments)
		f.writeIndent()
		f.formatExpr(s.Expr)
		f.writeln("")
	case *parser.IfStmt:
		f.writeComments(s.Comments)
		f.writeIndent()
		f.write("if ")
		f.formatExpr(s.Cond)
		f.writeln(" {")
		f.depth++
		f.formatStmts(s.Then)
		f.depth--
		for _, ei := range s.ElseIfs {
			f.writeComments(ei.Comments)
			f.writeIndent()
			f.write("} else if ")
			f.formatExpr(ei.Cond)
			f.writeln(" {")
			f.depth++
			f.formatStmts(ei.Body)
			f.depth--
		}
		if len(s.Else) > 0 {
			f.writeIndentedLn("} else {")
			f.depth++
			f.formatStmts(s.Else)
			f.depth--
		}
		f.writeIndentedLn("}")
	case *parser.AssertStmt:
		leading, trailing := splitComments(s.Comments)
		f.writeComments(leading)
		f.writeIndent()
		if s.Value != nil {
			// assert EXPR where CONSTRAINT form
			f.write("assert ")
			f.formatExpr(s.Value)
			f.write(" where ")
			f.formatExpr(s.Constraint)
		} else {
			f.write("assert ")
			f.formatExpr(s.Cond)
			if s.Message != nil {
				f.write(", ")
				f.formatExpr(s.Message)
			}
		}
		f.writeTrailingComment(trailing)
		f.writeln("")
	}
}

// Operator precedence for parenthesization.
func opPrec(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=", "<", ">", "<=", ">=":
		return 3
	case "|":
		return 4
	case "^":
		return 5
	case "&":
		return 6
	case "+", "-":
		return 7
	case "*", "/", "%":
		return 8
	default:
		return 0
	}
}

func (f *formatter) formatExpr(e parser.Expr) {
	f.formatExprPrec(e, 0)
}

func (f *formatter) formatExprPrec(e parser.Expr, parentPrec int) {
	switch e := e.(type) {
	case *parser.NumberLit:
		if e.Raw != "" {
			f.write(e.Raw)
		} else {
			f.write(formatNumber(e.Value))
		}
	case *parser.BoolLit:
		if e.Value {
			f.write("true")
		} else {
			f.write("false")
		}
	case *parser.StringLit:
		f.write(`"` + escapeString(e.Value) + `"`)
	case *parser.IdentExpr:
		f.write(e.Name)
	case *parser.UnaryExpr:
		f.write(e.Op)
		f.formatExprPrec(e.Operand, 9) // unary binds tighter than anything
	case *parser.BinaryExpr:
		prec := opPrec(e.Op)
		needParens := prec < parentPrec
		if needParens {
			f.write("(")
		}
		f.formatExprPrec(e.Left, prec)
		f.write(" " + e.Op + " ")
		// Right side needs parens if same precedence (left-associative)
		f.formatExprPrec(e.Right, prec+1)
		if needParens {
			f.write(")")
		}
	case *parser.CallExpr:
		f.write(e.Name + "(")
		f.formatArgs(e.Args)
		f.write(")")
	case *parser.BuiltinCallExpr:
		f.write(e.Name + "(")
		f.formatArgs(e.Args)
		f.write(")")
	case *parser.NamedArg:
		f.write(e.Name + ": ")
		f.formatExpr(e.Value)
	case *parser.MethodCallExpr:
		f.formatExprPrec(e.Receiver, 10) // method call binds tightest
		// Measure remaining .Method(args) — if it would exceed the line, break before '.'
		tail := f.measureMethodTail(e)
		if f.wouldExceed(tail) {
			f.writeln("")
			f.depth++
			f.writeIndent()
			f.depth--
		}
		f.write("." + e.Method + "(")
		f.formatArgs(e.Args)
		f.write(")")
	case *parser.FieldAccessExpr:
		f.formatExprPrec(e.Receiver, 10)
		f.write("." + e.Field)
	case *parser.IndexExpr:
		f.formatExprPrec(e.Receiver, 10)
		f.write("[")
		f.formatExpr(e.Index)
		f.write("]")
	case *parser.SliceExpr:
		f.formatExprPrec(e.Receiver, 10)
		f.write("[")
		if e.Start != nil {
			f.formatExpr(e.Start)
		}
		f.write(":")
		if e.End != nil {
			f.formatExpr(e.End)
		}
		f.write("]")
	case *parser.UnitExpr:
		f.formatExprPrec(e.Expr, 10)
		f.write(" " + e.Unit)
	case *parser.ArrayLitExpr:
		if e.TypeName != "" {
			f.write("[]")
			f.write(e.TypeName)
		}
		f.write("[")
		f.formatArrayElems(e)
		f.write("]")
	case *parser.RangeExpr:
		f.write("[")
		f.formatExpr(e.Start)
		f.write(":")
		if e.Exclusive {
			f.write("<")
		}
		f.formatExpr(e.End)
		if e.Step != nil {
			f.write(":")
			f.formatExpr(e.Step)
		}
		f.write("]")
	case *parser.ConstrainedRange:
		f.formatExpr(e.Range)
		if e.Unit != "" {
			f.write(" " + e.Unit)
		}
	case *parser.StructLitExpr:
		if e.TypeName != "" {
			f.write(e.TypeName)
		}
		f.write("{")
		if len(e.Fields) > 0 {
			// Measure single-line width
			total := 0
			for i, fi := range e.Fields {
				if i > 0 {
					total += 2
				}
				total += len(fi.Name) + 2 + f.measureExpr(fi.Value) // "name: value"
			}
			if !f.wouldExceed(total + 1) { // +1 for "}"
				for i, fi := range e.Fields {
					if i > 0 {
						f.write(", ")
					}
					f.write(fi.Name + ": ")
					f.formatExpr(fi.Value)
				}
			} else {
				// Pack fields onto lines, splitting at commas when needed.
				// Fields start right after { and } follows the last field.
				f.depth++
				for i, fi := range e.Fields {
					fieldStr := fi.Name + ": "
					fieldWidth := len(fieldStr) + f.measureExpr(fi.Value)
					if i > 0 {
						if f.wouldExceed(2 + fieldWidth) { // 2 for ", "
							f.write(",")
							f.writeln("")
							f.writeIndent()
						} else {
							f.write(", ")
						}
					}
					f.write(fieldStr)
					f.formatExpr(fi.Value)
				}
				f.depth--
			}
		}
		f.write("}")
	case *parser.LibExpr:
		f.write(`lib "` + e.Path + `"`)
	case *parser.ForYieldExpr:
		f.write("for ")
		for i, c := range e.Clauses {
			if i > 0 {
				f.write(", ")
			}
			if c.Index != "" {
				f.write(c.Index + ", ")
			}
			f.write(c.Var + " ")
			f.formatExpr(c.Iter)
		}
		f.writeln(" {")
		f.depth++
		f.formatStmts(e.Body)
		f.depth--
		f.writeIndent()
		f.write("}")
	case *parser.FoldExpr:
		f.write("fold " + e.AccVar + ", " + e.ElemVar + " ")
		f.formatExpr(e.Iter)
		f.writeln(" {")
		f.depth++
		f.formatStmts(e.Body)
		f.depth--
		f.writeIndent()
		f.write("}")
	case *parser.LambdaExpr:
		f.write("fn(")
		f.formatParams(e.Params)
		f.write(")")
		if e.ReturnType != "" {
			f.write(" " + e.ReturnType)
		}
		f.writeln(" {")
		f.depth++
		f.formatStmts(e.Body)
		f.depth--
		f.writeIndent()
		f.write("}")
	}
}

type elemInfo struct {
	expr      parser.Expr
	stripName bool // true if we should temporarily clear StructLitExpr.TypeName
}

func (f *formatter) formatArrayElems(e *parser.ArrayLitExpr) {
	if len(e.Elems) == 0 {
		return
	}
	elems := make([]elemInfo, len(e.Elems))
	for i, elem := range e.Elems {
		if sl, ok := elem.(*parser.StructLitExpr); ok && e.TypeName != "" && sl.TypeName == e.TypeName {
			elems[i] = elemInfo{expr: elem, stripName: true}
		} else {
			elems[i] = elemInfo{expr: elem}
		}
	}

	writeElem := func(ei elemInfo) {
		if ei.stripName {
			sl := ei.expr.(*parser.StructLitExpr)
			saved := sl.TypeName
			sl.TypeName = ""
			f.formatExpr(sl)
			sl.TypeName = saved
		} else {
			f.formatExpr(ei.expr)
		}
	}

	// Measure single-line width
	total := 0
	for i, ei := range elems {
		if i > 0 {
			total += 2
		}
		if ei.stripName {
			sl := ei.expr.(*parser.StructLitExpr)
			saved := sl.TypeName
			sl.TypeName = ""
			total += f.measureExpr(sl)
			sl.TypeName = saved
		} else {
			total += f.measureExpr(ei.expr)
		}
	}
	if !f.wouldExceed(total + 1) { // +1 for "]"
		for i, ei := range elems {
			if i > 0 {
				f.write(", ")
			}
			writeElem(ei)
		}
		return
	}
	// Multi-line: pack as many elements per line as fit.
	f.writeln("")
	f.depth++
	f.writeIndent()
	lineLen := f.currentLineLen()
	for i, ei := range elems {
		elemWidth := f.measureElemInfo(ei)
		sep := 0
		if i > 0 {
			sep = 2 // ", "
		}
		// Would this element push us past the limit?
		if i > 0 && lineLen+sep+elemWidth > f.opts.MaxLineLength {
			f.write(",")
			f.writeln("")
			f.writeIndent()
			lineLen = f.currentLineLen()
			sep = 0
		}
		if sep > 0 {
			f.write(", ")
			lineLen += 2
		}
		writeElem(ei)
		lineLen += elemWidth
	}
	f.writeln("")
	f.depth--
	f.writeIndent()
}

func (f *formatter) measureElemInfo(ei elemInfo) int {
	if ei.stripName {
		sl := ei.expr.(*parser.StructLitExpr)
		saved := sl.TypeName
		sl.TypeName = ""
		w := f.measureExpr(sl)
		sl.TypeName = saved
		return w
	}
	return f.measureExpr(ei.expr)
}

func (f *formatter) formatArgs(args []parser.Expr) {
	if len(args) == 0 {
		return
	}
	// Measure single-line width
	total := 0
	for i, arg := range args {
		if i > 0 {
			total += 2 // ", "
		}
		total += f.measureExpr(arg)
	}
	if !f.wouldExceed(total + 1) { // +1 for closing paren
		for i, arg := range args {
			if i > 0 {
				f.write(", ")
			}
			f.formatExpr(arg)
		}
		return
	}
	// Multi-line
	f.writeln("")
	f.depth++
	for i, arg := range args {
		f.writeIndent()
		f.formatExpr(arg)
		if i < len(args)-1 {
			f.write(",")
		}
		f.writeln("")
	}
	f.depth--
	f.writeIndent()
}

// escapeString re-escapes a string value for output in double quotes.
func escapeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatNumber formats a float64 as a readable number string.
// Integers are formatted without decimal points. Floats use minimal precision.
func formatNumber(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) && !math.IsNaN(v) {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return fmt.Sprintf("%g", v)
}
