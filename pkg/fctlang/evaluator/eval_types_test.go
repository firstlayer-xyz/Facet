package evaluator

import "testing"

// valueEqual must be reflexive and structural for arrays. The original
// implementation returned false unconditionally for arrays, which broke
// the `changed` tracker in coerceToType and caused needless reallocation
// when an array element is itself an array (and equal after coercion).

func TestValueEqualArrayReflexive(t *testing.T) {
	arr := array{elems: []value{float64(1), float64(2), float64(3)}, elemType: "Number"}
	if !valueEqual(arr, arr) {
		t.Error("valueEqual(arr, arr) should be true (reflexivity)")
	}
}

func TestValueEqualArraySameContents(t *testing.T) {
	a := array{elems: []value{float64(1), float64(2)}, elemType: "Number"}
	b := array{elems: []value{float64(1), float64(2)}, elemType: "Number"}
	if !valueEqual(a, b) {
		t.Error("valueEqual with same contents should be true")
	}
}

func TestValueEqualArrayDifferentContents(t *testing.T) {
	a := array{elems: []value{float64(1), float64(2)}, elemType: "Number"}
	b := array{elems: []value{float64(1), float64(3)}, elemType: "Number"}
	if valueEqual(a, b) {
		t.Error("valueEqual with different contents should be false")
	}
}

func TestValueEqualArrayDifferentLengths(t *testing.T) {
	a := array{elems: []value{float64(1), float64(2)}, elemType: "Number"}
	b := array{elems: []value{float64(1)}, elemType: "Number"}
	if valueEqual(a, b) {
		t.Error("valueEqual with different lengths should be false")
	}
}

func TestValueEqualNestedArray(t *testing.T) {
	inner := array{elems: []value{float64(1), float64(2)}, elemType: "Number"}
	a := array{elems: []value{inner}, elemType: "[]Number"}
	b := array{elems: []value{inner}, elemType: "[]Number"}
	if !valueEqual(a, b) {
		t.Error("valueEqual on nested arrays sharing an inner array should be true")
	}
}

func TestValueEqualStructPointerIdentity(t *testing.T) {
	sv := &structVal{typeName: "Foo"}
	if !valueEqual(sv, sv) {
		t.Error("valueEqual(sv, sv) should be true for same *structVal pointer")
	}
	other := &structVal{typeName: "Foo"}
	if valueEqual(sv, other) {
		t.Error("valueEqual on distinct *structVal pointers should be false")
	}
}

func TestValueEqualLength(t *testing.T) {
	if !valueEqual(length{mm: 5}, length{mm: 5}) {
		t.Error("valueEqual on equal lengths should be true")
	}
	if valueEqual(length{mm: 5}, length{mm: 6}) {
		t.Error("valueEqual on distinct lengths should be false")
	}
}
