package checker

import "testing"

// Scalar-vector broadcasting: a numeric scalar (Number, Length, Angle) on
// either side of +, -, *, / against an array applies element-wise. Mirrors
// SCAD/NumPy semantics. The result type is the array of the per-element op
// result.
func TestCheckBroadcastNumberTimesNumberArray(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = 3 * [1, 2, 3];
    return Cube(s: v[0] * 1 mm);
}`)
}

func TestCheckBroadcastNumberArrayTimesNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = [1, 2, 3] * 3;
    return Cube(s: v[0] * 1 mm);
}`)
}

func TestCheckBroadcastNumberPlusArray(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = 1 + [10, 20, 30];
    return Cube(s: v[0] * 1 mm);
}`)
}

func TestCheckBroadcastArrayMinusNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = [10, 20, 30] - 5;
    return Cube(s: v[1] * 1 mm);
}`)
}

func TestCheckBroadcastArrayDivNumber(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = [10, 20, 30] / 2;
    return Cube(s: v[0] * 1 mm);
}`)
}

// Length op Number array broadcasts the unit through: 2 mm * [1,2,3] yields
// an array of Length values.
func TestCheckBroadcastLengthTimesNumberArray(t *testing.T) {
	expectNoErrors(t, `fn Main() Solid {
    var v = 2 mm * [1, 2, 3];
    return Cube(s: v[0]);
}`)
}
