package checker

import "testing"

// TestCheckIfExpressionBasicAccepts confirms a well-formed if expression
// type-checks without errors when both arms match and cond is Bool.
func TestCheckIfExpressionBasicAccepts(t *testing.T) {
	expectNoErrors(t, `fn Main() Number {
		var c = if 1 > 0 { 1 } else { -1 };
		return c
	}`)
}

// TestCheckIfExpressionElseIfAccepts confirms an else-if chain with
// consistent arm types and Bool conds is accepted.
func TestCheckIfExpressionElseIfAccepts(t *testing.T) {
	expectNoErrors(t, `fn Main() Number {
		var c = if 1 > 0 { 1 } else if 1 < 0 { -1 } else { 0 };
		return c
	}`)
}

// TestCheckIfExpressionRejectsNonBoolCond pins the cond-type guard:
// passing a Number where Bool is required must error.
func TestCheckIfExpressionRejectsNonBoolCond(t *testing.T) {
	expectError(t, `fn Main() Number {
		var c = if 1 { 1 } else { 0 };
		return c
	}`, "must be Bool")
}

// TestCheckIfExpressionRejectsMismatchedArms pins the arm-type guard:
// then=Number / else=String must error.
func TestCheckIfExpressionRejectsMismatchedArms(t *testing.T) {
	expectError(t, `fn Main() Number {
		var c = if 1 > 0 { 1 } else { "no" };
		return c
	}`, "must match")
}

// TestCheckIfExpressionVarTypeInferred confirms the inferred type of
// a var bound to an if-expression is the arm type — so subsequent code
// (e.g. arithmetic on c) gets the right type info.
func TestCheckIfExpressionVarTypeInferred(t *testing.T) {
	vars := inferVarTypesFromSource(t, `fn Main() Number {
		var c = if 1 > 0 { 1 } else { -1 };
		return c
	}`)
	if vars["c"] != "Number" {
		t.Errorf("var c type = %q, want Number", vars["c"])
	}
}

// TestCheckIfExpressionElseIfRejectsNonBoolCond confirms the cond check
// applies inside else-if clauses too, not just the leading if.
func TestCheckIfExpressionElseIfRejectsNonBoolCond(t *testing.T) {
	expectError(t, `fn Main() Number {
		var c = if 1 > 0 { 1 } else if 1 { 2 } else { 0 };
		return c
	}`, "must be Bool")
}
